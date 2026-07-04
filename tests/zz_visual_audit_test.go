package site_test

import (
	"os"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

func TestZZVisualAudit(t *testing.T) {
	outDir := os.Getenv("PROBE_OUT")
	if outDir == "" {
		t.Skip("PROBE_OUT not set")
	}
	pages := map[string]string{
		"home":    "/",
		"post":    "/go/rate-limiting-via-nginx/",
		"archive": "/archive/",
		"about":   "/about/",
		"missing": "/definitely-missing-404/",
	}
	for name, path := range pages {
		page := newPage(t)
		goto_(t, page, path)
		_, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path:     new(outDir + "/" + name + "-light.png"),
			FullPage: new(true),
		})
		require.NoError(t, err)
		require.NoError(t, themeButton(t, page, "dark").Click())
		page.WaitForTimeout(600)
		_, err = page.Evaluate(`() => {
			const m = document.querySelector('[data-nav-menu]');
			if (m) { m.hidden = true; m.classList.remove('is-open'); }
		}`)
		require.NoError(t, err)
		_, err = page.Screenshot(playwright.PageScreenshotOptions{
			Path:     new(outDir + "/" + name + "-dark.png"),
			FullPage: new(true),
		})
		require.NoError(t, err)
	}
	for name, path := range map[string]string{"home": "/", "post": "/go/rate-limiting-via-nginx/"} {
		page := newMobilePage(t)
		goto_(t, page, path)
		_, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path:     new(outDir + "/" + name + "-mobile.png"),
			FullPage: new(true),
		})
		require.NoError(t, err)
	}
}
