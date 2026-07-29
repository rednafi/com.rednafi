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
		_, err = page.Evaluate(`() => {
			document.documentElement.setAttribute("data-theme", "dark");
		}`)
		require.NoError(t, err)
		page.WaitForTimeout(300)
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

	page := newPage(t)
	goto_(t, page, "/")
	require.NoError(t, page.Locator("[data-command-open]").Click())
	page.WaitForTimeout(250)
	_, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: new(outDir + "/command-palette-light.png"),
	})
	require.NoError(t, err)
	_, err = page.Evaluate(
		`() => document.documentElement.setAttribute("data-theme", "dark")`,
	)
	require.NoError(t, err)
	page.WaitForTimeout(300)
	_, err = page.Screenshot(playwright.PageScreenshotOptions{
		Path: new(outDir + "/command-palette-dark.png"),
	})
	require.NoError(t, err)

	page = newMobilePage(t)
	goto_(t, page, "/")
	require.NoError(t, page.Locator("[data-command-open]").Click())
	page.WaitForTimeout(250)
	_, err = page.Screenshot(playwright.PageScreenshotOptions{
		Path: new(outDir + "/command-palette-mobile.png"),
	})
	require.NoError(t, err)
}
