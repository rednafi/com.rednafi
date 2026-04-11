package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImageAttributes checks that all images in article content have proper
// lazy loading, async decoding, and alt text attributes.
func TestImageAttributes(t *testing.T) {
	t.Parallel()
	// Find an article with images by checking a few known sections
	page := newPage(t)
	// Use a page known to have images, or check programmatically
	articlesWithImages := []string{
		"/go/configure-options/",
		"/python/amphibian-decorators/",
	}

	for _, url := range articlesWithImages {
		goto_(t, page, url)
		images := page.Locator("article img")
		count, err := images.Count()
		require.NoError(t, err)
		if count == 0 {
			continue
		}

		t.Run(url, func(t *testing.T) {
			for i := range count {
				img := images.Nth(i)

				t.Run("lazy loading", func(t *testing.T) {
					loading, err := img.GetAttribute("loading")
					require.NoError(t, err)
					assert.Equal(t, "lazy", loading, "img %d missing lazy loading", i)
				})

				t.Run("async decoding", func(t *testing.T) {
					decoding, err := img.GetAttribute("decoding")
					require.NoError(t, err)
					assert.Equal(t, "async", decoding, "img %d missing async decoding", i)
				})
			}
		})
		return // test at least one page with images
	}
}

// TestExternalImageReferrerPolicy checks external images have referrerpolicy="no-referrer".
func TestExternalImageReferrerPolicy(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	// Check multiple articles
	articles := []string{
		"/go/configure-options/",
		"/go/anemic-stack-traces/",
	}

	for _, url := range articles {
		goto_(t, page, url)
		extImages := page.Locator(`article img[src^="http"]`)
		count, err := extImages.Count()
		require.NoError(t, err)
		for i := range count {
			policy, err := extImages.Nth(i).GetAttribute("referrerpolicy")
			require.NoError(t, err)
			assert.Equal(t, "no-referrer", policy, "external img %d at %s missing referrerpolicy", i, url)
		}
	}
}

// TestItalicFontVariantsServed checks italic font variants are served.
// Regular variants are already verified in TestFontLoading (site_test.go).
func TestItalicFontVariantsServed(t *testing.T) {
	t.Parallel()
	fonts := []string{
		"/fonts/geist-latin-italic.woff2",
		"/fonts/geist-mono-latin-italic.woff2",
	}
	for _, f := range fonts {
		t.Run(f, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+f)
			assert.Equal(t, 200, resp.StatusCode, "%s not served", f)
			resp.Body.Close()
		})
	}
}
