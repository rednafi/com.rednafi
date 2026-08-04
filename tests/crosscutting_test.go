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

// TestContentColumnMaxWidth verifies every page type uses the same centered
// 720px base wrapper. Purposeful editorial elements such as the homepage hero
// may bleed from that wrapper without changing the underlying column.
func TestContentColumnMaxWidth(t *testing.T) {
	t.Parallel()

	for name, path := range map[string]string{
		"home":    "/",
		"article": "/go/anemic-stack-traces/",
		"archive": "/archive/",
		"section": "/go/",
		"about":   "/about/",
		"404":     "/404.html",
	} {
		t.Run(name, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, path)
			values, err := page.Locator(".content-column").Evaluate(
				`el => ({ maxWidth: getComputedStyle(el).maxWidth, width: el.getBoundingClientRect().width })`, nil,
			)
			require.NoError(t, err)
			metrics := values.(map[string]any)
			assert.Equal(t, "720px", metrics["maxWidth"], "%s max-width drifted", name)
			assert.LessOrEqual(t, toFloat(metrics["width"]), 720.5, "%s exceeds the base column", name)
		})
	}

	t.Run("article reading content", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/go/anemic-stack-traces/")
		width, err := page.Locator(".article-content").Evaluate(
			`el => el.getBoundingClientRect().width`, nil,
		)
		require.NoError(t, err)
		assert.InDelta(t, 720, toFloat(width), 1,
			"desktop article should retain the standard centered 720px reading width")
	})
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
