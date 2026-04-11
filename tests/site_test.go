package site_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pw      *playwright.Playwright
	browser playwright.Browser
	baseURL string
)

func TestMain(m *testing.M) {
	// Find the public directory
	publicDir := filepath.Join("..", "public")
	if _, err := os.Stat(publicDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "public/ directory not found — run 'make build' first")
		os.Exit(1)
	}

	// Start a static file server
	absDir, _ := filepath.Abs(publicDir)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}
	baseURL = fmt.Sprintf("http://%s", listener.Addr().String())
	srv := &http.Server{Handler: http.FileServer(http.Dir(absDir))}
	go srv.Serve(listener)

	// Start Playwright
	pw, err = playwright.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start playwright: %v\n", err)
		os.Exit(1)
	}
	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: new(true),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not launch browser: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	browser.Close()
	pw.Stop()
	srv.Close()
	listener.Close()
	os.Exit(code)
}

// newPage creates a fresh browser context and page for a test.
func newPage(t *testing.T) playwright.Page {
	t.Helper()
	ctx, err := browser.NewContext()
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })
	page, err := ctx.NewPage()
	require.NoError(t, err)
	return page
}

// newMobilePage creates a page with a mobile viewport.
func newMobilePage(t *testing.T) playwright.Page {
	t.Helper()
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{Width: 390, Height: 844},
	})
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })
	page, err := ctx.NewPage()
	require.NoError(t, err)
	return page
}

func goto_(t *testing.T, page playwright.Page, path string) {
	t.Helper()
	_, err := page.Goto(baseURL + path)
	require.NoError(t, err)
}

// ---------- Structure & Layout ----------

func TestBaseLayout(t *testing.T) {
	t.Parallel()
	t.Run("has correct lang attribute", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		lang, err := page.Locator("html").GetAttribute("lang")
		require.NoError(t, err)
		assert.Equal(t, "en", lang)
	})

	t.Run("has skip-to-content link", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		skip := page.Locator("a.skip-link")
		href, err := skip.GetAttribute("href")
		require.NoError(t, err)
		assert.Equal(t, "#main", href)
		text, err := skip.TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Skip to content", text)
	})

	t.Run("has main element with id=main", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		visible, err := page.Locator("main#main").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
	})

	t.Run("has site header with title and theme toggle", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		header := page.Locator("header.site-header")
		visible, err := header.IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)

		title, err := header.Locator("a.site-title").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Redowan's Reflections", title)

		toggleVisible, err := header.Locator("button.theme-toggle").IsVisible()
		require.NoError(t, err)
		assert.True(t, toggleVisible)
	})

	t.Run("has footer with navigation links", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		hrefs, err := page.Locator("footer a").EvaluateAll(
			`els => els.map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		hrefList := toStringSlice(hrefs)
		assert.Contains(t, hrefList, "/about/")
		assert.Contains(t, hrefList, "/archive/")
		assert.Contains(t, hrefList, "/search/")
		assert.Contains(t, hrefList, "/tags/")
	})

	t.Run("has back-to-top button with aria-label", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		btn := page.Locator("a.back-to-top")
		label, err := btn.GetAttribute("aria-label")
		require.NoError(t, err)
		assert.Equal(t, "Back to top", label)
	})

	t.Run("body has home class on homepage only", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		cls, err := page.Locator("body").GetAttribute("class")
		require.NoError(t, err)
		assert.Contains(t, cls, "home")

		goto_(t, page, "/about/")
		cls2, err := page.Locator("body").GetAttribute("class")
		require.NoError(t, err)
		assert.NotContains(t, cls2, "home")
	})
}

func TestHomepage(t *testing.T) {
	t.Parallel()
	t.Run("sidebar has pages, browse, connect sections", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		headings, err := page.Locator(".aside-section h4").AllTextContents()
		require.NoError(t, err)
		lower := make([]string, len(headings))
		for i, h := range headings {
			lower[i] = strings.ToLower(h)
		}
		// Sidebar sections: pages, browse, recent reads (optional), connect
		assert.Contains(t, lower, "pages")
		assert.Contains(t, lower, "browse")
		assert.Contains(t, lower, "connect")
		assert.GreaterOrEqual(t, len(lower), 3, "sidebar should have at least 3 sections")
	})

	t.Run("pages sidebar has correct links", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		hrefs, err := page.Locator(".aside-section").First().Locator("a").EvaluateAll(
			`els => els.map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		hrefList := toStringSlice(hrefs)
		assert.Contains(t, hrefList, "/about/")
		assert.Contains(t, hrefList, "/appearances/")
		assert.Contains(t, hrefList, "/blogroll/")
		assert.Contains(t, hrefList, "/maxims/")
	})

	t.Run("post list renders with dates and type labels", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		count, err := page.Locator(".post-list .post").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)

		// Check type label
		text, err := page.Locator(".post-list .post").First().Locator(".type-label").TextContent()
		require.NoError(t, err)
		assert.Contains(t, []string{"article", "shard"}, strings.TrimSpace(text))

		// Check time element
		visible, err := page.Locator(".post-list .post").First().Locator("time").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
	})

	t.Run("pagination works", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		pag := page.Locator(".pagination")
		count, err := pag.Count()
		require.NoError(t, err)
		if count > 0 {
			nextLink := pag.Locator(`a:has-text("next")`)
			nc, _ := nextLink.Count()
			if nc > 0 {
				err = nextLink.Click()
				require.NoError(t, err)
				assert.Contains(t, page.URL(), "/page/2/")
			}
		}
	})
}

