package site_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCommandPaletteInteraction(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	trigger := page.Locator("[data-command-open]")
	require.Equal(t, 1, locatorCount(t, trigger))
	assert.Equal(t, "/", locatorAttr(t, trigger, "aria-keyshortcuts"))
	assert.Contains(t, locatorText(t, trigger), "Search")

	resources, err := page.Evaluate(`() => performance.getEntriesByType("resource").map(item => item.name)`)
	require.NoError(t, err)
	names := toStringSlice(resources)
	assert.True(t, slices.ContainsFunc(names, func(name string) bool {
		return strings.Contains(name, "/js/command-palette.min.")
	}), "the single small controller should load with the page")
	assert.False(t, slices.ContainsFunc(names, func(name string) bool {
		return strings.Contains(name, "/pagefind/")
	}), "Pagefind should wait for a query")

	require.NoError(t, trigger.Click())
	dialog := page.Locator("[data-command-palette]")
	open, err := dialog.Evaluate(`element => element.open && element.matches(":modal")`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, open, "search should use the browser's native modal dialog")
	assert.Equal(t, "true", locatorAttr(t, trigger, "aria-expanded"))

	focused, err := page.Evaluate(`() => document.activeElement.id`)
	require.NoError(t, err)
	assert.Equal(t, "command-palette-input", focused)
	assert.Equal(t, 7, locatorCount(t, page.Locator(`[data-command-source="quick-link"]`)))
	assert.Equal(t, 7, locatorCount(t, page.Locator(".command-palette__shortcut")))

	input := page.Locator("[data-command-input]")
	firstActive := locatorAttr(t, input, "aria-activedescendant")
	require.NotEmpty(t, firstActive)
	require.NoError(t, page.Keyboard().Press("ArrowDown"))
	assert.NotEqual(t, firstActive, locatorAttr(t, input, "aria-activedescendant"))

	require.NoError(t, input.Fill("goroutine"))
	result := page.Locator(`[data-command-source="pagefind"]`).First()
	require.NoError(t, result.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	}))
	loaded, err := page.Evaluate(`() => performance.getEntriesByType("resource").some(item => item.name.includes("/pagefind/pagefind.js"))`)
	require.NoError(t, err)
	assert.Equal(t, true, loaded)

	require.NoError(t, page.Keyboard().Press("Escape"))
	assert.Eventually(t, func() bool {
		expanded, err := trigger.GetAttribute("aria-expanded")
		return err == nil && expanded == "false"
	}, time.Second, 20*time.Millisecond)
	focused, err = trigger.Evaluate(`element => element === document.activeElement`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "native close should restore focus")

	require.NoError(t, trigger.Blur())
	require.NoError(t, page.Keyboard().Press("/"))
	open, err = dialog.Evaluate(`element => element.open`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, open)
}

func TestGlobalNavigationShortcuts(t *testing.T) {
	t.Parallel()
	for _, destination := range []struct {
		key  string
		path string
	}{{"h", "/"}, {"a", "/archive/"}, {"t", "/tags/"}, {"p", "/about/"}, {"m", "/maxims/"}, {"b", "/blogroll/"}} {
		t.Run(destination.path, func(t *testing.T) {
			page := newPage(t)
			start := "/"
			if destination.path == "/" {
				start = "/about/"
			}
			goto_(t, page, start)
			require.NoError(t, page.Keyboard().Press("g"))
			require.NoError(t, page.Keyboard().Press(destination.key))
			require.Eventually(t, func() bool {
				return strings.HasSuffix(page.URL(), destination.path)
			}, 2*time.Second, 20*time.Millisecond)
		})
	}

	t.Run("theme", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		_, err := page.Evaluate(`() => document.querySelector('[data-theme-set="light"]').click()`)
		require.NoError(t, err)
		require.NoError(t, page.Keyboard().Press("g"))
		require.NoError(t, page.Keyboard().Press("d"))
		assert.Equal(t, "dark", locatorAttr(t, page.Locator("html"), "data-theme"))
	})
}

func TestCommandPaletteDismissalAndSearchStates(t *testing.T) {
	t.Parallel()

	t.Run("close button restores trigger focus", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		trigger := page.Locator("[data-command-open]")
		require.NoError(t, trigger.Click())
		require.NoError(t, page.Locator("[data-command-close]").Click())
		open, err := page.Locator("[data-command-palette]").Evaluate(`element => element.open`, nil)
		require.NoError(t, err)
		assert.Equal(t, false, open)
		focused, err := trigger.Evaluate(`element => element === document.activeElement`, nil)
		require.NoError(t, err)
		assert.Equal(t, true, focused)
	})

	t.Run("clearing a query restores navigation links", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		input := page.Locator("[data-command-input]")
		require.NoError(t, input.Fill("goroutine"))
		require.NoError(t, page.Locator(`[data-command-source="pagefind"]`).First().WaitFor(
			playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)},
		))
		require.NoError(t, input.Fill(""))
		assert.Equal(t, 7, locatorCount(t, page.Locator(`[data-command-source="quick-link"]`)))
		assert.Equal(t, "Navigate", locatorText(t, page.Locator("[data-command-group-label]")))
	})

	t.Run("an impossible query shows the empty state", func(t *testing.T) {
		page := newPage(t)
		require.NoError(t, page.Route("**/pagefind/pagefind.js", func(route playwright.Route) {
			require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{
				Body:        `export async function search() { return { results: [] }; }`,
				ContentType: new("application/javascript"),
			}))
		}))
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		require.NoError(t, page.Locator("[data-command-input]").Fill("no matches"))
		require.NoError(t, page.Locator("[data-command-empty]").WaitFor(
			playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible},
		))
		assert.Zero(t, locatorCount(t, page.Locator("[data-command-result]")))
	})

	t.Run("stale results cannot replace a newer query", func(t *testing.T) {
		page := newPage(t)
		require.NoError(t, page.Route("**/pagefind/pagefind.js", func(route playwright.Route) {
			require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{
				Body: `export async function search(query) {
					await new Promise(resolve => setTimeout(resolve, query === "first" ? 500 : 0));
					return { results: [{ data: async () => ({ url: "/about/", meta: { title: query }, plain_excerpt: query }) }] };
				}`,
				ContentType: new("application/javascript"),
			}))
		}))
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		input := page.Locator("[data-command-input]")
		require.NoError(t, input.Fill("first"))
		page.WaitForTimeout(150)
		require.NoError(t, input.Fill("latest"))
		latest := page.Locator(`[data-command-source="pagefind"] strong`)
		require.NoError(t, latest.WaitFor())
		assert.Equal(t, "latest", locatorText(t, latest))
		page.WaitForTimeout(600)
		assert.Equal(t, "latest", locatorText(t, latest))
	})
}

