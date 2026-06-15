package site_test

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTabletLayout verifies the intermediate breakpoint (768px) where the
// sidebar moves below the post list and the bio spans the full sidebar width.
func TestTabletLayout(t *testing.T) {
	t.Parallel()
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{Width: 768, Height: 1024},
	})
	require.NoError(t, err)
	defer ctx.Close()
	page, err := ctx.NewPage()
	require.NoError(t, err)
	_, err = page.Goto(baseURL + "/")
	require.NoError(t, err)

	t.Run("sidebar still visible at 768px", func(t *testing.T) {
		visible, err := page.Locator(".index aside").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible, "sidebar should still be visible at tablet width")
	})

	t.Run("index stacks at tablet width", func(t *testing.T) {
		display, err := page.Locator(".index").Evaluate(
			`el => getComputedStyle(el).display`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "block", display, "homepage should stack at 768px")
	})

	t.Run("bio spans the full sidebar width", func(t *testing.T) {
		full, err := page.Evaluate(`() => {
			const bio = document.querySelector(".aside-bio").getBoundingClientRect().width;
			const aside = document.querySelector(".index aside").getBoundingClientRect().width;
			return bio / aside > 0.9;
		}`)
		require.NoError(t, err)
		assert.True(t, full.(bool),
			"bio should span the full sidebar when stacked on tablet")
	})
}

// TestMobileCodeBlockOverflow verifies code blocks are scrollable (not
// overflowing the viewport) on mobile devices.
func TestMobileCodeBlockOverflow(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	pre := page.Locator("article pre").First()
	count, err := pre.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no code blocks on this page")
	}

	t.Run("pre has overflow-x auto", func(t *testing.T) {
		overflow, err := pre.Evaluate(
			`el => getComputedStyle(el).overflowX`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "auto", overflow)
	})

	t.Run("pre visible box does not exceed viewport width", func(t *testing.T) {
		exceeds, err := page.Evaluate(
			`() => {
				const pre = document.querySelector("article pre");
				if (!pre) return false;
				// offsetWidth is the rendered box width (not scrollable content)
				return pre.getBoundingClientRect().width > window.innerWidth;
			}`,
		)
		require.NoError(t, err)
		assert.False(t, exceeds.(bool),
			"code block box should not exceed the mobile viewport")
	})
}

// TestMobileTypographyConsistent verifies type sizes do NOT shrink at the mobile
// breakpoint — desktop sizes carry across unchanged (no screen-based reduction).
func TestMobileTypographyConsistent(t *testing.T) {
	t.Parallel()
	desktop := newPage(t)
	goto_(t, desktop, "/go/anemic-stack-traces/")
	mobile := newMobilePage(t)
	goto_(t, mobile, "/go/anemic-stack-traces/")

	fontSize := func(page playwright.Page, sel string) float64 {
		size, err := page.Locator(sel).First().Evaluate(
			`el => parseFloat(getComputedStyle(el).fontSize)`, nil,
		)
		require.NoError(t, err)
		return size.(float64)
	}

	t.Run("h1 keeps its full size on mobile", func(t *testing.T) {
		assert.InDelta(t, fontSize(desktop, "h1"), fontSize(mobile, "h1"), 0.2,
			"h1 should not shrink at the mobile breakpoint")
	})

	t.Run("pre keeps its full size on mobile", func(t *testing.T) {
		assert.InDelta(t, fontSize(desktop, "pre"), fontSize(mobile, "pre"), 0.2,
			"pre should not shrink at the mobile breakpoint")
	})
}

