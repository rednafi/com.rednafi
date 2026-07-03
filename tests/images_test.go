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
		"/python/redis-cache/",
		"/misc/automerge-dependabot-prs-on-github/",
	}

	tested := false
	for _, url := range articlesWithImages {
		goto_(t, page, url)
		images := page.Locator("article img")
		count, err := images.Count()
		require.NoError(t, err)
		if count == 0 {
			continue
		}
		tested = true

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

				t.Run("intrinsic dimensions", func(t *testing.T) {
					// width/height prevent layout shift
					width, err := img.GetAttribute("width")
					require.NoError(t, err)
					height, err := img.GetAttribute("height")
					require.NoError(t, err)
					assert.NotEmpty(t, width, "img %d missing width", i)
					assert.NotEmpty(t, height, "img %d missing height", i)
				})

				t.Run("no distortion", func(t *testing.T) {
					// height:auto must keep attrs from skewing the rendered box
					ratio, err := img.Evaluate(`async el => {
						el.scrollIntoView();
						if (!el.complete) {
							await new Promise(r => {
								el.onload = r;
								el.onerror = r;
								setTimeout(r, 5000);
							});
						}
						return el.naturalWidth > 0
							? (el.clientWidth / el.clientHeight) / (el.naturalWidth / el.naturalHeight)
							: -1;
					}`, nil)
					require.NoError(t, err)
					if toFloat(ratio) < 0 {
						t.Skip("image did not load; ratio not checkable offline")
					}
					assert.InDelta(t, 1.0, toFloat(ratio), 0.02,
						"img %d rendered aspect ratio deviates from natural", i)
				})
			}
		})
		break // one image-bearing page is enough
	}
	require.True(t, tested, "no test page had images; update articlesWithImages")
}

// TestExternalImageReferrerPolicy checks external images have referrerpolicy="no-referrer".
func TestExternalImageReferrerPolicy(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	// Check multiple articles
	articles := []string{
		"/python/redis-cache/",
		"/misc/automerge-dependabot-prs-on-github/",
	}

	checked := 0
	for _, url := range articles {
		goto_(t, page, url)
		extImages := page.Locator(`article img[src^="http"]`)
		count, err := extImages.Count()
		require.NoError(t, err)
		checked += count
		for i := range count {
			policy, err := extImages.Nth(i).GetAttribute("referrerpolicy")
			require.NoError(t, err)
			assert.Equal(t, "no-referrer", policy, "external img %d at %s missing referrerpolicy", i, url)
		}
	}
	require.Positive(t, checked, "no external images found; update articles list")
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
