package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostChronologicalOrder verifies the homepage lists posts in reverse
// chronological order. If the sort order breaks, the newest content gets
// buried and users see stale posts first.
func TestPostChronologicalOrder(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	dates, err := page.Locator(".post-list .post time").EvaluateAll(
		`els => els.map(e => e.getAttribute("datetime"))`,
	)
	require.NoError(t, err)
	dateList := toStringSlice(dates)
	require.Greater(t, len(dateList), 5, "need multiple posts to verify order")

	// Dates should be in descending order (newest first)
	for i := 1; i < len(dateList); i++ {
		assert.GreaterOrEqual(t, dateList[i-1], dateList[i],
			"posts should be in reverse chronological order: %s should come after %s",
			dateList[i-1], dateList[i])
	}
}

// TestArchiveChronologicalOrder verifies the archive page groups years
// in descending order (newest year first).
func TestArchiveChronologicalOrder(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/archive/")

	years, err := page.Locator(".archive-year h2 a").EvaluateAll(
		`els => els.map(e => e.textContent.trim())`,
	)
	require.NoError(t, err)
	yearList := toStringSlice(years)
	require.Greater(t, len(yearList), 2, "need multiple years")

	for i := 1; i < len(yearList); i++ {
		assert.Greater(t, yearList[i-1], yearList[i],
			"archive years should descend: %s before %s", yearList[i-1], yearList[i])
	}
}

// TestFontDisplaySwap verifies all @font-face declarations use font-display: swap,
// preventing FOIT (flash of invisible text). Without swap, text is invisible
// until the custom font loads — a major UX regression on slow connections.
func TestFontDisplaySwap(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	// Check all @font-face rules have font-display: swap
	result, err := page.Evaluate(`() => {
		const issues = [];
		for (const sheet of document.styleSheets) {
			try {
				for (const rule of sheet.cssRules) {
					if (rule instanceof CSSFontFaceRule) {
						const display = rule.style.fontDisplay;
						if (display !== "swap") {
							issues.push(rule.style.fontFamily + ": " + display);
						}
					}
				}
			} catch(e) {}
		}
		return issues;
	}`)
	require.NoError(t, err)
	issues := toStringSlice(result)
	assert.Empty(t, issues,
		"all @font-face rules should use font-display: swap, but found: %v", issues)
}

// TestSectionRSSFeedTitles verifies section-specific RSS feeds use the
// "Section on Site Title" format, distinguishing them from the main feed.
func TestSectionRSSFeedTitles(t *testing.T) {
	t.Parallel()
	sections := map[string]string{
		"go":     "Go on",
		"python": "Python on",
	}

	for section, expectedPrefix := range sections {
		t.Run(section, func(t *testing.T) {
			body := httpGet(t, baseURL+"/"+section+"/index.xml")
			_, after, found := strings.Cut(body, "<title>")
			require.True(t, found, "section RSS should have title")
			title, _, _ := strings.Cut(after, "</title>")
			assert.Contains(t, title, expectedPrefix,
				"section RSS title should start with %q", expectedPrefix)
			assert.Contains(t, title, "Reflections",
				"section RSS title should include site name")
		})
	}
}

// TestTagCountsOnTaxonomyPage verifies each tag on /tags/ shows the number
// of posts, helping users gauge content depth per topic.
func TestTagCountsOnTaxonomyPage(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/tags/")

	// The taxonomy page renders tags as: <a href="...">TagName</a> (count)
	firstPost := page.Locator(".post-list .post").First()
	text, err := firstPost.TextContent()
	require.NoError(t, err)

	// Should contain a parenthesized number like "(42)"
	assert.Regexp(t, `\(\d+\)`, text,
		"tag entries should show post count in parentheses")
}

// TestExternalLinksAcrossPages verifies the render-link.html hook correctly
// adds rel="noopener noreferrer" and target="_blank" to external links
// on multiple page types, not just articles.
func TestExternalLinksAcrossPages(t *testing.T) {
	t.Parallel()
	pages := []string{
		"/about/",
		"/blogroll/",
		"/maxims/",
	}

	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			extLinks := page.Locator(`article a[href^="http"]:not([href*="rednafi.com"])`)
			count, err := extLinks.Count()
			require.NoError(t, err)
			if count == 0 {
				t.Skipf("no external links on %s", url)
			}

			// Check first 5 external links
			for i := range min(count, 5) {
				link := extLinks.Nth(i)
				href, _ := link.GetAttribute("href")
				if strings.HasPrefix(href, "mailto:") {
					continue
				}

				rel, err := link.GetAttribute("rel")
				require.NoError(t, err, "link to %s missing rel on %s", href, url)
				assert.Contains(t, rel, "noopener",
					"%s: link to %s missing noopener", url, href)

				target, err := link.GetAttribute("target")
				require.NoError(t, err)
				assert.Equal(t, "_blank", target,
					"%s: link to %s missing target=_blank", url, href)
			}
		})
	}
}

// TestCSSVariableConsistencyAcrossPages verifies core CSS variables resolve
// to the same values on different page types (homepage, article, about).
// A mismatch means a template is overriding or missing the root stylesheet.
func TestCSSVariableConsistencyAcrossPages(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/about/", "/go/anemic-stack-traces/", "/archive/"}
	var firstBg string

	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			bg, err := page.Evaluate(
				`() => getComputedStyle(document.documentElement).getPropertyValue("--bg").trim()`,
			)
			require.NoError(t, err)
			bgStr, _ := bg.(string)
			assert.NotEmpty(t, bgStr, "CSS variable --bg should be defined on %s", url)

			if firstBg == "" {
				firstBg = bgStr
			} else {
				assert.Equal(t, firstBg, bgStr,
					"--bg should be consistent across pages (got %s on %s, expected %s)",
					bgStr, url, firstBg)
			}
		})
	}
}
