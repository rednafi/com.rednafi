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
		"https://rednafi.com/go/anemic-stack-traces/",
		"https://rednafi.com/python/dataclasses/",
		"https://rednafi.com/shards/2026/04/dynamo/",
	}

	for _, page := range keyPages {
		t.Run(page, func(t *testing.T) {
			assert.Contains(t, body, "<loc>"+page+"</loc>",
				"sitemap missing key page: %s", page)
		})
	}

	assert.NotContains(t, body, "<loc>https://rednafi.com/about/</loc>",
		"removed about page should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/archive/</loc>",
		"archive page should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/appearances/</loc>",
		"appearances page should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/maxims/</loc>",
		"maxims page should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/blogroll/</loc>",
		"noindex utility pages should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/articles/</loc>",
		"duplicate article index should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/page/2/</loc>",
		"paginated homepage pages are noindex and should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/shards/</loc>",
		"duplicate shard index should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/python/</loc>",
		"section pages should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/go/</loc>",
		"section pages should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/misc/</loc>",
		"section pages should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/feed/2025/</loc>",
		"feed detail pages should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/tags/</loc>",
		"taxonomy index pages are noindex and should not be listed in sitemap")
	assert.NotContains(t, body, "<loc>https://rednafi.com/tags/go/</loc>",
		"tag term pages are noindex and should not be listed in sitemap")
}

// TestSitemapAndRobotsConsistency verifies robots.txt keeps crawlers able to
// see page-level noindex directives while still advertising the sitemap.
func TestSitemapAndRobotsConsistency(t *testing.T) {
	t.Parallel()
	robots := httpGet(t, baseURL+"/robots.txt")
	sitemap := httpGet(t, baseURL+"/sitemap.xml")

	// robots.txt must reference the sitemap
	assert.Contains(t, robots, "sitemap.xml", "robots.txt should reference sitemap")
	assert.NotContains(t, robots, "Disallow: /search/",
		"search should be crawlable so Google can see its noindex meta")

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