func TestCommandPaletteIndexCoverage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		query    string
		selector string
	}{
		{query: "oscillate between product", selector: `[href="/about/"]`},
		{query: "Anton Zhiyanov", selector: `[href="/blogroll/"]`},
		{query: "Cheap rigor", selector: `[href="/appearances/"]`},
		{query: "Chesterton's fence", selector: `[href="/maxims/"]`},
		{query: "goroutine", selector: `[href^="/go/"]`},
		{query: "decorator", selector: `[href^="/python/"]`},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, "/")
			require.NoError(t, page.Locator("[data-command-open]").Click())
			require.NoError(t, page.Locator("[data-command-input]").Fill(tc.query))
			result := page.Locator(`[data-command-source="pagefind"]` + tc.selector).First()
			require.NoError(t, result.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			}))
			assert.NotEmpty(t, locatorText(t, result.Locator("strong")))
		})
	}
}

func TestCommandPaletteIndexesPostTags(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/rate-limiting-via-nginx/")
	metadata := page.Locator(`[data-pagefind-meta="tags"]`)
	require.Equal(t, 1, locatorCount(t, metadata))
	assert.Contains(t, locatorText(t, metadata), "Networking")
	box, err := metadata.BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, box)
	assert.LessOrEqual(t, box.Width, float64(1))
	assert.LessOrEqual(t, box.Height, float64(1))
}

func TestPagefindRootsTrackSiteSections(t *testing.T) {
	t.Parallel()
	var config struct {
		Params struct {
			MainSections []string `yaml:"mainSections"`
			NotesSection string   `yaml:"notesSection"`
		} `yaml:"params"`
	}
	data, err := os.ReadFile("../config.yml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &config))

	var pagefind struct {
		Glob string `yaml:"glob"`
	}
	data, err = os.ReadFile("../pagefind.yml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &pagefind))

	roots := strings.TrimSuffix(strings.TrimPrefix(pagefind.Glob, "{"), "}/**/*.html")
	want := append([]string{}, config.Params.MainSections...)
	want = append(want, config.Params.NotesSection, "about", "blogroll", "appearances", "maxims")
	assert.ElementsMatch(t, want, strings.Split(roots, ","))
}

func TestCommandPaletteResponsiveLayout(t *testing.T) {
	t.Parallel()
	for _, viewport := range []playwright.Size{{Width: 320, Height: 568}, {Width: 844, Height: 390}} {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/")
			require.NoError(t, page.Locator("[data-command-open]").Click())

			fits, err := page.Evaluate(`() => {
				const dialog = document.querySelector('[data-command-palette]').getBoundingClientRect();
				const panel = document.querySelector('.command-palette__dialog').getBoundingClientRect();
				const body = document.querySelector('.command-palette__body');
				return dialog.left >= 0 && dialog.top >= 0 && dialog.right <= innerWidth &&
					panel.bottom <= innerHeight && body.clientHeight <= body.scrollHeight;
			}`)
			require.NoError(t, err)
			assert.Equal(t, true, fits)
		})
	}
}

func TestCommandPaletteBundleIsMinimal(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"../public/pagefind/pagefind.js",
		"../public/pagefind/pagefind-worker.js",
		"../public/pagefind/pagefind-entry.json",
		"../public/pagefind/wasm.en.pagefind",
	} {
		_, err := os.Stat(path)
		require.NoErrorf(t, err, "%s is required by Pagefind", path)
	}

	scripts, err := filepath.Glob("../public/js/command-palette.min.*.js")
	require.NoError(t, err)
	require.Len(t, scripts, 1)
	info, err := os.Stat(scripts[0])
	require.NoError(t, err)
	assert.LessOrEqual(t, info.Size(), int64(6*1024), "the complete controller should stay under 6 KiB minified")

	loaders, err := filepath.Glob("../public/js/command-palette-loader*.js")
	require.NoError(t, err)
	assert.Empty(t, loaders, "the redundant loader should not be shipped")

	resp := httpGetResp(t, baseURL+"/search/")
	defer resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func locatorAttr(t *testing.T, locator playwright.Locator, name string) string {
	t.Helper()
	value, err := locator.GetAttribute(name)
	require.NoError(t, err)
	return value
}

func locatorCount(t *testing.T, locator playwright.Locator) int {
	t.Helper()
	count, err := locator.Count()
	require.NoError(t, err)
	return count
}

func locatorText(t *testing.T, locator playwright.Locator) string {
	t.Helper()
	value, err := locator.TextContent()
	require.NoError(t, err)
	return strings.TrimSpace(value)
}
