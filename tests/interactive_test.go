package site_test

import (
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrintURLExpansion verifies that in print media, external links show their
// URL in parentheses after the link text. This is critical for printed articles
// where hyperlinks aren't clickable.
func TestPrintURLExpansion(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")
	require.NoError(t, page.EmulateMedia(playwright.PageEmulateMediaOptions{
		Media: playwright.MediaPrint,
	}))

	t.Run("print media CSS contains URL expansion rules", func(t *testing.T) {
		// The minified CSS may combine selectors, so we search through
		// all @media print rules looking for the attr(href) content.
		hasURLExpansion, err := page.Evaluate(`() => {
			for (const sheet of document.styleSheets) {
				try {
					for (const rule of sheet.cssRules) {
						// Look in @media print groups
						if (rule.media && rule.media.mediaText === "print") {
							for (const inner of rule.cssRules) {
								const text = inner.cssText || "";
								if (text.includes("attr(href)")) return true;
							}
						}
					}
				} catch(e) {}
			}
			return false;
		}`)
		require.NoError(t, err)
		assert.True(t, hasURLExpansion.(bool),
			"print CSS should expand link URLs via attr(href)")
	})

	t.Run("post-tags hidden in print", func(t *testing.T) {
		display, err := page.Evaluate(
			`() => {
				const tags = document.querySelector(".post-tags");
				return tags ? getComputedStyle(tags).display : "not-found";
			}`,
		)
		require.NoError(t, err)
		if display != "not-found" {
			assert.Equal(t, "none", display, "post-tags should be hidden in print")
		}
	})

	t.Run("related posts hidden in print", func(t *testing.T) {
		display, err := page.Evaluate(
			`() => {
				const related = document.querySelector(".related-posts");
				return related ? getComputedStyle(related).display : "not-found";
			}`,
		)
		require.NoError(t, err)
		if display != "not-found" {
			assert.Equal(t, "none", display, "related posts should be hidden in print")
		}
	})
}

// TestFocusVisibleStyles verifies keyboard users see an outline when focusing
// interactive elements. This is a WCAG 2.1 Level AA requirement.
func TestFocusVisibleStyles(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("focus-visible CSS rule exists", func(t *testing.T) {
		hasRule, err := page.Evaluate(`() => {
			for (const sheet of document.styleSheets) {
				try {
					for (const rule of sheet.cssRules) {
						if (rule.selectorText && rule.selectorText.includes(":focus-visible")) {
							return true;
						}
					}
				} catch(e) {}
			}
			return false;
		}`)
		require.NoError(t, err)
		assert.True(t, hasRule.(bool), "CSS should define :focus-visible styles")
	})

	t.Run("theme toggle shows outline on keyboard focus", func(t *testing.T) {
		require.NoError(t, page.Locator("button.theme-toggle").Focus())
		outline, err := page.Locator("button.theme-toggle").Evaluate(
			`el => getComputedStyle(el).outlineStyle`, nil,
		)
		require.NoError(t, err)
		// Chromium may report "auto" or "solid" for focus-visible
		assert.NotEqual(t, "none", outline, "focused button should have visible outline")
	})
}

// TestTOCExpandCollapse verifies the table of contents <details> element
// can be toggled open and closed.
func TestTOCExpandCollapse(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/configure-options/")

	toc := page.Locator("details.toc")
	count, err := toc.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no TOC on this page")
	}

	t.Run("starts closed", func(t *testing.T) {
		open, err := toc.GetAttribute("open")
		require.NoError(t, err)
		assert.Empty(t, open, "TOC should start collapsed")
	})

	t.Run("opens on click", func(t *testing.T) {
		require.NoError(t, toc.Locator("summary").Click())
		// After clicking, <details> should have the "open" attribute
		isOpen, err := toc.Evaluate(`el => el.open`, nil)
		require.NoError(t, err)
		assert.True(t, isOpen.(bool), "TOC should be open after click")
	})

	t.Run("shows links when open", func(t *testing.T) {
		links := toc.Locator("a")
		linkCount, err := links.Count()
		require.NoError(t, err)
		assert.Greater(t, linkCount, 0, "open TOC should show navigation links")
	})

	t.Run("closes on second click", func(t *testing.T) {
		require.NoError(t, toc.Locator("summary").Click())
		isOpen, err := toc.Evaluate(`el => el.open`, nil)
		require.NoError(t, err)
		assert.False(t, isOpen.(bool), "TOC should close on second click")
	})
}

