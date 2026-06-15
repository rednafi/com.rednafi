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
			// Rules live inside @layer blocks (and @media groups), so recurse.
			const walk = (rules) => {
				for (const rule of rules) {
					if ((rule.cssText || "").includes("attr(href)")) return true;
					if (rule.cssRules && walk(rule.cssRules)) return true;
				}
				return false;
			};
			for (const sheet of document.styleSheets) {
				try { if (walk(sheet.cssRules)) return true; } catch(e) {}
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
			// Rules live inside @layer blocks, so recurse into nested cssRules.
			const walk = (rules) => {
				for (const rule of rules) {
					if (rule.selectorText && rule.selectorText.includes(":focus-visible")) return true;
					if (rule.cssRules && walk(rule.cssRules)) return true;
				}
				return false;
			};
			for (const sheet of document.styleSheets) {
				try { if (walk(sheet.cssRules)) return true; } catch(e) {}
			}
			return false;
		}`)
		require.NoError(t, err)
		assert.True(t, hasRule.(bool), "CSS should define :focus-visible styles")
	})

	t.Run("theme toggle shows a focus indicator on keyboard focus", func(t *testing.T) {
		require.NoError(t, page.Locator("button[data-theme-set='dark']").Focus())
		// A visible focus indicator is either an outline or a Geist box-shadow
		// focus ring — accept either.
		indicator, err := page.Locator("button[data-theme-set='dark']").Evaluate(
			`el => {
				const s = getComputedStyle(el);
				const hasOutline = s.outlineStyle !== "none" && parseFloat(s.outlineWidth) > 0;
				const hasRing = s.boxShadow && s.boxShadow !== "none";
				return hasOutline || hasRing ? "visible" : "none";
			}`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "visible", indicator,
			"focused button should show a visible focus indicator (outline or ring)")
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

// TestBackToTopVisibility verifies the back-to-top button stays keyboard
// reachable while scrolled down and hides only when returning near the top.
func TestBackToTopVisibility(t *testing.T) {
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

	// It should not disappear while the user is still scrolled down.
	time.Sleep(2 * time.Second)

	hasVisible, err = btn.Evaluate(`el => el.classList.contains("visible")`, nil)
	require.NoError(t, err)
	assert.True(t, hasVisible.(bool), "should stay visible while scrolled down")

	_, err = page.Evaluate(`() => window.scrollTo(0, 0)`)
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	hasVisible, err = btn.Evaluate(`el => el.classList.contains("visible")`, nil)
	require.NoError(t, err)
	assert.False(t, hasVisible.(bool), "should hide near the top")
}

// TestBreadcrumbsOnDifferentPages verifies breadcrumb navigation renders
// correctly across different page types (articles, sections).
func TestBreadcrumbsOnDifferentPages(t *testing.T) {
	t.Parallel()
	pages := map[string][]string{
		"/go/anemic-stack-traces/":    {"home", "go"},
		"/python/dataclasses/":        {"home", "python"},
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

// TestPostListHoverCSS verifies the date-forward post list: the hover affordance
// lives on the title (a colour transition), rows are split by a single hairline,
// and the date/category meta line sits above the title in DOM and visual order.
// We verify the rules rather than observing computed hover state, because CSS
// transitions make hover assertions timing-sensitive and flaky.
func TestPostListHoverCSS(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("title has a colour transition for hover", func(t *testing.T) {
		transition, err := page.Locator(".post-list .post > a").First().Evaluate(
			`el => getComputedStyle(el).transition`, nil,
		)
		require.NoError(t, err)
		transStr, _ := transition.(string)
		assert.Contains(t, transStr, "color",
			"post title should have a colour transition for its hover affordance")
	})

	t.Run("rows are separated by a hairline", func(t *testing.T) {
		// A top border sits between siblings (.post + .post): the first row has
		// none, the second carries a solid hairline.
		borderStyle, err := page.Locator(".post-list .post").Nth(1).Evaluate(
			`el => getComputedStyle(el).borderTopStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "solid", borderStyle,
			"rows after the first should have a solid top hairline")
	})

	t.Run("meta line is a kicker above the title", func(t *testing.T) {
		firstClass, err := page.Locator(".post-list .post").First().Evaluate(
			`el => el.firstElementChild.className`, nil,
		)
		require.NoError(t, err)
		assert.Contains(t, firstClass, "post-meta-line",
			"meta line should precede the title in DOM order")
	})
}

// TestSidebarLinkTransition verifies sidebar links keep a simple color-only
// hover affordance without motion.
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

	t.Run("has no transform transition", func(t *testing.T) {
		transition, err := link.Evaluate(
			`el => getComputedStyle(el).transition`, nil,
		)
		require.NoError(t, err)
		transStr, _ := transition.(string)
		assert.Contains(t, transStr, "color",
			"sidebar link should have a color transition")
		assert.NotContains(t, transStr, "transform",
			"sidebar link hover should not move")
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
	require.NoError(t, page.Locator("button[data-theme-set='dark']").Click())

	// Get the --code-bg variable value in dark theme
	darkCodeBg, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--code-bg").trim()`,
	)
	require.NoError(t, err)

	assert.NotEqual(t, lightCodeBg, darkCodeBg,
		"--code-bg should differ between light (%v) and dark (%v) themes",
		lightCodeBg, darkCodeBg)
	assert.Equal(t, "#1a1a1a", darkCodeBg)
}
