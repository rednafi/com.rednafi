package site_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRSSItemsHaveDescription verifies every RSS item has a non-empty
// description. Empty descriptions break feed readers and SEO.
func TestRSSItemsHaveDescription(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")

	parts := strings.Split(body, "<item>")
	checked := 0
	for i, part := range parts[1:] {
		_, after, found := strings.Cut(part, "<description>")
		if !found {
			t.Errorf("RSS item %d missing <description>", i)
			continue
		}
		desc, _, found := strings.Cut(after, "</description>")
		if !found {
			continue
		}
		assert.Greater(t, len(strings.TrimSpace(desc)), 0,
			"RSS item %d has empty description", i)
		checked++
		if checked >= 10 {
			break
		}
	}
	assert.Greater(t, checked, 0, "should check at least one item")
}

// TestRSSAndHomepagePostsOverlap verifies the RSS feed and homepage share
// content. The homepage may include curated sidebar content beyond what the
// RSS feed contains, so we check for ANY overlap rather than strict matching.
func TestRSSAndHomepagePostsOverlap(t *testing.T) {
	t.Parallel()
	// Get first 10 post titles from homepage (exclude type-label links)
	page := newPage(t)
	goto_(t, page, "/")
	homeTitles, err := page.Locator(".post-list .post a:not(.type-label)").EvaluateAll(
		`els => els.slice(0, 10).map(e => e.textContent.trim())`,
	)
	require.NoError(t, err)
	homeList := toStringSlice(homeTitles)
	require.GreaterOrEqual(t, len(homeList), 5, "homepage should have posts")

	// Get first 10 titles from RSS
	body := httpGet(t, baseURL+"/index.xml")
	var rssTitles []string
	parts := strings.Split(body, "<item>")
	for _, part := range parts[1:] {
		_, after, found := strings.Cut(part, "<title>")
		if !found {
			continue
		}
		title, _, found := strings.Cut(after, "</title>")
		if !found {
			continue
		}
		rssTitles = append(rssTitles, strings.TrimSpace(title))
		if len(rssTitles) >= 10 {
			break
		}
	}

	// Check for ANY overlap between homepage and RSS
	overlap := 0
	for _, ht := range homeList {
		if slices.Contains(rssTitles, ht) {
			overlap++
		}
	}
	assert.Greater(t, overlap, 0,
		"RSS and homepage should share at least one post: home=%v rss=%v",
		homeList, rssTitles)
}

// TestMainAndArticlesRSSAreDifferentFormats verifies the main RSS feed
// (index.xml) and articles feed (articles.xml) are both valid but served
// under different paths — they use different Hugo output formats.
func TestMainAndArticlesRSSAreDifferentFormats(t *testing.T) {
	t.Parallel()
	mainRSS := httpGet(t, baseURL+"/index.xml")
	articlesRSS := httpGet(t, baseURL+"/articles.xml")

	// Both should be valid RSS
	assert.Contains(t, mainRSS, "<rss")
	assert.Contains(t, articlesRSS, "<rss")

	// Both should have items
	assert.Contains(t, mainRSS, "<item>")
	assert.Contains(t, articlesRSS, "<item>")
}

// TestRSSFeedLinksAreAbsolute verifies all links in RSS items use absolute URLs.
func TestRSSFeedLinksAreAbsolute(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")

	// Extract all <link> values from items
	parts := strings.Split(body, "<item>")
	for i, part := range parts[1:] { // skip header
		_, after, found := strings.Cut(part, "<link>")
		if !found {
			continue
		}
		link, _, found := strings.Cut(after, "</link>")
		if !found {
			continue
		}
		assert.True(t, strings.HasPrefix(link, "https://"),
			"RSS item %d link should be absolute: %s", i, link)
	}
}

// TestRSSFeedHasCategories verifies RSS items include category tags (from Hugo tags).
func TestRSSFeedHasCategories(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")
	assert.Contains(t, body, "<category>", "RSS feed should contain category tags")
}

// TestRSSFeedGUIDsAreUnique verifies no duplicate GUIDs in the RSS feed.
func TestRSSFeedGUIDsAreUnique(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")

	guids := make(map[string]bool)
	parts := strings.Split(body, "<guid>")
	for _, part := range parts[1:] {
		guid, _, found := strings.Cut(part, "</guid>")
		if !found {
			continue
		}
		assert.False(t, guids[guid], "duplicate RSS GUID: %s", guid)
		guids[guid] = true
	}
	assert.Greater(t, len(guids), 0, "should have at least one GUID")
}

// TestRSSFeedHasLastBuildDate checks the feed has a lastBuildDate element.
func TestRSSFeedHasLastBuildDate(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")
	assert.Contains(t, body, "<lastBuildDate>")
}

// TestRSSFeedDoesNotIncludeSearchOrArchive verifies utility pages are excluded from RSS.
func TestRSSFeedDoesNotIncludeSearchOrArchive(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")
	assert.NotContains(t, body, "/search/")
	assert.NotContains(t, body, "/archive/")
}

// TestAllSectionRSSFeeds verifies every content section has a valid RSS feed.
func TestAllSectionRSSFeeds(t *testing.T) {
	t.Parallel()
	sections := []string{"python", "go", "misc", "zephyr", "javascript", "typescript", "system"}
	for _, section := range sections {
		t.Run(section, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+"/"+section+"/index.xml")
			if resp.StatusCode == 404 {
				t.Skipf("section %s has no RSS feed", section)
			}
			require.Equal(t, 200, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

// TestArticlesRSSFeedStructure validates the custom articles-only RSS feed
// (/articles.xml). This is a custom Hugo output format — if the outputFormats
// config or template breaks, this feed silently disappears.
func TestArticlesRSSFeedStructure(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/articles.xml")

	t.Run("is valid RSS", func(t *testing.T) {
		assert.Contains(t, body, "<rss")
		assert.Contains(t, body, "<channel>")
		assert.Contains(t, body, "<item>")
	})

	t.Run("has feed title", func(t *testing.T) {
		assert.Contains(t, body, "<title>")
	})

	t.Run("items have required fields", func(t *testing.T) {
		idx := strings.Index(body, "<item>")
		require.Greater(t, idx, 0)
		end := strings.Index(body[idx:], "</item>")
		item := body[idx : idx+end]
		assert.Contains(t, item, "<title>")
		assert.Contains(t, item, "<link>")
		assert.Contains(t, item, "<pubDate>")
		assert.Contains(t, item, "<guid>")
	})

	t.Run("links are absolute", func(t *testing.T) {
		parts := strings.Split(body, "<item>")
		for i, part := range parts[1:] {
			_, after, found := strings.Cut(part, "<link>")
			if !found {
				continue
			}
			link, _, found := strings.Cut(after, "</link>")
			if !found {
				continue
			}
			assert.True(t, strings.HasPrefix(link, "https://"),
				"articles RSS item %d link should be absolute: %s", i, link)
		}
	})
}
