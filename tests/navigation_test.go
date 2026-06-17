package site_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectSidebarLinks verifies the /about page "Connect" section contains
// all expected social links with correct destinations.
func TestConnectSidebarLinks(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/about/")

	// Find the Connect section by heading.
	connect := page.Locator(".aside-section").Filter(playwright.LocatorFilterOptions{
		HasText: "Connect",
	})
	heading, err := connect.Locator(".aside-section-title").TextContent()
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

	t.Run("connect links have SVG icons", func(t *testing.T) {
		svgs, err := connect.Locator("svg").Count()
		require.NoError(t, err)
		assert.Equal(t, count, svgs, "each connect link should have one icon")
	})

	t.Run("icons inherit link color", func(t *testing.T) {
		ok, err := connect.Locator("svg").EvaluateAll(
			`els => els.every(el =>
				el.getAttribute("fill") === "currentColor"
			)`,
		)
		require.NoError(t, err)
		assert.True(t, ok.(bool), "connect icons should inherit link color")
	})
}

// TestInternalNavigationStaysOnLocalOrigin verifies browser-facing internal
// links stay relative, so local runs do not jump to the production domain.
func TestInternalNavigationStaysOnLocalOrigin(t *testing.T) {
	t.Parallel()

	t.Run("homepage post links stay local when clicked", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")

		link := page.Locator(`.home-feed .post-list .post > a`).First()
		href, err := link.GetAttribute("href")
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(href, "/"), "post href should be root-relative: %s", href)
		require.NotContains(t, href, "rednafi.com")

		require.NoError(t, link.Click())
		assertLocalURL(t, page)
	})

	t.Run("same-site absolute Markdown links render relative", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/zephyr/footnotes-for-the-win/")

		hrefsRaw, err := page.Locator(`.article-content a[href]`).EvaluateAll(
			`els => els.map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		hrefs := toStringSlice(hrefsRaw)
		require.Contains(t, hrefs, "/")
		require.NotContains(t, hrefs, "https://rednafi.com")
	})

	t.Run("search result links stay local when clicked", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/search/")

		err := page.Locator(".pagefind-ui__search-input").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(10000),
		})
		require.NoError(t, err)
		require.NoError(t, page.Locator(".pagefind-ui__search-input").Fill("postgres"))

		result := page.Locator(".pagefind-ui__result-link").First()
		err = result.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(10000),
		})
		require.NoError(t, err)

		href, err := result.GetAttribute("href")
		require.NoError(t, err)
		require.NotContains(t, href, "rednafi.com")

		require.NoError(t, result.Click())
		assertLocalURL(t, page)
	})

	t.Run("alias refresh stays local", func(t *testing.T) {
		page := newPage(t)
		_, err := page.Goto(baseURL + "/misc/dns_record_to_share_text/")
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return strings.HasPrefix(page.URL(), baseURL+"/misc/dns-record-to-share-text/")
		}, 2*time.Second, 50*time.Millisecond)
		assertLocalURL(t, page)
	})
}

func assertLocalURL(t *testing.T, page playwright.Page) {
	t.Helper()
	require.Eventually(t, func() bool {
		return strings.HasPrefix(page.URL(), baseURL+"/")
	}, 2*time.Second, 50*time.Millisecond)
	require.NotContains(t, page.URL(), "rednafi.com")
}

// TestTypeLabels verifies the post list correctly labels posts as
// "article", "shard", or a custom type_label from frontmatter.
func TestCategoryChips(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	posts := page.Locator(".post-list .post")
	count, err := posts.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0)

	// Each post shows a section category chip (.post-cat) that links to its section.
	chipFound := false
	for i := range min(count, 30) {
		chip := posts.Nth(i).Locator(".post-cat")
		chipCount, err := chip.Count()
		require.NoError(t, err)
		if chipCount == 0 {
			continue
		}
		chipFound = true

		text, err := chip.First().TextContent()
		require.NoError(t, err)
		assert.NotEmpty(t, strings.TrimSpace(text),
			"category chip should have a non-empty label")

		href, err := chip.First().GetAttribute("href")
		require.NoError(t, err)
		assert.Regexp(t, `^/[a-z0-9-]+/$`, href,
			"category chip should link to a section, got %q", href)
	}

	assert.True(t, chipFound, "homepage posts should show a category chip")
}

// TestArchiveYearAnchorNavigation verifies year headings in the archive are
// linkable via anchor fragments and month section headings are not rendered.
func TestArchiveYearAnchorNavigation(t *testing.T) {
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

	t.Run("archive has year post counts", func(t *testing.T) {
		sups := page.Locator(".archive-year h2 sup")
		count, err := sups.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "year headings should show post counts")
	})

	t.Run("archive has no month section headings", func(t *testing.T) {
		count, err := page.Locator(".archive-month, .archive-year h3").Count()
		require.NoError(t, err)
		assert.Equal(t, 0, count)
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
		assert.Contains(t, strings.ToLower(text), "prev", "pagination should have prev link")
	})

	t.Run("has next link", func(t *testing.T) {
		text, err := pag.TextContent()
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(text), "next", "pagination on page 2 should have next link")
	})
}
