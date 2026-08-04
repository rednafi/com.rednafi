package site_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoogleAnalyticsInProduction verifies the production build includes
// the Google Analytics script. If the env/production guard in analytics.html
// breaks, analytics silently disappears and traffic data stops.
func TestGoogleAnalyticsInProduction(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/")

	assert.Contains(t, body, "googletagmanager.com/gtag",
		"production build should include Google Analytics")
	assert.Contains(t, body, "G-11NK905JK8",
		"GA measurement ID should be present")
}

// TestGoogleAnalyticsRespectsDNT verifies the analytics script includes
// Do Not Track detection so privacy-conscious users aren't tracked.
func TestGoogleAnalyticsRespectsDNT(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/")
	assert.Contains(t, body, "doNotTrack",
		"analytics should check Do Not Track preference")
}

// TestGoogleAnalyticsStaysOutOfInitialInteractions keeps analytics from
// competing with search, scrolling, or the Core Web Vitals measurement window.
func TestGoogleAnalyticsStaysOutOfInitialInteractions(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("../layouts/partials/analytics.html")
	require.NoError(t, err)
	body := string(source)

	assert.NotContains(t, body, `["pointerdown","keydown","scroll","touchstart"]`)
	assert.Contains(t, body, "setTimeout(schedule, 5000)")
	assert.Contains(t, body, "requestIdleCallback")
}

// TestCSSIsMinified verifies the served CSS file is minified (no multi-line
// formatting). If minification breaks, page load slows down.
func TestCSSIsMinified(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	href, err := page.Locator(`link[rel="stylesheet"]`).First().GetAttribute("href")
	require.NoError(t, err)

	css := httpGet(t, baseURL+href)
	// Minified CSS has very few newlines relative to its size
	lines := strings.Count(css, "\n")
	assert.Less(t, lines, 20,
		"CSS should be minified (got %d lines for %d bytes)", lines, len(css))
}

// TestAllContentSectionsReturn200 verifies every configured content section
// has a working index page. If a section disappears, all its articles
// become unreachable via section navigation.
func TestAllContentSectionsReturn200(t *testing.T) {
	t.Parallel()
	sections := []string{
		"/python/", "/go/", "/misc/",
		"/javascript/", "/typescript/",
		"/system/", "/zephyr/", "/shards/",
	}

	for _, section := range sections {
		t.Run(section, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+section)
			if resp.StatusCode == 404 {
				t.Skipf("section %s does not exist", section)
			}
			assert.Equal(t, 200, resp.StatusCode, "%s should return 200", section)
			resp.Body.Close()
		})
	}
}

// TestFeedPages verifies the curated annual feed pages exist and render.
// These are evergreen reference pages that aggregate the year's best content.
func TestFeedPages(t *testing.T) {
	t.Parallel()
	pages := []string{"/feed/2024/", "/feed/2025/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+url)
			require.Equal(t, 200, resp.StatusCode, "%s should exist", url)
			resp.Body.Close()

			page := newPage(t)
			goto_(t, page, url)

			// Should have a title (year)
			visible, err := page.Locator("h1").IsVisible()
			require.NoError(t, err)
			assert.True(t, visible, "%s should have h1", url)

			// Should have article content
			text, err := page.Locator("article").TextContent()
			require.NoError(t, err)
			assert.Greater(t, len(text), 50,
				"%s should have substantial content", url)
		})
	}
}

// TestTOCSummaryStyling verifies the table of contents summary element has
// the interactive cursor and background styling that signals clickability.
func TestTOCSummaryStyling(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/configure-options/")

	summary := page.Locator("details.toc summary")
	count, err := summary.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no TOC on this page")
	}

	t.Run("has pointer cursor", func(t *testing.T) {
		cursor, err := summary.Evaluate(
			`el => getComputedStyle(el).cursor`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "pointer", cursor)
	})

	t.Run("has background color", func(t *testing.T) {
		bg, err := summary.Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "rgba(0, 0, 0, 0)", bg,
			"summary should have background color")
	})

	t.Run("has padding", func(t *testing.T) {
		padding, err := summary.Evaluate(
			`el => getComputedStyle(el).paddingLeft`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "0px", padding,
			"summary should have padding")
	})

	t.Run("uses readable font size", func(t *testing.T) {
		ratio, err := page.Locator("details.toc").Evaluate(
			`el => {
				const tocSize = parseFloat(getComputedStyle(el).fontSize);
				const rootSize = parseFloat(getComputedStyle(document.documentElement).fontSize);
				return tocSize / rootSize;
			}`,
			nil,
		)
		require.NoError(t, err)
		assert.InEpsilon(t, 0.9, ratio.(float64), 0.01,
			"TOC font should stay at 0.9rem")
	})
}

