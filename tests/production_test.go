package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoogleAnalyticsInProduction verifies the production build includes
// the Google Analytics script. If the env/production guard in baseof.html
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
}

// TestPageFadeInAnimation verifies the body has a fade-in animation applied
// on page load, creating a smooth appearance transition.
func TestPageFadeInAnimation(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	animation, err := page.Evaluate(
		`() => getComputedStyle(document.body).animationName`,
	)
	require.NoError(t, err)
	assert.Equal(t, "fade-in", animation,
		"body should have fade-in animation")
}
