package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetiredPageRedirects verify retired duplicate pages point to the homepage.
func TestRetiredPageRedirects(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/about/", "/articles/"} {
		t.Run(path, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+path)
			defer resp.Body.Close()
			assert.Equal(t, 200, resp.StatusCode)

			body := httpGet(t, baseURL+path)
			assert.Contains(t, body, "http-equiv=refresh",
				"%s alias should contain a meta refresh", path)
			assert.Contains(t, body, "url=https://rednafi.com/",
				"%s alias should redirect to the homepage", path)
			assert.Contains(t, body, `rel=canonical href=https://rednafi.com/`,
				"%s alias canonical should point to the homepage", path)
			assert.Contains(t, body, `name=robots content="noindex, follow"`,
				"%s alias should tell crawlers not to index the redirect page", path)
		})
	}
}

// TestHostRedirectRules verifies hosts that support _redirects can serve
// retired duplicate pages as real permanent redirects.
func TestHostRedirectRules(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/_redirects")
	assert.Contains(t, body, "/about/ / 301")
	assert.Contains(t, body, "/articles/ / 301")
}

// TestSearchPageConfiguration verifies the Pagefind search UI is configured
// with the expected options (min term length, excerpt length, etc.).
func TestSearchPageConfiguration(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/search/")

	t.Run("pagefind CSS is loaded", func(t *testing.T) {
		count, err := page.Locator(`link[href*="pagefind"]`).Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0, "pagefind CSS should be loaded")
	})

	t.Run("pagefind JS is deferred", func(t *testing.T) {
		script := page.Locator(`script[src*="pagefind"]`)
		count, err := script.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "pagefind JS should be present")

		defer_, err := script.First().GetAttribute("defer")
		require.NoError(t, err)
		assert.NotNil(t, defer_, "pagefind JS should be deferred")
	})

	t.Run("short search terms rejected", func(t *testing.T) {
		// The processTerm function rejects terms < 3 characters
		err := page.Locator(".pagefind-ui__search-input").WaitFor()
		require.NoError(t, err)

		input := page.Locator(".pagefind-ui__search-input")
		require.NoError(t, input.Fill("go"))

		// With a 2-char term, processTerm returns null → no results should appear
		count, err := page.Locator(".pagefind-ui__result").Count()
		require.NoError(t, err)
		assert.Equal(t, 0, count,
			"2-character search term should be rejected by processTerm")
	})
}

// TestSkipLinkFocusBehavior verifies the skip-to-content link is hidden
// off-screen by default but becomes visible when focused via keyboard.
func TestSkipLinkFocusBehavior(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	skip := page.Locator("a.skip-link")

	t.Run("hidden off-screen by default", func(t *testing.T) {
		left, err := skip.Evaluate(
			`el => getComputedStyle(el).left`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "-9999px", left,
			"skip link should be positioned off-screen")
	})

	t.Run("visible on focus", func(t *testing.T) {
		require.NoError(t, skip.Focus())

		left, err := skip.Evaluate(
			`el => getComputedStyle(el).left`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "-9999px", left,
			"skip link should move on-screen when focused")
	})
}

// TestHorizontalRuleRendering verifies the horizontal rule renders as a clean
// centered hairline (no border; a thin background bar).
func TestHorizontalRuleRendering(t *testing.T) {
	t.Parallel()
	requirePage(t, "/shards/2026/03/background-jobs-inherited-fd/")
	page := newPage(t)
	goto_(t, page, "/shards/2026/03/background-jobs-inherited-fd/")

	hr := page.Locator("article hr")
	count, err := hr.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no horizontal rule on this page")
	}

	t.Run("hr has no visible border", func(t *testing.T) {
		border, err := hr.First().Evaluate(
			`el => getComputedStyle(el).borderTopStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "none", border,
			"hr should have border: none (styled as a background bar)")
	})

	t.Run("hr renders as a thin visible bar", func(t *testing.T) {
		bg, err := hr.First().Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "rgba(0, 0, 0, 0)", bg,
			"hr should have a visible background color")
		assert.NotEqual(t, "transparent", bg,
			"hr should have a visible background color")
	})
}

// TestArticleLinkUnderlineAnimation verifies the animated underline on
// article body links (gradient background-size from 0% to 100% on hover).
func TestArticleLinkUnderlineAnimation(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	link := page.Locator("article a:not(.anchor):not(.footnotes a)").First()
	count, err := link.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no article links on this page")
	}

	t.Run("link has no text-decoration", func(t *testing.T) {
		// CSS: article a { text-decoration: none; background-image: linear-gradient(...) }
		td, err := link.Evaluate(
			`el => getComputedStyle(el).textDecorationLine`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "none", td,
			"article links use gradient underline, not text-decoration")
	})

	t.Run("link has gradient background-image", func(t *testing.T) {
		bgImage, err := link.Evaluate(
			`el => getComputedStyle(el).backgroundImage`, nil,
		)
		require.NoError(t, err)
		bgStr, _ := bgImage.(string)
		assert.Contains(t, bgStr, "gradient",
			"article links should have gradient background-image for underline")
	})
}

// TestTagPillStyling verifies tag pills on article pages have the expected
// pill-shaped styling (rounded corners, background, border).
func TestTagPillStyling(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	tags := page.Locator("ul.post-tags a")
	count, err := tags.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no tags on this page")
	}

	tag := tags.First()

	t.Run("has pill border-radius", func(t *testing.T) {
		radius, err := tag.Evaluate(
			`el => getComputedStyle(el).borderRadius`, nil,
		)
		require.NoError(t, err)
		assert.Contains(t, radius, "999", "tags should have pill border-radius (999px)")
	})

	t.Run("has background color", func(t *testing.T) {
		bg, err := tag.Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "rgba(0, 0, 0, 0)", bg,
			"tag pill should have background color")
	})

	t.Run("has border", func(t *testing.T) {
		borderStyle, err := tag.Evaluate(
			`el => getComputedStyle(el).borderStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "solid", borderStyle, "tag pill should have solid border")
	})
}