// ---------- Theme Toggle & Reactivity ----------

func TestThemeToggle(t *testing.T) {
	t.Parallel()
	t.Run("defaults to light theme", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		// null or "light" both mean light
		assert.True(t, theme == "" || theme == "light", "expected light, got %q", theme)
	})

	t.Run("clicking toggle switches to dark", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("button.theme-toggle").Click())
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		assert.Equal(t, "dark", theme)
	})

	t.Run("double click returns to light", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("button.theme-toggle").Click())
		require.NoError(t, page.Locator("button.theme-toggle").Click())
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		assert.Equal(t, "light", theme)
	})

	t.Run("persists across navigation via localStorage", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("button.theme-toggle").Click())

		stored, err := page.Evaluate(`() => localStorage.getItem("theme")`)
		require.NoError(t, err)
		assert.Equal(t, "dark", stored)

		goto_(t, page, "/about/")
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		assert.Equal(t, "dark", theme)
	})

	t.Run("respects prefers-color-scheme dark", func(t *testing.T) {
		ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
			ColorScheme: playwright.ColorSchemeDark,
		})
		require.NoError(t, err)
		defer ctx.Close()
		page, err := ctx.NewPage()
		require.NoError(t, err)
		_, err = page.Goto(baseURL + "/")
		require.NoError(t, err)
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		assert.Equal(t, "dark", theme)
	})

	t.Run("localStorage overrides prefers-color-scheme", func(t *testing.T) {
		ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
			ColorScheme: playwright.ColorSchemeDark,
		})
		require.NoError(t, err)
		defer ctx.Close()
		page, err := ctx.NewPage()
		require.NoError(t, err)
		_, err = page.Goto(baseURL + "/")
		require.NoError(t, err)
		_, err = page.Evaluate(`() => localStorage.setItem("theme", "light")`)
		require.NoError(t, err)
		_, err = page.Goto(baseURL + "/")
		require.NoError(t, err)
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		assert.True(t, theme == "" || theme == "light")
	})

	t.Run("CSS variables change with theme", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		lightBg, err := page.Evaluate(`() => getComputedStyle(document.documentElement).getPropertyValue("--bg").trim()`)
		require.NoError(t, err)
		require.NoError(t, page.Locator("button.theme-toggle").Click())
		darkBg, err := page.Evaluate(`() => getComputedStyle(document.documentElement).getPropertyValue("--bg").trim()`)
		require.NoError(t, err)
		assert.NotEqual(t, lightBg, darkBg)
		assert.Equal(t, "#212529", darkBg)
	})
}