// TestBackToTopAutoHide verifies the back-to-top button automatically hides
// after 1.5 seconds of no scrolling (the JS uses setTimeout(1500)).
func TestBackToTopAutoHide(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/archive/")

	btn := page.Locator("a.back-to-top")

	// Scroll down to trigger visibility
	_, err := page.Evaluate(`() => window.scrollTo(0, 500)`)
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	hasVisible, err := btn.Evaluate(`el => el.classList.contains("visible")`, nil)
	require.NoError(t, err)
	assert.True(t, hasVisible.(bool), "should be visible after scroll")

	// Wait for auto-hide timeout (1.5s + buffer)
	time.Sleep(2 * time.Second)

	hasVisible, err = btn.Evaluate(`el => el.classList.contains("visible")`, nil)
	require.NoError(t, err)
	assert.False(t, hasVisible.(bool), "should auto-hide after 1.5s of no scrolling")
}

// TestBreadcrumbsOnDifferentPages verifies breadcrumb navigation renders
// correctly across different page types (articles, sections).
func TestBreadcrumbsOnDifferentPages(t *testing.T) {
	t.Parallel()
	pages := map[string][]string{
		"/go/anemic-stack-traces/": {"home", "go"},
		"/python/dataclasses/":     {"home", "python"},
		"/misc/pesky-little-scripts/": {"home", "misc"},
	}

	for url, expectedCrumbs := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			crumbs := page.Locator("nav.breadcrumbs a")
			count, err := crumbs.Count()
			require.NoError(t, err)
			require.Equal(t, len(expectedCrumbs), count,
				"breadcrumbs count mismatch for %s", url)

			for i, expected := range expectedCrumbs {
				text, err := crumbs.Nth(i).TextContent()
				require.NoError(t, err)
				assert.Equal(t, expected, text,
					"breadcrumb %d on %s should be %q", i, url, expected)
			}
		})
	}
}

// TestPostListHoverCSS verifies the CSS rule for post list hover state exists.
// We verify the rule rather than observing computed hover state, because
// CSS transitions make hover assertions timing-sensitive and flaky.
func TestPostListHoverCSS(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("post has transition property", func(t *testing.T) {
		transition, err := page.Locator(".post-list .post").First().Evaluate(
			`el => getComputedStyle(el).transition`, nil,
		)
		require.NoError(t, err)
		transStr, _ := transition.(string)
		assert.Contains(t, transStr, "background-color",
			"post should have background-color transition for hover effect")
	})

	t.Run("post has border-bottom", func(t *testing.T) {
		border, err := page.Locator(".post-list .post").First().Evaluate(
			`el => getComputedStyle(el).borderBottomStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "solid", border,
			"post should have bottom border separator")
	})
}

// TestSidebarLinkTransition verifies sidebar links have the CSS transition
// properties needed for the hover translateX(2px) effect.
func TestSidebarLinkTransition(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	link := page.Locator(".aside-section li a").First()

	t.Run("has display inline-block", func(t *testing.T) {
		display, err := link.Evaluate(
			`el => getComputedStyle(el).display`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "inline-block", display,
			"sidebar links need inline-block for transform to work")
	})

	t.Run("has transform transition", func(t *testing.T) {
		transition, err := link.Evaluate(
			`el => getComputedStyle(el).transition`, nil,
		)
		require.NoError(t, err)
		transStr, _ := transition.(string)
		assert.Contains(t, transStr, "transform",
			"sidebar link should have transform transition for hover effect")
	})
}

// TestDarkThemeCodeBackground verifies code blocks get a darker background
// when dark theme is active. We verify via CSS variable value since the pre
// background transitions over 200ms.
func TestDarkThemeCodeBackground(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	// Get the --code-bg variable value in light theme
	lightCodeBg, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--code-bg").trim()`,
	)
	require.NoError(t, err)

	// Switch to dark
	require.NoError(t, page.Locator("button.theme-toggle").Click())

	// Get the --code-bg variable value in dark theme
	darkCodeBg, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--code-bg").trim()`,
	)
	require.NoError(t, err)

	assert.NotEqual(t, lightCodeBg, darkCodeBg,
		"--code-bg should differ between light (%v) and dark (%v) themes",
		lightCodeBg, darkCodeBg)
	assert.Equal(t, "#2c3034", darkCodeBg)
}