// TestPostListTitleTypographyTokens verifies post-list titles use the --fs-title
// token (1.18rem) consistently on desktop and mobile (no responsive reduction).
func TestPostListTitleTypographyTokens(t *testing.T) {
	t.Parallel()
	desktop := newPage(t)
	goto_(t, desktop, "/")
	mobile := newMobilePage(t)
	goto_(t, mobile, "/")

	read := func(page playwright.Page) float64 {
		size, err := page.Locator(".post-list .post > a").First().Evaluate(
			`el => parseFloat(getComputedStyle(el).fontSize)`, nil,
		)
		require.NoError(t, err)
		return size.(float64)
	}

	assert.InDelta(t, 20.0, read(desktop), 0.2,
		"desktop post-list title should use the --fs-title token (1.18rem at the 17px root)")
	assert.InDelta(t, 20.0, read(mobile), 0.2,
		"mobile post-list title should match desktop (--fs-title, no reduction)")
}

// TestMobileSidebarWrapping verifies the sidebar stacks full-width below the
// content on mobile, with a separator and the bio using the full width.
func TestMobileSidebarWrapping(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/")

	aside := page.Locator(".index aside")
	visible, err := aside.IsVisible()
	require.NoError(t, err)
	require.True(t, visible)

	t.Run("aside spans the full mobile width", func(t *testing.T) {
		full, err := page.Evaluate(`() => {
			const a = document.querySelector(".index aside").getBoundingClientRect().width;
			return a / window.innerWidth > 0.85;
		}`)
		require.NoError(t, err)
		assert.True(t, full.(bool),
			"mobile aside should span the full width")
	})

	t.Run("bio uses the available mobile width", func(t *testing.T) {
		ratio, err := page.Evaluate(
			`() => {
				const bio = document.querySelector(".aside-bio").getBoundingClientRect().width;
				return bio / window.innerWidth;
			}`,
		)
		require.NoError(t, err)
		assert.Greater(t, ratio.(float64), 0.85,
			"bio should use most of the mobile viewport width")
	})

	t.Run("aside has border-top separator", func(t *testing.T) {
		borderStyle, err := aside.Evaluate(
			`el => getComputedStyle(el).borderTopStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "solid", borderStyle,
			"mobile aside should have top border separator")
	})
}

// TestMobileHighlightedLineFullWidth verifies that highlighted code lines
// extend to the full scrollable content width when a code block overflows
// horizontally on mobile, instead of being clamped to the visible viewport.
func TestMobileHighlightedLineFullWidth(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/go/closure-mutable-refs/")

	hl := page.Locator(".highlight .line.hl").First()
	require.NoError(t, hl.WaitFor())

	// Sanity-check the test condition: the surrounding <pre> must actually
	// overflow horizontally at this viewport, otherwise the bug isn't being
	// exercised.
	overflows, err := hl.Evaluate(`el => {
		const pre = el.closest("pre");
		return pre.scrollWidth > pre.clientWidth + 1;
	}`, nil)
	require.NoError(t, err)
	require.True(t, overflows.(bool),
		"expected highlighted code block to overflow on mobile")

	// The highlighted line's right edge should reach the <code>'s right edge
	// (i.e. the full content width). Without the fix it stops at the visible
	// viewport edge, well short of the code's bounding box.
	reachesEnd, err := hl.Evaluate(`el => {
		const code = el.closest("pre").querySelector("code");
		const lineRight = el.getBoundingClientRect().right;
		const codeRight = code.getBoundingClientRect().right;
		return lineRight >= codeRight - 1;
	}`, nil)
	require.NoError(t, err)
	assert.True(t, reachesEnd.(bool),
		"highlighted line should extend to the end of the code content")
}

// TestMobileArticleBreadcrumbs verifies breadcrumbs are visible and properly
// sized on mobile viewports.
func TestMobileArticleBreadcrumbs(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	crumbs := page.Locator("nav.breadcrumbs")
	visible, err := crumbs.IsVisible()
	require.NoError(t, err)
	assert.True(t, visible, "breadcrumbs should be visible on mobile")

	fontSize, err := crumbs.Evaluate(
		`el => parseFloat(getComputedStyle(el).fontSize)`, nil,
	)
	require.NoError(t, err)
	assert.Less(t, fontSize.(float64), float64(16),
		"breadcrumbs should use small font (0.85rem)")
}
