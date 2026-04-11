package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHideBreadcrumbs verifies pages with hideBreadcrumbs: true in frontmatter
// do not render breadcrumb navigation. The about, appearances, and blogroll
// pages use this to present a cleaner layout.
func TestHideBreadcrumbs(t *testing.T) {
	t.Parallel()
	pages := []string{"/about/", "/appearances/", "/blogroll/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)
			count, err := page.Locator("nav.breadcrumbs").Count()
			require.NoError(t, err)
			assert.Equal(t, 0, count,
				"%s has hideBreadcrumbs: true but breadcrumbs are visible", url)
		})
	}

	// Contrast: a normal article SHOULD have breadcrumbs
	t.Run("article has breadcrumbs", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/go/anemic-stack-traces/")
		count, err := page.Locator("nav.breadcrumbs").Count()
		require.NoError(t, err)
		assert.Equal(t, 1, count, "normal article should show breadcrumbs")
	})
}

// TestHideMeta verifies pages with hideMeta: true do not show the post-meta
// section (date). These are evergreen pages like about, blogroll, maxims.
func TestHideMeta(t *testing.T) {
	t.Parallel()
	pages := []string{"/about/", "/appearances/", "/blogroll/", "/maxims/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)
			count, err := page.Locator(".post-meta").Count()
			require.NoError(t, err)
			assert.Equal(t, 0, count,
				"%s has hideMeta: true but post-meta is visible", url)
		})
	}

	// Contrast: a normal article SHOULD have post-meta
	t.Run("article has post-meta", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/go/anemic-stack-traces/")
		count, err := page.Locator(".post-meta").Count()
		require.NoError(t, err)
		assert.Equal(t, 1, count, "normal article should show post-meta")
	})
}

// TestHideRelated verifies pages with hideRelated: true do not show the
// related posts section. These pages aren't articles and shouldn't suggest
// "related" content.
func TestHideRelated(t *testing.T) {
	t.Parallel()
	pages := []string{"/about/", "/appearances/", "/blogroll/", "/maxims/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)
			count, err := page.Locator("nav.related-posts").Count()
			require.NoError(t, err)
			assert.Equal(t, 0, count,
				"%s has hideRelated: true but related posts are visible", url)
		})
	}
}

// TestRobotsNoIndex verifies pages with robotsNoIndex: true get a noindex
// robots meta tag, preventing search engines from indexing utility pages.
func TestRobotsNoIndex(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/search/")

	robots, err := page.Locator(`meta[name="robots"]`).GetAttribute("content")
	require.NoError(t, err)
	assert.Contains(t, robots, "noindex",
		"search page should have noindex robots meta")
}

// TestRobotsIndexOnRegularPages contrasts with the above — regular pages
// should have index, follow.
func TestRobotsIndexOnRegularPages(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/about/", "/go/anemic-stack-traces/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)
			robots, err := page.Locator(`meta[name="robots"]`).GetAttribute("content")
			require.NoError(t, err)
			assert.Equal(t, "index, follow", robots,
				"%s should be indexable", url)
		})
	}
}
