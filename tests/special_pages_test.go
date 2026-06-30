package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	link := page.Locator(".article-content a:not(.anchor):not(.footnotes a)").First()
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

	t.Run("has rounded Geist badge corners", func(t *testing.T) {
		// Geist badges are small rounded rectangles (var(--radius-sm) = 4px),
		// not full pills.
		radius, err := tag.Evaluate(
			`el => getComputedStyle(el).borderTopLeftRadius`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "0px", radius, "tags should have rounded corners")
		assert.NotContains(t, radius, "999", "tags should be a rounded rectangle, not a full pill")
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
