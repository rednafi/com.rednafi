package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotFoundPage(t *testing.T) {
	t.Parallel()
	t.Run("returns 404 for nonexistent paths", func(t *testing.T) {
		paths := []string{
			"/nonexistent-page/",
			"/go/not-a-real-article/",
			"/python/fake-slug/",
			"/zzz/",
		}
		for _, p := range paths {
			resp := httpGetResp(t, baseURL+p)
			assert.Equal(t, 404, resp.StatusCode, "%s should be 404", p)
			resp.Body.Close()
		}
	})

	// Hugo generates a static 404.html — test its content directly
	// (Go's http.FileServer doesn't serve custom error pages for missing routes)
	t.Run("404.html exists and is valid", func(t *testing.T) {
		page := newPage(t)
		_, err := page.Goto(baseURL + "/404.html")
		require.NoError(t, err)

		// Should have site header and footer
		headerVisible, err := page.Locator("header.site-header").IsVisible()
		require.NoError(t, err)
		assert.True(t, headerVisible, "404 page should show site header")

		footerVisible, err := page.Locator("footer").IsVisible()
		require.NoError(t, err)
		assert.True(t, footerVisible, "404 page should show footer")
	})

	t.Run("404.html has noindex robots meta", func(t *testing.T) {
		page := newPage(t)
		_, err := page.Goto(baseURL + "/404.html")
		require.NoError(t, err)

		robots, err := page.Locator(`meta[name="robots"]`).GetAttribute("content")
		require.NoError(t, err)
		assert.Contains(t, robots, "noindex")
	})
}
