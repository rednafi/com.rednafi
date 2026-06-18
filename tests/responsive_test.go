package site_test

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTabletLayout verifies the intermediate breakpoint (768px) keeps the home
// as a single centered feed column (the bio lives on /about, no home sidebar).
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

	t.Run("home has no sidebar at 768px", func(t *testing.T) {
		count, err := page.Locator("main aside").Count()
		require.NoError(t, err)
		assert.Equal(t, 0, count, "home is single-column at tablet width")
	})

	t.Run("feed column is centered within the reading width", func(t *testing.T) {
		mw, err := page.Locator(".content-column.home").Evaluate(
			`el => getComputedStyle(el).maxWidth`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "720px", mw, "home shares the 720px reading column")
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
		return toFloat(size)
	}

	// matched to vercel.com/blog: h1 48px desktop → 40px mobile, body 18px → 16px
	t.Run("h1 matches the vercel scale on desktop and mobile", func(t *testing.T) {
		assert.InDelta(t, 48.0, fontSize(desktop, "h1"), 0.5, "desktop h1 should be 48px")
		assert.InDelta(t, 40.0, fontSize(mobile, "h1"), 0.5, "mobile h1 should be 40px")
	})

	t.Run("reading body matches the vercel scale", func(t *testing.T) {
		assert.InDelta(t, 18.0, fontSize(desktop, ".article-content p"), 0.5,
			"desktop article body should be 18px")
		assert.InDelta(t, 16.0, fontSize(mobile, ".article-content p"), 0.5,
			"mobile article body should be 16px")
	})

	t.Run("pre keeps its full size on mobile", func(t *testing.T) {
		assert.InDelta(t, fontSize(desktop, "pre"), fontSize(mobile, "pre"), 0.2,
			"pre should not shrink at the mobile breakpoint")
	})
}

// TestPostListTitleTypographyTokens verifies post-list titles use the vercel
// display-type scale: large + weight-450 + tight tracking (--fs-list-title:
// 26px desktop, 23px mobile), not small + bold.
func TestPostListTitleTypographyTokens(t *testing.T) {
	t.Parallel()
	desktop := newPage(t)
	goto_(t, desktop, "/")
	mobile := newMobilePage(t)
	goto_(t, mobile, "/")

	size := func(page playwright.Page) float64 {
		v, err := page.Locator(".post-list .post > a").First().Evaluate(
			`el => parseFloat(getComputedStyle(el).fontSize)`, nil,
		)
		require.NoError(t, err)
		return toFloat(v)
	}
	weight := func(page playwright.Page) string {
		v, err := page.Locator(".post-list .post > a").First().Evaluate(
			`el => getComputedStyle(el).fontWeight`, nil,
		)
		require.NoError(t, err)
		s, _ := v.(string)
		return s
	}

	assert.InDelta(t, 26.0, size(desktop), 0.5,
		"desktop post-list title should use --fs-list-title (26px)")
	assert.InDelta(t, 23.0, size(mobile), 0.5,
		"mobile post-list title should drop to --fs-list-title (23px)")
	assert.Equal(t, "450", weight(desktop),
		"list titles use weight-450 — a touch more ink than 400, not bold")
}

// TestStackedHomeFrameNoRail verifies the home frame drops its inner vertical
// rail on small screens.
func TestStackedHomeFrameNoRail(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/")

	bg, err := page.Locator("main.main-list").Evaluate(
		`el => getComputedStyle(el).backgroundImage`, nil,
	)
	require.NoError(t, err)
	assert.NotContains(t, bg.(string), "repeating-linear-gradient",
		"stacked home layout should not draw the inner vertical rail")
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
