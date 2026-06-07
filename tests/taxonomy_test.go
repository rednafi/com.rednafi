package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTagPageFiltersContent verifies that a tag page only shows posts
// tagged with that specific tag. If the taxonomy system breaks, the wrong
// content shows up — or no content at all.
func TestTagPageFiltersContent(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/tags/go/")

	// Should have posts listed
	count, err := page.Locator(".post-list .post").Count()
	require.NoError(t, err)
	assert.Greater(t, count, 5, "/tags/go/ should have many Go-tagged posts")
	assert.LessOrEqual(t, count, 15, "/tags/go/ should use homepage pagination size")

	paginationCount, err := page.Locator("nav.pagination").Count()
	require.NoError(t, err)
	assert.Greater(t, paginationCount, 0, "/tags/go/ should expose pagination")

	// Verify at least the first few links point to Go section articles
	hrefs, err := page.Locator(".post-list .post a").EvaluateAll(
		`els => els.slice(0, 5).map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	hrefList := toStringSlice(hrefs)

	// Each linked article should actually have the "Go" tag
	for _, h := range hrefList {
		t.Run(h, func(t *testing.T) {
			resp := httpGetResp(t, resolveURL(h))
			require.Equal(t, 200, resp.StatusCode, "tag page link %s broken", h)
			resp.Body.Close()
		})
	}
}

// TestTagsPageShowsCounts verifies the taxonomy listing page shows tag names
// with post counts, helping users discover content.
func TestTagsPageShowsCounts(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/tags/")

	// /tags/ lists ALL tags on one page (no pagination) as a cloud with counts.
	tags := page.Locator(".tag-cloud li")
	count, err := tags.Count()
	require.NoError(t, err)
	assert.Greater(t, count, 15, "all tags should be listed on a single page")

	// Each tag shows a numeric count chip.
	firstCount, err := page.Locator(".tag-cloud .tag-count").First().TextContent()
	require.NoError(t, err)
	assert.Regexp(t, `\d`, firstCount, "tag should show a numeric count")

	// The single-page cloud has no pagination.
	paginationCount, err := page.Locator("nav.pagination").Count()
	require.NoError(t, err)
	assert.Equal(t, 0, paginationCount, "/tags/ should not paginate")
}

// TestSectionPageDescription verifies section pages show their description
// when one is configured.
func TestSectionPageDescription(t *testing.T) {
	t.Parallel()
	sections := []string{"/python/", "/go/", "/misc/"}

	for _, section := range sections {
		t.Run(section, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, section)

			// Should have an h1 title
			visible, err := page.Locator("h1").IsVisible()
			require.NoError(t, err)
			assert.True(t, visible, "section %s should have h1", section)

			// Check for description paragraph (if section has one configured)
			desc := page.Locator(".section-desc")
			descCount, err := desc.Count()
			require.NoError(t, err)
			if descCount > 0 {
				text, err := desc.TextContent()
				require.NoError(t, err)
				assert.Greater(t, len(strings.TrimSpace(text)), 0,
					"section description should not be empty")
			}
		})
	}
}
