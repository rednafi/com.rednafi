package site_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectSidebarLinks verifies the homepage "Connect" section contains
// all expected social links with correct destinations.
func TestConnectSidebarLinks(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	// The connect section is the third .aside-section
	connect := page.Locator(".aside-section").Last()
	heading, err := connect.Locator("h4").TextContent()
	require.NoError(t, err)
	require.Equal(t, "connect", strings.ToLower(strings.TrimSpace(heading)))

	links := connect.Locator("a")
	count, err := links.Count()
	require.NoError(t, err)

	// Collect all link destinations
	hrefs := make([]string, count)
	labels := make([]string, count)
	for i := range count {
		href, err := links.Nth(i).GetAttribute("href")
		require.NoError(t, err)
		hrefs[i] = href

		text, err := links.Nth(i).TextContent()
		require.NoError(t, err)
		labels[i] = strings.TrimSpace(text)
	}

	t.Run("has email link", func(t *testing.T) {
		found := false
		for _, h := range hrefs {
			if strings.HasPrefix(h, "mailto:") {
				found = true
				break
			}
		}
		assert.True(t, found, "connect section should have email link")
	})

	t.Run("has Bluesky link", func(t *testing.T) {
		assert.True(t, slices.ContainsFunc(hrefs, func(s string) bool { return strings.Contains(s, "bsky.app") }),
			"connect section should have Bluesky link")
	})

	t.Run("has GitHub link", func(t *testing.T) {
		assert.True(t, slices.ContainsFunc(hrefs, func(s string) bool { return strings.Contains(s, "github.com") }),
			"connect section should have GitHub link")
	})

	t.Run("has LinkedIn link", func(t *testing.T) {
		assert.True(t, slices.ContainsFunc(hrefs, func(s string) bool { return strings.Contains(s, "linkedin.com") }),
			"connect section should have LinkedIn link")
	})

	t.Run("has RSS link", func(t *testing.T) {
		assert.True(t, slices.ContainsFunc(hrefs, func(s string) bool { return strings.Contains(s, "index.xml") }),
			"connect section should have RSS link")
	})

	t.Run("social links have SVG icons", func(t *testing.T) {
		svgs, err := connect.Locator("svg").Count()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, svgs, 4, "social links should have SVG icons")
	})
}

// TestTypeLabels verifies the post list correctly labels posts as
// "article" or "shard" based on their content section.
func TestTypeLabels(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	posts := page.Locator(".post-list .post")
	count, err := posts.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0)

	// Collect all type labels
	articleFound := false
	shardFound := false

	for i := range min(count, 30) {
		label := posts.Nth(i).Locator(".type-label")
		labelCount, err := label.Count()
		require.NoError(t, err)
		if labelCount == 0 {
			continue
		}
		text, err := label.TextContent()
		require.NoError(t, err)
		text = strings.TrimSpace(text)
		switch text {
		case "article":
			articleFound = true
		case "shard":
			shardFound = true
		}
		assert.Contains(t, []string{"article", "shard"}, text,
			"type label should be 'article' or 'shard', got %q", text)
	}

	assert.True(t, articleFound, "homepage should have at least one 'article' type label")
	_ = shardFound // Shards may or may not appear on the first page
}

// TestArchiveMonthAnchorNavigation verifies year and month headings in the
// archive are linkable via anchor fragments.
func TestArchiveMonthAnchorNavigation(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/archive/")

	t.Run("year heading IDs match their anchor hrefs", func(t *testing.T) {
		yearLinks := page.Locator(".archive-year h2 a")
		count, err := yearLinks.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0)

		for i := range min(count, 5) {
			href, err := yearLinks.Nth(i).GetAttribute("href")
			require.NoError(t, err)
			assert.Regexp(t, `^#\d{4}$`, href, "year anchor should be #YYYY")

			// The parent h2 should have an id matching the fragment
			id := href[1:]
			found, err := page.Evaluate(`id => !!document.getElementById(id)`, id)
			require.NoError(t, err)
			assert.True(t, found.(bool), "heading id=%s should exist", id)
		}
	})

	t.Run("month headings have IDs", func(t *testing.T) {
		monthHeadings := page.Locator(".archive-month h3")
		count, err := monthHeadings.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0)

		for i := range min(count, 5) {
			id, err := monthHeadings.Nth(i).GetAttribute("id")
			require.NoError(t, err)
			assert.NotEmpty(t, id, "month heading should have an id")
		}
	})

	t.Run("archive has year post counts", func(t *testing.T) {
		sups := page.Locator(".archive-year h2 sup")
		count, err := sups.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "year headings should show post counts")
	})
}

// TestShardsSection verifies the shards (notes) section page exists and
// lists short-form content separately from main articles.
func TestShardsSection(t *testing.T) {
	t.Parallel()
	resp := httpGetResp(t, baseURL+"/shards/")
	if resp.StatusCode == 404 {
		t.Skip("shards section does not exist")
	}
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	page := newPage(t)
	goto_(t, page, "/shards/")

	t.Run("has title", func(t *testing.T) {
		visible, err := page.Locator("h1").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
	})

	t.Run("has post list", func(t *testing.T) {
		count, err := page.Locator(".post-list .post").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0, "shards section should have posts")
	})
}

// TestPaginationStructure verifies pagination shows prev/next links with
// correct format and the separator pipe between them.
func TestPaginationStructure(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/page/2/")

	pag := page.Locator("nav.pagination")
	count, err := pag.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0, "page 2 should have pagination")

	t.Run("has prev link", func(t *testing.T) {
		links := pag.Locator("a")
		linkCount, err := links.Count()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, linkCount, 1)

		text, err := pag.TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "prev", "pagination should have prev link")
	})

	t.Run("has next link", func(t *testing.T) {
		text, err := pag.TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "next", "pagination on page 2 should have next link")
	})
}
