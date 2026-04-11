package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOGTagsOnAllPageTypes verifies Open Graph tags are present on every page type.
func TestOGTagsOnAllPageTypes(t *testing.T) {
	t.Parallel()
	pages := map[string]string{
		"homepage": "/",
		"about":    "/about/",
		"article":  "/go/anemic-stack-traces/",
		"section":  "/python/",
		"tag":      "/tags/go/",
	}

	for name, url := range pages {
		t.Run(name, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			ogTitle, err := page.Locator(`meta[property="og:title"]`).GetAttribute("content")
			require.NoError(t, err)
			assert.NotEmpty(t, ogTitle, "og:title missing on %s", url)

			ogDesc, err := page.Locator(`meta[property="og:description"]`).GetAttribute("content")
			require.NoError(t, err)
			assert.NotEmpty(t, ogDesc, "og:description missing on %s", url)

			ogURL, err := page.Locator(`meta[property="og:url"]`).GetAttribute("content")
			require.NoError(t, err)
			assert.NotEmpty(t, ogURL, "og:url missing on %s", url)
		})
	}
}

// TestCanonicalURLsAreAbsolute verifies canonical URLs use absolute paths with production domain.
func TestCanonicalURLsAreAbsolute(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/about/", "/go/anemic-stack-traces/", "/archive/"}

	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			canonical, err := page.Locator(`link[rel="canonical"]`).GetAttribute("href")
			require.NoError(t, err)
			assert.Contains(t, canonical, "https://rednafi.com", "canonical should be absolute on %s", url)
		})
	}
}

// TestArticleOGTags verifies article-specific OG meta.
func TestArticleOGTags(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	t.Run("has article:section", func(t *testing.T) {
		section, err := page.Locator(`meta[property="article:section"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.NotEmpty(t, section)
	})

	t.Run("has article:author", func(t *testing.T) {
		author, err := page.Locator(`meta[property="article:author"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.NotEmpty(t, author)
	})

	t.Run("has article:tag", func(t *testing.T) {
		count, err := page.Locator(`meta[property="article:tag"]`).Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0, "should have at least one article:tag")
	})

	t.Run("has article:modified_time", func(t *testing.T) {
		modTime, err := page.Locator(`meta[property="article:modified_time"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, modTime)
	})
}

// TestTwitterCardOnAllPages verifies Twitter card meta is present everywhere.
func TestTwitterCardOnAllPages(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/about/", "/go/anemic-stack-traces/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			card, err := page.Locator(`meta[name="twitter:card"]`).GetAttribute("content")
			require.NoError(t, err)
			assert.Equal(t, "summary_large_image", card)

			site, err := page.Locator(`meta[name="twitter:site"]`).GetAttribute("content")
			require.NoError(t, err)
			assert.Equal(t, "@rednafi", site)
		})
	}
}
