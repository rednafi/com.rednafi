package site_test

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTabletLayout verifies the intermediate breakpoint (768px) where the
// sidebar narrows to 35% width but stays side-by-side with content.
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

	t.Run("index still uses flex layout", func(t *testing.T) {
		display, err := page.Locator(".index").Evaluate(
			`el => getComputedStyle(el).display`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "flex", display, "homepage should use flex at 768px")
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

// TestMobileFontReduction verifies mobile breakpoint reduces font sizes.
func TestMobileFontReduction(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	t.Run("h1 is smaller on mobile", func(t *testing.T) {
		mobileH1, err := page.Evaluate(
			`() => parseFloat(getComputedStyle(document.querySelector("h1")).fontSize)`,
		)
		require.NoError(t, err)
		// CSS: @media (max-width: 640px) { h1 { font-size: 1.3rem; } }
		// At 17px base, 1.3rem = 22.1px. Desktop h1 is 1.5rem = 25.5px
		assert.Less(t, mobileH1.(float64), float64(24),
			"h1 should be smaller on mobile (<=1.3rem)")
	})

	t.Run("pre font is smaller on mobile", func(t *testing.T) {
		mobilePre, err := page.Evaluate(
			`() => {
				const pre = document.querySelector("pre");
				return pre ? parseFloat(getComputedStyle(pre).fontSize) : 0;
			}`,
		)
		require.NoError(t, err)
		// CSS: @media (max-width: 640px) { pre { font-size: .8rem; } }
		assert.Less(t, mobilePre.(float64), float64(15),
			"pre should use smaller font on mobile")
	})
}

// TestMobileSidebarWrapping verifies the sidebar sections wrap into a
// horizontal flex layout on mobile instead of stacking vertically.
func TestMobileSidebarWrapping(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/")

	aside := page.Locator(".index aside")
	visible, err := aside.IsVisible()
	require.NoError(t, err)
	require.True(t, visible)

	t.Run("aside uses flex-wrap on mobile", func(t *testing.T) {
		wrap, err := aside.Evaluate(
			`el => getComputedStyle(el).flexWrap`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "wrap", wrap, "mobile aside should use flex-wrap")
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
