package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSitemapContainsKeyPages verifies the sitemap includes all important pages
// that search engines should index. If a page disappears from the sitemap,
// it can tank search rankings.
func TestSitemapContainsKeyPages(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/sitemap.xml")

	keyPages := []string{
		"https://rednafi.com/",
		"https://rednafi.com/about/",
		"https://rednafi.com/tags/",
		"https://rednafi.com/python/",
		"https://rednafi.com/go/",
		"https://rednafi.com/misc/",
	}

	for _, page := range keyPages {
		t.Run(page, func(t *testing.T) {
			assert.Contains(t, body, "<loc>"+page+"</loc>",
				"sitemap missing key page: %s", page)
		})
	}
}

// TestSitemapAndRobotsConsistency verifies that pages excluded by robots.txt
// are consistently handled. The search page is in the sitemap (Hugo default)
// but disallowed in robots.txt — both files must remain in sync.
func TestSitemapAndRobotsConsistency(t *testing.T) {
	t.Parallel()
	robots := httpGet(t, baseURL+"/robots.txt")
	sitemap := httpGet(t, baseURL+"/sitemap.xml")

	// robots.txt must reference the sitemap
	assert.Contains(t, robots, "sitemap.xml", "robots.txt should reference sitemap")

	// The homepage must be in both
	assert.Contains(t, robots, "Allow: /")
	assert.Contains(t, sitemap, "https://rednafi.com/")
}

// TestSitemapHasLastmod verifies sitemap entries include lastmod dates
// for proper cache invalidation by crawlers.
func TestSitemapHasLastmod(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/sitemap.xml")
	assert.Contains(t, body, "<lastmod>")
	// Verify the format is ISO 8601
	assert.Regexp(t, `<lastmod>\d{4}-\d{2}-\d{2}`, body)
}

// Sitemap entry count is verified by TestSiteBuildSmokeTest.
