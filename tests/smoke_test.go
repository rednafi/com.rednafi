package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSiteBuildSmokeTest is a build-level sanity check that verifies the
// Hugo site produced a reasonable number of pages, RSS feeds, and assets.
// A sudden drop in any count signals a build regression — content not
// being processed, templates failing silently, or config changes breaking
// output generation.
func TestSiteBuildSmokeTest(t *testing.T) {
	t.Parallel()
	t.Run("sitemap has 200+ entries", func(t *testing.T) {
		body := httpGet(t, baseURL+"/sitemap.xml")
		count := strings.Count(body, "<loc>")
		assert.Greater(t, count, 200,
			"sitemap should have 200+ entries (got %d) — content may be missing", count)
	})

	t.Run("homepage has 30 posts", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		count, err := page.Locator(".post-list .post").Count()
		require.NoError(t, err)
		assert.Equal(t, 30, count,
			"homepage should show exactly 30 posts (pagerSize)")
	})

	t.Run("archive has 100+ posts", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/archive/")
		count, err := page.Locator(".archive-month .post").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 100,
			"archive should list 100+ posts (got %d)", count)
	})

	t.Run("main RSS has 20+ items", func(t *testing.T) {
		body := httpGet(t, baseURL+"/index.xml")
		count := strings.Count(body, "<item>")
		assert.Greater(t, count, 20,
			"RSS should have 20+ items (got %d)", count)
	})

	t.Run("CSS file is served and non-empty", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		href, err := page.Locator(`link[rel="stylesheet"]`).First().GetAttribute("href")
		require.NoError(t, err)
		css := httpGet(t, baseURL+href)
		assert.Greater(t, len(css), 5000,
			"CSS should be substantial (got %d bytes)", len(css))
	})

	t.Run("section pages all have posts", func(t *testing.T) {
		for _, section := range []string{"/python/", "/go/", "/misc/"} {
			page := newPage(t)
			goto_(t, page, section)
			count, err := page.Locator(".post-list .post").Count()
			require.NoError(t, err)
			assert.Greater(t, count, 5,
				"%s should have 5+ posts (got %d)", section, count)
		}
	})
}

// TestNoConsoleErrors verifies no JavaScript errors are thrown on key pages.
// Console errors signal broken scripts (theme toggle, back-to-top, Pagefind).
func TestNoConsoleErrors(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/go/anemic-stack-traces/", "/about/"}

	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)

			var errors []string
			page.On("pageerror", func(err error) {
				errors = append(errors, err.Error())
			})

			goto_(t, page, url)

			// Give scripts time to execute
			page.Evaluate(`() => new Promise(r => setTimeout(r, 500))`)

			assert.Empty(t, errors,
				"page %s has JS errors: %v", url, errors)
		})
	}
}
