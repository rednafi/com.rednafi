package site_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCharsetAndViewportOnAllPageTypes verifies every page type has the
// critical meta tags needed for correct rendering. Missing charset causes
// encoding bugs; missing viewport breaks mobile rendering.
func TestCharsetAndViewportOnAllPageTypes(t *testing.T) {
	t.Parallel()
	pages := map[string]string{
		"homepage": "/",
		"article":  "/go/anemic-stack-traces/",
		"about":    "/about/",
		"archive":  "/archive/",
		"tags":     "/tags/",
		"section":  "/python/",
		"search":   "/search/",
		"404":      "/404.html",
	}

	for name, url := range pages {
		t.Run(name, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			charset, err := page.Locator(`meta[charset]`).GetAttribute("charset")
			require.NoError(t, err, "%s missing charset", name)
			assert.Equal(t, "utf-8", charset, "%s charset should be utf-8", name)

			viewport, err := page.Locator(`meta[name="viewport"]`).GetAttribute("content")
			require.NoError(t, err, "%s missing viewport", name)
			assert.Contains(t, viewport, "width=device-width", "%s missing width=device-width", name)
		})
	}
}

// TestContentColumnMaxWidth verifies the content-column never exceeds 720px
// across page types. If max-width breaks, text becomes unreadable on wide
// screens — a major readability regression.
func TestContentColumnMaxWidth(t *testing.T) {
	t.Parallel()
	pages := []string{"/about/", "/go/anemic-stack-traces/", "/archive/"}

	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			mw, err := page.Locator(".content-column").Evaluate(
				`el => getComputedStyle(el).maxWidth`, nil,
			)
			require.NoError(t, err)
			assert.Equal(t, "720px", mw,
				"content-column max-width should be 720px on %s", url)
		})
	}
}

// TestReadingNotInMainSections verifies reading entries don't appear in the
// main homepage post list (only in the sidebar). Reading entries use a
// separate content flow and shouldn't dilute the article/shard stream.
func TestReadingNotInMainSections(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	// Get all post links from the main post list (not sidebar)
	hrefs, err := page.Locator(".article-list .post-list .post a:not(.type-label)").EvaluateAll(
		`els => els.map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	hrefList := toStringSlice(hrefs)

	for _, h := range hrefList {
		assert.NotContains(t, h, "/reading/",
			"main post list should not contain reading entries")
	}
}

// TestNoOrphanedPages verifies key pages are reachable through navigation.
// An orphaned page exists but has no inbound link — users can never find it.
func TestNoOrphanedPages(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	// Collect all links from homepage (navigation, sidebar, post list)
	allHrefs, err := page.Locator("a[href]").EvaluateAll(
		`els => els.map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	hrefList := toStringSlice(allHrefs)

	// Key pages that must be linked from homepage
	mustLink := []string{
		"/about/", "/archive/", "/search/", "/tags/",
		"/appearances/", "/blogroll/", "/maxims/",
	}

	for _, target := range mustLink {
		assert.True(t, slices.Contains(hrefList, target),
			"%s should be linked from homepage", target)
	}
}

// TestHTMLLangOnAllPages verifies the lang attribute is set on every page
// type. Missing lang breaks screen readers and translation tools.
func TestHTMLLangOnAllPages(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/about/", "/go/anemic-stack-traces/", "/archive/"}

	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)
			lang, err := page.Locator("html").GetAttribute("lang")
			require.NoError(t, err)
			assert.Equal(t, "en", lang, "html lang should be 'en' on %s", url)
		})
	}
}