func TestBackToTop(t *testing.T) {
	t.Parallel()
	t.Run("hidden initially, appears on scroll", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/archive/")
		btn := page.Locator("a.back-to-top")

		hasClass, err := btn.Evaluate(`el => el.classList.contains("visible")`, nil)
		require.NoError(t, err)
		assert.False(t, hasClass.(bool))

		_, err = page.Evaluate(`() => window.scrollTo(0, 500)`)
		require.NoError(t, err)
		time.Sleep(300 * time.Millisecond)

		hasClass, err = btn.Evaluate(`el => el.classList.contains("visible")`, nil)
		require.NoError(t, err)
		assert.True(t, hasClass.(bool))
	})

	t.Run("clicking scrolls to top", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/archive/")
		_, err := page.Evaluate(`() => window.scrollTo(0, 500)`)
		require.NoError(t, err)
		time.Sleep(300 * time.Millisecond)
		require.NoError(t, page.Locator("a.back-to-top").Click())
		time.Sleep(500 * time.Millisecond)
		scrollY, err := page.Evaluate(`() => window.scrollY`)
		require.NoError(t, err)
		assert.EqualValues(t, 0, scrollY)
	})
}

// ---------- SEO & Meta Tags ----------

func TestSEOHomepage(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("correct title", func(t *testing.T) {
		title, err := page.Title()
		require.NoError(t, err)
		assert.Equal(t, "Redowan's Reflections", title)
	})

	t.Run("meta description", func(t *testing.T) {
		desc, err := page.Locator(`meta[name="description"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Greater(t, len(desc), 10)
	})

	t.Run("canonical URL", func(t *testing.T) {
		canonical, err := page.Locator(`link[rel="canonical"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Contains(t, canonical, "rednafi.com")
	})

	t.Run("robots meta", func(t *testing.T) {
		robots, err := page.Locator(`meta[name="robots"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "index, follow", robots)
	})

	// OG tags, Twitter Card, and JSON-LD schema are tested in detail
	// by TestOGTagsOnAllPageTypes, TestTwitterCardOnAllPages,
	// TestHomepageSchemaCompleteness, and TestOGImageDimensions.
	// Here we only verify og:type is "website" (not "article") on homepage.
	t.Run("og:type is website", func(t *testing.T) {
		ogType, err := page.Locator(`meta[property="og:type"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "website", ogType)
	})

	t.Run("sitemap link", func(t *testing.T) {
		href, err := page.Locator(`link[rel="sitemap"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Contains(t, href, "sitemap.xml")
	})

	t.Run("RSS link", func(t *testing.T) {
		count, err := page.Locator(`link[rel="alternate"][type*="xml"]`).Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})

	t.Run("theme-color meta", func(t *testing.T) {
		light, err := page.Locator(`meta[name="theme-color"][media*="light"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "#fefcf6", light)

		dark, err := page.Locator(`meta[name="theme-color"][media*="dark"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "#212529", dark)
	})

	t.Run("favicon links", func(t *testing.T) {
		svg, err := page.Locator(`link[rel="icon"][type="image/svg+xml"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Contains(t, svg, "favicon.svg")

		png, err := page.Locator(`link[rel="icon"][type="image/png"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Contains(t, png, "favicon.png")
	})
}

func TestSEOArticle(t *testing.T) {
	t.Parallel()
	article := findArticle(t, "go")
	page := newPage(t)
	goto_(t, page, article)

	t.Run("title includes site name", func(t *testing.T) {
		title, err := page.Title()
		require.NoError(t, err)
		assert.Contains(t, title, "Redowan's Reflections")
		assert.Greater(t, len(title), 20)
	})

	t.Run("og:type is article", func(t *testing.T) {
		ogType, err := page.Locator(`meta[property="og:type"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Equal(t, "article", ogType)
	})

	t.Run("has article:published_time", func(t *testing.T) {
		pubTime, err := page.Locator(`meta[property="article:published_time"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, pubTime)
	})

	// JSON-LD BlogPosting schema tested in detail by TestArticleSchemaCompleteness (schema_test.go)
}

// ---------- RSS, Robots, Sitemap ----------

func TestRSSFeeds(t *testing.T) {
	t.Parallel()
	// Detailed RSS validation is in rss_deep_test.go.
	// Here we verify the essential structure and atom self-link.
	t.Run("main RSS has atom self-link", func(t *testing.T) {
		body := httpGet(t, baseURL+"/index.xml")
		assert.Contains(t, body, "atom:link", "RSS should have Atom self-link")
	})

	t.Run("RSS feed respects item limit", func(t *testing.T) {
		body := httpGet(t, baseURL+"/index.xml")
		count := strings.Count(body, "<item>")
		assert.LessOrEqual(t, count, 30, "should not exceed pagination limit of 30")
		assert.Greater(t, count, 0)
	})
}

func TestRobotsTxt(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/robots.txt")
	assert.Contains(t, body, "User-agent: *")
	assert.Contains(t, body, "Allow: /")
	assert.Contains(t, body, "Disallow: /search/")
	assert.Contains(t, body, "Sitemap:")
	assert.Contains(t, body, "sitemap.xml")
}

func TestSitemap(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/sitemap.xml")
	assert.Contains(t, body, "<urlset")
	assert.Contains(t, body, "<url>")
	assert.Contains(t, body, "<loc>")
}

// ---------- Content & Features ----------

func TestSingleArticle(t *testing.T) {
	t.Parallel()
	article := findArticle(t, "go")
	page := newPage(t)
	goto_(t, page, article)

	t.Run("has breadcrumbs", func(t *testing.T) {
		crumbs := page.Locator("nav.breadcrumbs")
		visible, err := crumbs.IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)

		hrefs, err := crumbs.Locator("a").EvaluateAll(`els => els.map(e => e.getAttribute("href"))`)
		require.NoError(t, err)
		hrefList := toStringSlice(hrefs)
		assert.Contains(t, hrefList, "/")
	})

	t.Run("has h1 title", func(t *testing.T) {
		visible, err := page.Locator("h1").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
	})

	t.Run("has post meta with date", func(t *testing.T) {
		dt, err := page.Locator(".post-meta time").GetAttribute("datetime")
		require.NoError(t, err)
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, dt)
	})

	t.Run("has article content", func(t *testing.T) {
		text, err := page.Locator("article").TextContent()
		require.NoError(t, err)
		assert.Greater(t, len(text), 100)
	})

	t.Run("has tags", func(t *testing.T) {
		tags := page.Locator("ul.post-tags")
		count, err := tags.Count()
		require.NoError(t, err)
		if count > 0 {
			tagCount, err := tags.Locator("a").Count()
			require.NoError(t, err)
			assert.Greater(t, tagCount, 0)

			href, err := tags.Locator("a").First().GetAttribute("href")
			require.NoError(t, err)
			assert.Contains(t, href, "/tags/")
		}
	})

	// Related posts tested in detail by TestRelatedPostsSection (content_test.go)
	// Syntax highlighting tested by TestSyntaxHighlightingPresence (content_test.go)
}

func TestArticleWithHeadings(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/configure-options/")

	t.Run("has table of contents", func(t *testing.T) {
		toc := page.Locator("details.toc")
		count, err := toc.Count()
		require.NoError(t, err)
		if count > 0 {
			summary, err := toc.Locator("summary").TextContent()
			require.NoError(t, err)
			assert.Equal(t, "Table of contents", summary)

			linkCount, err := toc.Locator("a").Count()
			require.NoError(t, err)
			assert.Greater(t, linkCount, 0)
		}
	})

	t.Run("TOC links point to valid heading IDs", func(t *testing.T) {
		toc := page.Locator("details.toc")
		count, err := toc.Count()
		require.NoError(t, err)
		if count == 0 {
			t.Skip("no TOC on this page")
		}
		hrefs, err := toc.Locator("a").EvaluateAll(`els => els.map(e => e.getAttribute("href"))`)
		require.NoError(t, err)
		for _, h := range toStringSlice(hrefs) {
			if !strings.HasPrefix(h, "#") {
				continue
			}
			id := h[1:]
			found, err := page.Evaluate(`id => !!document.getElementById(id)`, id)
			require.NoError(t, err)
			assert.True(t, found.(bool), "TOC target %s not found", h)
		}
	})

	t.Run("heading anchors have # prefix", func(t *testing.T) {
		anchors := page.Locator("article .anchor")
		count, err := anchors.Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
		for i := range min(count, 10) {
			href, err := anchors.Nth(i).GetAttribute("href")
			require.NoError(t, err)
			assert.Regexp(t, `^#.+`, href)
		}
	})

	t.Run("anchor targets exist", func(t *testing.T) {
		anchors := page.Locator("article .anchor")
		count, err := anchors.Count()
		require.NoError(t, err)
		for i := range min(count, 10) {
			href, err := anchors.Nth(i).GetAttribute("href")
			require.NoError(t, err)
			id := href[1:]
			found, err := page.Evaluate(`id => !!document.getElementById(id)`, id)
			require.NoError(t, err)
			assert.True(t, found.(bool), "target for %s not found", href)
		}
	})
}

func TestArchive(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/archive/")

	t.Run("groups by year", func(t *testing.T) {
		count, err := page.Locator(".archive-year").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})

	t.Run("year headings have anchor links", func(t *testing.T) {
		count, err := page.Locator(".archive-year h2 a").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
		href, err := page.Locator(".archive-year h2 a").First().GetAttribute("href")
		require.NoError(t, err)
		assert.Regexp(t, `^#\d{4}$`, href)
	})

	t.Run("groups by month", func(t *testing.T) {
		count, err := page.Locator(".archive-month").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})

	t.Run("posts have links and dates", func(t *testing.T) {
		first := page.Locator(".archive-month .post").First()
		visible, err := first.Locator("a").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
		tVisible, err := first.Locator("time").IsVisible()
		require.NoError(t, err)
		assert.True(t, tVisible)
	})
}

// TestSearchPage is covered by TestSearchFunctionality (search_test.go) and
// TestSearchPageConfiguration (special_pages_test.go). Removed to avoid
// redundant Pagefind initialization that causes flakiness under load.

func TestTagsPage(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/tags/")

	t.Run("lists tags", func(t *testing.T) {
		count, err := page.Locator(".post-list .post a").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})

	t.Run("tag links resolve", func(t *testing.T) {
		hrefs, err := page.Locator(".post-list .post a").EvaluateAll(
			`els => els.slice(0, 5).map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		for _, h := range toStringSlice(hrefs) {
			resp := httpGetResp(t, resolveURL(h))
			assert.Equal(t, 200, resp.StatusCode, "tag page %s returned %d", h, resp.StatusCode)
			resp.Body.Close()
		}
	})
}

func TestSectionPage(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/python/")

	t.Run("has title and posts", func(t *testing.T) {
		visible, err := page.Locator("h1").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
		count, err := page.Locator(".post-list .post").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})

	t.Run("post links resolve", func(t *testing.T) {
		hrefs, err := page.Locator(".post-list .post a").EvaluateAll(
			`els => els.slice(0, 5).map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		for _, h := range toStringSlice(hrefs) {
			resp := httpGetResp(t, resolveURL(h))
			assert.Equal(t, 200, resp.StatusCode, "%s returned %d", h, resp.StatusCode)
			resp.Body.Close()
		}
	})
}

// ---------- Pagination ----------

func TestPaginationNavigation(t *testing.T) {
	t.Parallel()
	// Verify pagination nav links work correctly across pages.

	t.Run("page 2 has prev and next nav links", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/page/2/")

		pag := page.Locator("nav.pagination")
		text, err := pag.TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "prev", "page 2 should have prev link")
		assert.Contains(t, text, "next", "page 2 should have next link")
	})

	t.Run("page 1 has next but no prev", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")

		pag := page.Locator("nav.pagination")
		count, err := pag.Count()
		require.NoError(t, err)
		if count > 0 {
			text, err := pag.TextContent()
			require.NoError(t, err)
			assert.Contains(t, text, "next")
			assert.NotContains(t, text, "prev",
				"page 1 should not have prev link")
		}
	})
}

func TestAppleTouchIcon(t *testing.T) {
	t.Parallel()
	t.Run("meta tag present", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		href, err := page.Locator(`link[rel="apple-touch-icon"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Contains(t, href, "favicon.png", "apple-touch-icon should reference favicon.png")
	})

	t.Run("icon file served", func(t *testing.T) {
		resp := httpGetResp(t, baseURL+"/favicon.png")
		assert.Equal(t, 200, resp.StatusCode, "favicon.png should be served")
		resp.Body.Close()
	})
}

// ---------- Links ----------

func TestExternalLinks(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	t.Run("have rel=noopener noreferrer and target=_blank", func(t *testing.T) {
		links := page.Locator(`article a[href^="http"]:not([href*="rednafi.com"])`)
		count, err := links.Count()
		require.NoError(t, err)
		for i := range min(count, 20) {
			link := links.Nth(i)
			rel, err := link.GetAttribute("rel")
			require.NoError(t, err)
			assert.Contains(t, rel, "noopener", "link %d missing noopener", i)
			assert.Contains(t, rel, "noreferrer", "link %d missing noreferrer", i)
			target, err := link.GetAttribute("target")
			require.NoError(t, err)
			assert.Equal(t, "_blank", target, "link %d missing target=_blank", i)
		}
	})

	t.Run("internal links do NOT have target=_blank", func(t *testing.T) {
		links := page.Locator(`article a[href^="/"], article a[href^="#"]`)
		count, err := links.Count()
		require.NoError(t, err)
		for i := range min(count, 20) {
			target, err := links.Nth(i).GetAttribute("target")
			require.NoError(t, err)
			assert.Empty(t, target, "internal link %d should not have target", i)
		}
	})
}

func TestKeyPagesReturn200(t *testing.T) {
	t.Parallel()
	pages := []string{
		"/", "/about/", "/appearances/", "/blogroll/",
		"/maxims/", "/archive/", "/search/", "/tags/",
		"/python/", "/go/", "/misc/",
	}
	for _, p := range pages {
		t.Run(p, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+p)
			assert.Equal(t, 200, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

func TestFaviconFilesServed(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"/favicon.svg", "/favicon.png"} {
		t.Run(f, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+f)
			assert.Equal(t, 200, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

// ---------- Accessibility ----------

func TestSemanticHTML(t *testing.T) {
	t.Parallel()
	t.Run("article has exactly one h1", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/go/anemic-stack-traces/")
		count, err := page.Locator("h1").Count()
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("headings follow hierarchy", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/go/configure-options/")
		levels, err := page.Locator("h1, h2, h3, h4, h5, h6").EvaluateAll(
			`els => els.map(e => parseInt(e.tagName[1]))`,
		)
		require.NoError(t, err)
		nums := toFloat64Slice(levels)
		for i := 1; i < len(nums); i++ {
			assert.LessOrEqual(t, nums[i]-nums[i-1], float64(1),
				"heading jump from h%.0f to h%.0f", nums[i-1], nums[i])
		}
	})

	t.Run("theme toggle has aria-label", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		label, err := page.Locator("button.theme-toggle").GetAttribute("aria-label")
		require.NoError(t, err)
		assert.Equal(t, "Toggle theme", label)
	})

	t.Run("time elements have datetime", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		times := page.Locator("time")
		count, err := times.Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0)
		for i := range min(count, 10) {
			dt, err := times.Nth(i).GetAttribute("datetime")
			require.NoError(t, err)
			assert.Regexp(t, `^\d{4}-\d{2}-\d{2}`, dt, "time[%d] bad datetime", i)
		}
	})

	t.Run("keyboard: theme toggle accessible", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("button.theme-toggle").Focus())
		require.NoError(t, page.Keyboard().Press("Enter"))
		theme, err := page.Locator("html").GetAttribute("data-theme")
		require.NoError(t, err)
		assert.Equal(t, "dark", theme)
	})

	t.Run("skip link targets main", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		mainID, err := page.Locator("main").GetAttribute("id")
		require.NoError(t, err)
		assert.Equal(t, "main", mainID)
		skipHref, err := page.Locator("a.skip-link").GetAttribute("href")
		require.NoError(t, err)
		assert.Equal(t, "#main", skipHref)
	})
}

// ---------- Responsive Layout ----------

func TestDesktopLayout(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("homepage sidebar beside article list", func(t *testing.T) {
		display, err := page.Locator(".index").Evaluate(
			`el => getComputedStyle(el).display`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "flex", display)
		visible, err := page.Locator(".index aside").IsVisible()
		require.NoError(t, err)
		assert.True(t, visible)
	})

	t.Run("content column max-width 720px", func(t *testing.T) {
		page2 := newPage(t)
		goto_(t, page2, "/about/")
		mw, err := page2.Locator(".content-column").Evaluate(
			`el => getComputedStyle(el).maxWidth`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "720px", mw)
	})

	t.Run("body max-width 920px", func(t *testing.T) {
		mw, err := page.Locator("body").Evaluate(
			`el => getComputedStyle(el).maxWidth`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "920px", mw)
	})
}

func TestMobileLayout(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/")

	t.Run("sidebar stacks below content", func(t *testing.T) {
		display, err := page.Locator(".index").Evaluate(
			`el => getComputedStyle(el).display`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "block", display)
	})

	t.Run("sidebar is full width", func(t *testing.T) {
		w, err := page.Locator(".index aside").Evaluate(
			`el => parseFloat(getComputedStyle(el).width)`, nil,
		)
		require.NoError(t, err)
		assert.Greater(t, w.(float64), float64(300))
	})

	t.Run("body has reduced padding", func(t *testing.T) {
		p, err := page.Locator("body").Evaluate(
			`el => parseFloat(getComputedStyle(el).paddingLeft)`, nil,
		)
		require.NoError(t, err)
		assert.LessOrEqual(t, p.(float64), float64(16))
	})
}

func TestPrintStyles(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")
	require.NoError(t, page.EmulateMedia(playwright.PageEmulateMediaOptions{
		Media: playwright.MediaPrint,
	}))

	for _, sel := range []string{".site-header", "footer", ".breadcrumbs", ".back-to-top", ".skip-link"} {
		t.Run(sel+" hidden in print", func(t *testing.T) {
			// Use page.Evaluate because locator.Evaluate auto-waits for visibility,
			// which times out on display:none elements.
			display, err := page.Evaluate(
				`sel => getComputedStyle(document.querySelector(sel)).display`, sel,
			)
			require.NoError(t, err)
			assert.Equal(t, "none", display, "%s should be display:none in print", sel)
		})
	}
}

// ---------- Performance ----------

func TestFontLoading(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("Geist fonts preloaded", func(t *testing.T) {
		hrefs, err := page.Locator(`link[rel="preload"][as="font"]`).EvaluateAll(
			`els => els.map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		hrefList := toStringSlice(hrefs)
		assert.True(t, slices.ContainsFunc(hrefList, func(s string) bool { return strings.Contains(s, "geist-latin") }), "missing geist-latin preload")
		assert.True(t, slices.ContainsFunc(hrefList, func(s string) bool { return strings.Contains(s, "geist-mono-latin") }), "missing geist-mono preload")
	})

	t.Run("font preloads have crossorigin", func(t *testing.T) {
		preloads := page.Locator(`link[rel="preload"][as="font"]`)
		count, err := preloads.Count()
		require.NoError(t, err)
		for i := range count {
			co, err := preloads.Nth(i).GetAttribute("crossorigin")
			require.NoError(t, err)
			assert.Equal(t, "anonymous", co)
		}
	})

	t.Run("font files served", func(t *testing.T) {
		for _, f := range []string{"/fonts/geist-latin.woff2", "/fonts/geist-mono-latin.woff2"} {
			resp := httpGetResp(t, baseURL+f)
			assert.Equal(t, 200, resp.StatusCode, "%s not served", f)
			resp.Body.Close()
		}
	})
}

func TestAssetOptimization(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	t.Run("CSS is fingerprinted", func(t *testing.T) {
		href, err := page.Locator(`link[rel="stylesheet"]`).First().GetAttribute("href")
		require.NoError(t, err)
		assert.Regexp(t, `style\.min\.\w+\.css`, href)
	})

	t.Run("CSS file is served", func(t *testing.T) {
		href, err := page.Locator(`link[rel="stylesheet"]`).First().GetAttribute("href")
		require.NoError(t, err)
		resp := httpGetResp(t, baseURL+href)
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("HTML is minified", func(t *testing.T) {
		body := httpGet(t, baseURL+"/")
		assert.Regexp(t, `^<!doctype html>`, strings.TrimSpace(body))
	})
}

func TestDNSPrefetch(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	hrefs, err := page.Locator(`link[rel="dns-prefetch"]`).EvaluateAll(
		`els => els.map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	hrefList := toStringSlice(hrefs)
	assert.Contains(t, hrefList, "https://blob.rednafi.com")
	assert.Contains(t, hrefList, "https://cdn.jsdelivr.net")
}

func TestPageLoadPerformance(t *testing.T) {
	t.Parallel()
	t.Run("no render-blocking scripts in head", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		count, err := page.Locator(`head script[src]:not([defer]):not([async])`).Count()
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("viewport meta is set", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		content, err := page.Locator(`meta[name="viewport"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Contains(t, content, "width=device-width")
		assert.Contains(t, content, "initial-scale=1")
	})
}

func TestReducedMotion(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	require.NoError(t, page.EmulateMedia(playwright.PageEmulateMediaOptions{
		ReducedMotion: playwright.ReducedMotionReduce,
	}))
	goto_(t, page, "/")
	sb, err := page.Evaluate(`() => getComputedStyle(document.documentElement).scrollBehavior`)
	require.NoError(t, err)
	assert.Equal(t, "auto", sb)
}

// ---------- Navigation (footer/sidebar links resolve) ----------

func TestFooterLinksResolve(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	hrefs, err := page.Locator("footer a").EvaluateAll(
		`els => els.map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	for _, h := range toStringSlice(hrefs) {
		resp := httpGetResp(t, baseURL+h)
		assert.Equal(t, 200, resp.StatusCode, "%s returned %d", h, resp.StatusCode)
		resp.Body.Close()
	}
}

func TestSidebarLinksResolve(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	hrefs, err := page.Locator(".aside-section a").EvaluateAll(
		`els => els.map(e => e.getAttribute("href")).filter(h => h && h.startsWith("/"))`,
	)
	require.NoError(t, err)
	for _, h := range toStringSlice(hrefs) {
		resp := httpGetResp(t, baseURL+h)
		assert.Equal(t, 200, resp.StatusCode, "%s returned %d", h, resp.StatusCode)
		resp.Body.Close()
	}
}

// ---------- Helpers ----------

func httpGet(t *testing.T, url string) string {
	t.Helper()
	resp := httpGetResp(t, url)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

func httpGetResp(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	return resp
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(arr))
	for i, item := range arr {
		s, _ := item.(string)
		out[i] = s
	}
	return out
}

func toFloat64Slice(v any) []float64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]float64, len(arr))
	for i, item := range arr {
		f, _ := item.(float64)
		out[i] = f
	}
	return out
}

// findArticle discovers an article URL from a given section by checking the
// archive page. Returns a local path like "/go/some-article/". This avoids
// hardcoding slugs that may be renamed or deleted.
func findArticle(t *testing.T, section string) string {
	t.Helper()
	body := httpGet(t, baseURL+"/sitemap.xml")
	prefix := "https://rednafi.com/" + section + "/"
	for part := range strings.SplitSeq(body, "<loc>") {
		loc, _, found := strings.Cut(part, "</loc>")
		if !found {
			continue
		}
		loc = strings.TrimSpace(loc)
		if strings.HasPrefix(loc, prefix) && loc != prefix {
			return strings.Replace(loc, "https://rednafi.com", "", 1)
		}
	}
	t.Fatalf("no article found in section %s", section)
	return ""
}

// requirePage verifies a path returns 200, skipping the test if not.
// Use this with hardcoded article paths so tests degrade gracefully
// when content is renamed or deleted.
func requirePage(t *testing.T, path string) {
	t.Helper()
	resp := httpGetResp(t, baseURL+path)
	resp.Body.Close()
	if resp.StatusCode == 404 {
		t.Skipf("page %s no longer exists, skipping", path)
	}
}

// resolveURL converts Hugo's absolute URLs (https://rednafi.com/...) to local server URLs.
func resolveURL(href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		// Replace the production domain with our local test server
		href = strings.Replace(href, "https://rednafi.com", baseURL, 1)
		href = strings.Replace(href, "http://rednafi.com", baseURL, 1)
		return href
	}
	if strings.HasPrefix(href, "/") {
		return baseURL + href
	}
	return baseURL + "/" + href
}