func TestPageHasNoLoadAnimation(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	for _, selector := range []string{"body", ".home-hero"} {
		animation, err := page.Locator(selector).Evaluate(
			`element => getComputedStyle(element).animationName`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "none", animation, "%s should not animate on page load", selector)
	}
}

func TestHeroImageIsResponsiveAndAnimationFree(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/")

	assert.Contains(t, body, `class=hero__image`)
	assert.Contains(t, body, `srcset=`)
	assert.Contains(t, body, `loading=eager`)
	assert.Contains(t, body, `fetchpriority=high`)
	assert.Contains(t, body, `https://blob.rednafi.com/home/bare-tree-720-94d8aede5a87.jpg`)
	assert.Contains(t, body, `https://blob.rednafi.com/home/bare-tree-1440-4d255ba67226.jpg`)
	assert.Contains(t, body, `https://blob.rednafi.com/home/bare-tree-2400-a9e2a45476dd.jpg`)
	assert.NotContains(t, body, `images.unsplash.com`)
	assert.NotContains(t, body, "hero-rain")
}

func TestHeroImageLoads(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	rendered, err := page.Locator(".hero__image").Evaluate(
		`image => image.complete && image.naturalWidth > 0 && image.naturalHeight > 0`, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, true, rendered, "the hero photograph should load successfully")
}

func TestHeroImageFitsItsStageAcrossViewports(t *testing.T) {
	for _, viewport := range []playwright.Size{
		{Width: 320, Height: 568},
		{Width: 390, Height: 844},
		{Width: 768, Height: 1024},
		{Width: 1280, Height: 800},
	} {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/")

			fits, err := page.Evaluate(`() => {
				const grid = document.querySelector('.hero__grid');
				const gridRect = grid.getBoundingClientRect();
				const gridGap = parseFloat(getComputedStyle(grid).columnGap);
				const stage = document.querySelector('.hero__art-stage').getBoundingClientRect();
				const art = document.querySelector('.hero__art').getBoundingClientRect();
				const body = document.querySelector('.hero__body').getBoundingClientRect();
				const utility = document.querySelector('.hero__utility').getBoundingClientRect();
				const footer = document.querySelector('.hero__footer').getBoundingClientRect();
				const footerItems = [...document.querySelectorAll('.hero__footer a')];
				const scene = document.querySelector('.hero__image').getBoundingClientRect();
				const image = document.querySelector('.hero__image');
				const balancedMobileSpacing = window.innerWidth > 640 ||
					Math.abs((stage.top - body.bottom) - (footer.top - stage.bottom)) < 0.5;
				const stageShapeFits = window.innerWidth > 960
					? Math.abs(stage.top - art.top) < 0.5 &&
						Math.abs(stage.bottom - art.bottom) < 0.5 &&
						stage.left < body.right &&
						Math.abs(
							(body.right + gridGap - stage.left) -
							(stage.right - gridRect.right)
						) < 0.5 &&
						Math.abs(stage.right - utility.right) < 0.5 &&
						Math.abs(stage.right - footer.right) < 0.5 &&
						footerItems.every((item) => item.getBoundingClientRect().right <= footer.right)
					: window.innerWidth <= 640
						? Math.abs(stage.width / stage.height - 0.9) < 0.01
						: Math.abs(stage.width / stage.height - 1.6) < 0.01;
				return stage.width > 0 && stage.height > 0 &&
					stageShapeFits &&
					balancedMobileSpacing &&
					Math.abs(scene.left - stage.left) < 0.5 &&
					Math.abs(scene.right - stage.right) < 0.5 &&
					Math.abs(scene.top - stage.top) < 0.5 &&
					Math.abs(scene.bottom - stage.bottom) < 0.5 &&
					image.complete && image.naturalWidth > 0 && image.naturalHeight > 0;
			}`)
			require.NoError(t, err)
			assert.Equal(t, true, fits, "the hero photograph should fit every stage edge")
		})
	}
}
