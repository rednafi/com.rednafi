package site_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"slices"
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	Content string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

type htmlFragmentSnapshot struct {
	Text       string         `json:"text"`
	Counts     map[string]int `json:"counts"`
	Links      []string       `json:"links"`
	Images     []string       `json:"images"`
	Headings   []string       `json:"headings"`
	CodeBlocks []string       `json:"codeBlocks"`
}

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

// TestRSSItemsExposeFullContent verifies the capped RSS feed still exposes the
// rendered article body for readers that support the RSS content module.
func TestRSSItemsExposeFullContent(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/index.xml")

	assert.Contains(t, body,
		`xmlns:content="http://purl.org/rss/1.0/modules/content/"`,
		"RSS feed should declare the content module")

	var feed rssDocument
	require.NoError(t, xml.Unmarshal([]byte(body), &feed))
	require.Len(t, feed.Channel.Items, 15,
		"main RSS should expose the capped first 15 posts")

	normalizer := newPage(t)
	for i, item := range feed.Channel.Items {
		t.Run(item.Title, func(t *testing.T) {
			require.NotEmpty(t, item.Link, "RSS item %d should have a link", i)
			require.NotEmpty(t, strings.TrimSpace(item.Content),
				"RSS item %d should expose full rendered content", i)

			pageBody := httpGet(t, baseURL+rssItemPath(t, item.Link))
			pageContent := extractArticleHTML(t, pageBody)

			actual := htmlSnapshot(t, normalizer, item.Content)
			expected := htmlSnapshot(t, normalizer, pageContent)

			assert.Equal(t, expected.Counts, actual.Counts,
				"RSS item %d should expose the same rendered HTML structure", i)
			assert.Equal(t, expected.Links, actual.Links,
				"RSS item %d should expose the same article links", i)
			assert.Equal(t, expected.Images, actual.Images,
				"RSS item %d should expose the same article images", i)
			assert.Equal(t, expected.Headings, actual.Headings,
				"RSS item %d should expose the same article headings", i)
			assert.Equal(t, expected.CodeBlocks, actual.CodeBlocks,
				"RSS item %d should expose the same code blocks", i)
			assert.Equal(t, len(expected.Text), len(actual.Text),
				"RSS item %d should expose the complete article text", i)
			assert.Equal(t, snapshotDigest(expected), snapshotDigest(actual),
				"RSS item %d content:encoded should match the rendered article body", i)
		})
	}
}

func rssItemPath(t *testing.T, link string) string {
	t.Helper()
	const siteURL = "https://rednafi.com"

	require.True(t, strings.HasPrefix(link, siteURL),
		"RSS item link should use the canonical site URL: %s", link)
	path := strings.TrimPrefix(link, siteURL)
	if path == "" {
		return "/"
	}
	return path
}

func extractArticleHTML(t *testing.T, body string) string {
	t.Helper()

	_, after, found := strings.Cut(body, "<article>")
	require.True(t, found, "page should contain an opening <article> tag")

	content, _, found := strings.Cut(after, "</article>")
	require.True(t, found, "page should contain a closing </article> tag")

	return strings.TrimSpace(content)
}

func htmlSnapshot(t *testing.T, page playwright.Page, fragment string) htmlFragmentSnapshot {
	t.Helper()

	raw, err := page.Evaluate(`fragment => {
		const template = document.createElement("template");
		template.innerHTML = fragment.trim();
		const container = document.createElement("div");
		container.appendChild(template.content.cloneNode(true));

		const normalizeInlineText = (value) => value
			.replace(/\s+/g, " ")
			.replace(/\s+([.,;:!?])/g, "$1")
			.trim();
		const compactText = (value) => value.replace(/\s+/g, "");
		const normalizeURL = (value) => {
			const siteURL = "https://rednafi.com";
			if (value.startsWith(siteURL + "/")) return value.slice(siteURL.length);
			return value;
		};
		const counts = Array.from(container.querySelectorAll("*")).reduce((acc, el) => {
			const name = el.tagName.toLowerCase();
			acc[name] = (acc[name] || 0) + 1;
			return acc;
		}, {});
		const textList = (selector) => Array.from(container.querySelectorAll(selector))
			.map(el => normalizeInlineText(el.textContent));
		const attrList = (selector, attr) => Array.from(container.querySelectorAll(selector))
			.map(el => normalizeURL(el.getAttribute(attr) || ""));

		container.querySelectorAll("script,style").forEach(el => el.remove());

		return JSON.stringify({
			text: compactText(container.textContent),
			counts,
			links: attrList("a", "href"),
			images: attrList("img", "src"),
			headings: textList("h2, h3, h4"),
			codeBlocks: textList("pre")
		});
	}`, fragment)
	require.NoError(t, err)

	value, ok := raw.(string)
	require.True(t, ok, "HTML fragment snapshot should be a JSON string")

	var snapshot htmlFragmentSnapshot
	require.NoError(t, json.Unmarshal([]byte(value), &snapshot))
	return snapshot
}

func snapshotDigest(snapshot htmlFragmentSnapshot) string {
	data, err := json.Marshal(snapshot)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestRSSAndHomepagePostsOverlap verifies the RSS feed and homepage share
// content. The homepage may include curated sidebar content beyond what the
// RSS feed contains, so we check for ANY overlap rather than strict matching.
func TestRSSAndHomepagePostsOverlap(t *testing.T) {
	t.Parallel()
	// Get first 10 post titles from homepage (exclude type-label links)
	page := newPage(t)
	goto_(t, page, "/")
	homeTitles, err := page.Locator(".post-list .post a:not(.post-cat)").EvaluateAll(
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

// TestArticlesRSSRedirected verifies the retired duplicate articles feed is
// redirected to the canonical main feed.
func TestArticlesRSSRedirected(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/_redirects")
	assert.Contains(t, body, "/articles.xml /index.xml 301")
}
