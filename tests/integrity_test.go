package site_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodeBlockLanguageAttributes verifies code fences have data-lang attributes
// from Hugo's syntax highlighting. If these disappear, Chroma can't apply
// language-specific colors and code blocks become monochrome.
func TestCodeBlockLanguageAttributes(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	codeBlocks := page.Locator("code[data-lang]")
	count, err := codeBlocks.Count()
	require.NoError(t, err)
	assert.Greater(t, count, 3,
		"article with multiple code blocks should have data-lang attributes")

	// Verify at least one block is tagged as Go
	langs, err := codeBlocks.EvaluateAll(
		`els => els.map(e => e.getAttribute("data-lang"))`,
	)
	require.NoError(t, err)
	langList := toStringSlice(langs)
	assert.True(t, slices.ContainsFunc(langList, func(s string) bool { return strings.Contains(s, "go") }),
		"Go article should have code blocks with data-lang='go'")
}

// Homepage post count and archive total are verified by TestSiteBuildSmokeTest.

// TestRSSFeedImageElement verifies the RSS feed includes the channel image
// element with the site cover image, which feed readers display as the
// feed's icon/logo.
func TestRSSFeedImageElement(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")

	assert.Contains(t, body, "<image>", "RSS should have channel image element")
	assert.Contains(t, body, "<url>", "RSS image should have url")
	assert.Contains(t, body, "blob.rednafi.com",
		"RSS image URL should reference the cover image CDN")
}

// TestRSSFeedGeneratorTag verifies Hugo version appears in the RSS feed,
// which helps diagnose build environment issues.
func TestRSSFeedGeneratorTag(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")
	assert.Contains(t, body, "<generator>Hugo",
		"RSS feed should declare Hugo as generator")
}

// TestBodyOverflowHidden verifies the body prevents horizontal scrolling.
// If this breaks, long code blocks or images cause a horizontal scrollbar.
func TestBodyOverflowHidden(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	overflow, err := page.Evaluate(
		`() => getComputedStyle(document.body).overflowX`,
	)
	require.NoError(t, err)
	assert.Equal(t, "hidden", overflow,
		"body should have overflow-x: hidden to prevent horizontal scroll")
}

// TestSmoothScrollBehavior verifies the page uses smooth scrolling for
// anchor navigation (but not when prefers-reduced-motion is set — that's
// tested in TestReducedMotion).
func TestSmoothScrollBehavior(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	sb, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).scrollBehavior`,
	)
	require.NoError(t, err)
	assert.Equal(t, "smooth", sb,
		"html should have scroll-behavior: smooth for anchor navigation")
}

// TestImageBorderRadius verifies article images have rounded corners
// applied by the global CSS rule.
func TestImageBorderRadius(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/about/")

	img := page.Locator("article img, .about-avatar").First()
	count, err := img.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no images on this page")
	}

	radius, err := img.Evaluate(
		`el => getComputedStyle(el).borderRadius`, nil,
	)
	require.NoError(t, err)
	assert.NotEqual(t, "0px", radius,
		"images should have border-radius for rounded corners")
}

// TestRSSFeedDescriptionFallback verifies RSS items use Description when
// available, falling back to Summary. Items without either would show
// empty content in feed readers.
func TestRSSFeedDescriptionFallback(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")

	parts := strings.Split(body, "<item>")
	for i, part := range parts[1:] {
		_, after, found := strings.Cut(part, "<description>")
		if !found {
			t.Errorf("RSS item %d missing <description>", i)
			continue
		}
		desc, _, _ := strings.Cut(after, "</description>")
		assert.Greater(t, len(strings.TrimSpace(desc)), 10,
			"RSS item %d description too short (possible empty fallback)", i)
		if i >= 9 {
			break // check first 10
		}
	}
}
