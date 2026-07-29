package site_test

import (
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
		"archive":  "/archive/",
		"tags":     "/tags/",
		"section":  "/python/",
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
			assert.Contains(t, viewport, "initial-scale=1", "%s missing initial-scale=1", name)
		})
	}
}

// TestContentColumnMaxWidth verifies the content-column never exceeds 720px
// across page types. If max-width breaks, text becomes unreadable on wide
// screens — a major readability regression.
func TestContentColumnMaxWidth(t *testing.T) {
	t.Parallel()
	pages := []string{"/go/anemic-stack-traces/", "/archive/"}

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

func TestArchiveIsDiscoverable(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	allHrefs, err := page.Locator("a[href]").EvaluateAll(
		`els => els.map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	hrefList := toStringSlice(allHrefs)

	assert.Contains(t, hrefList, "/archive/", "archive should be linked from homepage")
}

// TestHTMLLangOnAllPages verifies the lang attribute is set on every page
// type. Missing lang breaks screen readers and translation tools.
func TestHTMLLangOnAllPages(t *testing.T) {
	t.Parallel()
	pages := []string{"/", "/go/anemic-stack-traces/", "/archive/"}

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
