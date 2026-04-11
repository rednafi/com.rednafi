package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityHeaders verifies the _headers file serves correct security headers.
func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/_headers")

	t.Run("X-Content-Type-Options nosniff", func(t *testing.T) {
		assert.Contains(t, body, "X-Content-Type-Options: nosniff")
	})

	t.Run("X-Frame-Options DENY", func(t *testing.T) {
		assert.Contains(t, body, "X-Frame-Options: DENY")
	})

	t.Run("cache control for assets", func(t *testing.T) {
		assert.Contains(t, body, "max-age=31536000")
		assert.Contains(t, body, "immutable")
	})
}

// TestNoMixedContent verifies no HTTP resources are loaded on HTTPS pages.
// Since we test locally over HTTP, we check that production URLs in the HTML
// use https://, not http://.
func TestNoMixedContent(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	content, err := page.Content()
	require.NoError(t, err)

	// Should not contain http:// links to external resources (except localhost)
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(line, `src="http://`) || strings.Contains(line, `href="http://`) {
			// Allow local references
			if strings.Contains(line, "localhost") || strings.Contains(line, "127.0.0.1") {
				continue
			}
			t.Errorf("potential mixed content: %s", strings.TrimSpace(line))
		}
	}
}

// TestExternalLinksNoOpener verifies ALL external links across multiple pages
// have security attributes.
func TestExternalLinksNoOpener(t *testing.T) {
	t.Parallel()
	pages := []string{
		"/about/",
		"/blogroll/",
		"/appearances/",
	}

	for _, p := range pages {
		t.Run(p, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, p)

			links := page.Locator(`a[href^="http"]:not([href*="rednafi.com"])`)
			count, err := links.Count()
			require.NoError(t, err)

			for i := range count {
				link := links.Nth(i)
				href, _ := link.GetAttribute("href")
				// Skip mailto and javascript links
				if strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "javascript:") {
					continue
				}

				rel, err := link.GetAttribute("rel")
				require.NoError(t, err, "link to %s missing rel", href)
				assert.Contains(t, rel, "noopener", "%s missing noopener on %s", href, p)
				assert.Contains(t, rel, "noreferrer", "%s missing noreferrer on %s", href, p)

				target, err := link.GetAttribute("target")
				require.NoError(t, err)
				assert.Equal(t, "_blank", target, "%s missing target=_blank on %s", href, p)
			}
		})
	}
}

// TestSiteVerificationTags checks that search engine verification meta tags are present.
func TestSiteVerificationTags(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("Google verification", func(t *testing.T) {
		content, err := page.Locator(`meta[name="google-site-verification"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.NotEmpty(t, content)
	})

	t.Run("Yandex verification", func(t *testing.T) {
		content, err := page.Locator(`meta[name="yandex-verification"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.NotEmpty(t, content)
	})

	t.Run("Naver verification", func(t *testing.T) {
		content, err := page.Locator(`meta[name="naver-site-verification"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.NotEmpty(t, content)
	})
}
