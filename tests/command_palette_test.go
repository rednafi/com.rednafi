package site_test

import (
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
	count, err := trigger.Count()
	require.NoError(t, err)
	require.Equal(t, 1, count, "search should have one global trigger")
	visible, err := trigger.IsVisible()
	require.NoError(t, err)
	require.True(t, visible, "the search shortcut should be visible in the top bar")

	triggerText, err := trigger.InnerText()
	require.NoError(t, err)
	assert.Contains(t, triggerText, "Search")
	assert.Contains(t, triggerText, "K", "the top bar should advertise the keyboard shortcut")
	labelVisible, err := trigger.Locator(".command-trigger__label").IsVisible()
	require.NoError(t, err)
	assert.True(t, labelVisible)
	shortcutVisible, err := trigger.Locator("kbd").IsVisible()
	require.NoError(t, err)
	assert.True(t, shortcutVisible)
	modifier, err := trigger.Locator("[data-command-modifier]").TextContent()
	require.NoError(t, err)
	assert.Contains(t, []string{"⌘", "Ctrl"}, modifier)
	shortcuts, err := trigger.GetAttribute("aria-keyshortcuts")
	require.NoError(t, err)
	assert.Equal(t, "Meta+K Control+K", shortcuts)
	duplicateCount, err := page.Locator(
		`#site-menu [data-command-open], #site-menu a[href="/search/"], #site-menu [aria-label="Open search"]`,
	).Count()
	require.NoError(t, err)
	assert.Zero(t, duplicateCount, "the menu should not contain a duplicate search control")

	initialResources, err := page.Evaluate(`() =>
		performance.getEntriesByType("resource").map(entry => entry.name)`)
	require.NoError(t, err)
	initialResourceNames := toStringSlice(initialResources)
	assert.True(t, slices.ContainsFunc(initialResourceNames, func(name string) bool {
		return strings.Contains(name, "/js/command-palette-loader.")
	}), "the small shortcut loader should be present")
	assert.False(t, slices.ContainsFunc(initialResourceNames, func(name string) bool {
		return strings.Contains(name, "/js/command-palette.min.")
	}), "the full command controller should wait for search intent")

	require.NoError(t, trigger.Click())
	dialog := page.Locator("#command-palette")
	require.NoError(t, page.Locator("[data-command-ready]").WaitFor())
	require.NoError(t, dialog.WaitFor())
	controllerLoaded, err := page.Evaluate(`() =>
		performance.getEntriesByType("resource").some(entry =>
			entry.name.includes("/js/command-palette.min."))`)
	require.NoError(t, err)
	assert.Equal(t, true, controllerLoaded, "search intent should load the command controller")
	assert.Equal(t, "dialog", locatorAttr(t, dialog, "role"))
	assert.Equal(t, "true", locatorAttr(t, dialog, "aria-modal"))
	labelID := locatorAttr(t, dialog, "aria-labelledby")
	require.Equal(t, 1, locatorCount(t, page.Locator("#"+labelID)))

	input := page.Locator("[data-command-input]")
	assert.Equal(t, "combobox", locatorAttr(t, input, "role"))
	resultsID := locatorAttr(t, input, "aria-controls")
	results := page.Locator("#" + resultsID)
	require.Equal(t, 1, locatorCount(t, results))
	assert.Equal(t, "listbox", locatorAttr(t, results, "role"))

	focused, err := page.Evaluate(`() => document.activeElement && document.activeElement.id`)
	require.NoError(t, err)
	assert.Equal(t, "command-palette-input", focused)
	inert, err := page.Locator(".page").Evaluate(`el => el.inert`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, inert, "the background should be inert while the modal is open")
	pagefindLoaded, err := page.Evaluate(`() =>
		performance.getEntriesByType("resource").some(e => e.name.includes("/pagefind/"))`)
	require.NoError(t, err)
	assert.Equal(t, false, pagefindLoaded, "opening an empty palette should not load Pagefind")

	for _, href := range []string{"/about/", "/appearances/", "/blogroll/"} {
		count, err := page.Locator(
			`#command-palette-results [data-command-result][href="` + href + `"]`,
		).Count()
		require.NoError(t, err)
		assert.Equalf(t, 1, count, "%s should be an immediate quick link", href)
	}

	require.NoError(t, page.Keyboard().Press("Shift+Tab"))
	onClose, err := page.Evaluate(
		`() => document.activeElement && document.activeElement.hasAttribute("data-command-close")`,
	)
	require.NoError(t, err)
	assert.Equal(t, true, onClose)
	require.NoError(t, page.Keyboard().Press("Tab"))
	focused, err = page.Evaluate(`() => document.activeElement && document.activeElement.id`)
	require.NoError(t, err)
	assert.Equal(t, "command-palette-input", focused)

	initialActive, err := input.GetAttribute("aria-activedescendant")
	require.NoError(t, err)
	require.NotEmpty(t, initialActive)
	optionCount := locatorCount(t, results.Locator(`[role="option"]`))
	require.Greater(t, optionCount, 1)
	lastOptionID := locatorAttr(t, results.Locator(`[role="option"]`).Nth(optionCount-1), "id")
	require.NoError(t, page.Keyboard().Press("ArrowUp"))
	wrappedActive, err := input.GetAttribute("aria-activedescendant")
	require.NoError(t, err)
	assert.Equal(t, lastOptionID, wrappedActive, "ArrowUp on the first result should wrap")
	require.NoError(t, page.Keyboard().Press("ArrowDown"))
	returnedActive, err := input.GetAttribute("aria-activedescendant")
	require.NoError(t, err)
	assert.Equal(t, initialActive, returnedActive)
	assert.Equal(t, 1, locatorCount(t, results.Locator(`[role="option"][aria-selected="true"]`)))
	require.Equal(t, 1, locatorCount(t, page.Locator("#"+returnedActive)))

	require.NoError(t, input.Fill("goroutine"))
	result := page.Locator(
		`#command-palette-results [data-command-source="pagefind"]`,
	).First()
	require.NoError(t, result.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	}))
	pagefindLoaded, err = page.Evaluate(`() =>
		performance.getEntriesByType("resource").some(e => e.name.includes("/pagefind/pagefind.js"))`)
	require.NoError(t, err)
	assert.Equal(t, true, pagefindLoaded, "Pagefind should load after the first real query")

	require.NoError(t, page.Keyboard().Press("Escape"))
	assert.Eventually(t, func() bool {
		hidden, err := page.Locator("[data-command-palette]").Evaluate(`el => el.hidden`, nil)
		return err == nil && hidden == true
	}, 2*time.Second, 20*time.Millisecond)
	focused, err = page.Evaluate(
		`() => document.activeElement && document.activeElement.hasAttribute("data-command-open")`,
	)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "closing should restore focus to the trigger")
	inert, err = page.Locator(".page").Evaluate(`el => el.inert`, nil)
	require.NoError(t, err)
	assert.Equal(t, false, inert)

	_, err = page.Evaluate(`() => {
		document.dispatchEvent(new KeyboardEvent("keydown", {
			key: "k", ctrlKey: true, shiftKey: true, bubbles: true
		}));
		document.dispatchEvent(new KeyboardEvent("keydown", {
			key: "k", ctrlKey: true, repeat: true, bubbles: true
		}));
	}`)
	require.NoError(t, err)
	hidden, err := page.Locator("[data-command-palette]").Evaluate(`el => el.hidden`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, hidden, "modified and repeated shortcuts should be ignored")

	require.NoError(t, page.Keyboard().Press("Control+k"))
	require.NoError(t, dialog.WaitFor())
	expanded, err := trigger.GetAttribute("aria-expanded")
	require.NoError(t, err)
	assert.Equal(t, "true", expanded)
	require.NoError(t, page.Keyboard().Press("Control+k"))
	require.NoError(t, page.Keyboard().Press("Meta+k"))
	require.NoError(t, dialog.WaitFor())
	require.NoError(t, page.Keyboard().Press("Escape"))

	require.NoError(t, themeButton(t, page, "dark").Focus())
	require.NoError(t, page.Keyboard().Press("Control+k"))
	require.NoError(t, page.Keyboard().Press("Escape"))
	assert.Eventually(t, func() bool {
		navFocused, err := page.Evaluate(
			`() => document.activeElement && document.activeElement.hasAttribute("data-nav-toggle")`,
		)
		return err == nil && navFocused == true
	}, time.Second, 20*time.Millisecond,
		"opening search from the menu should restore focus to the visible menu trigger")
}

func TestCommandPaletteDismissalAndSearchStates(t *testing.T) {
	t.Parallel()

	assertClosed := func(t *testing.T, page playwright.Page) {
		t.Helper()
		hidden, err := page.Locator("[data-command-palette]").Evaluate(`el => el.hidden`, nil)
		require.NoError(t, err)
		assert.Equal(t, true, hidden)
		assert.Equal(t, "false", locatorAttr(t, page.Locator("[data-command-open]"), "aria-expanded"))

		inert, err := page.Locator(".page").Evaluate(`el => el.inert`, nil)
		require.NoError(t, err)
		assert.Equal(t, false, inert)
		bodyLocked, err := page.Locator("body").Evaluate(
			`el => el.classList.contains("command-palette-open")`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, false, bodyLocked)
	}

	t.Run("close button restores trigger focus", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		require.NoError(t, page.Locator(`button[data-command-close]`).Click())
		assertClosed(t, page)

		focused, err := page.Evaluate(
			`() => document.activeElement && document.activeElement.hasAttribute("data-command-open")`,
		)
		require.NoError(t, err)
		assert.Equal(t, true, focused)
	})

	t.Run("backdrop closes and restores trigger focus", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		require.NoError(t, page.Locator(".command-palette__backdrop").Click(
			playwright.LocatorClickOptions{
				Position: &playwright.Position{X: 4, Y: 4},
			},
		))
		assertClosed(t, page)

		focused, err := page.Evaluate(
			`() => document.activeElement && document.activeElement.hasAttribute("data-command-open")`,
		)
		require.NoError(t, err)
		assert.Equal(t, true, focused)
	})

	t.Run("clearing a query restores quick links", func(t *testing.T) {
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
		assert.Equal(t, "Quick links", locatorText(t, page.Locator("[data-command-group-label]")))
		assert.Equal(t, "false", locatorAttr(t, page.Locator("[data-command-results]"), "aria-busy"))
	})

	t.Run("an impossible query settles into the empty state", func(t *testing.T) {
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

		empty := page.Locator("[data-command-empty]")
		require.NoError(t, empty.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(10000),
		}))
		assert.Zero(t, locatorCount(t, page.Locator("[data-command-result]")))
		assert.Equal(t, "false", locatorAttr(t, page.Locator("[data-command-results]"), "aria-busy"))
		assert.Contains(t, locatorText(t, page.Locator("[data-command-status]")), "0 results")
	})

	t.Run("stale search results cannot replace the latest query", func(t *testing.T) {
		page := newPage(t)
		require.NoError(t, page.Route("**/pagefind/pagefind.js", func(route playwright.Route) {
			require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{
				Body: `export async function search(query) {
					await new Promise(resolve => setTimeout(resolve, query === "first-query" ? 700 : 0));
					return { results: [{
						data: async () => ({
							url: "/about/",
							meta: { title: query },
							plain_excerpt: query
						})
					}] };
				}`,
				ContentType: new("application/javascript"),
			}))
		}))
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		input := page.Locator("[data-command-input]")
		require.NoError(t, input.Fill("first-query"))
		page.WaitForTimeout(200)
		require.NoError(t, input.Fill("latest-query"))

		latest := page.Locator(`[data-command-source="pagefind"] strong`)
		require.NoError(t, latest.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: new(float64(10000)),
		}))
		assert.Equal(t, "latest-query", locatorText(t, latest))
		page.WaitForTimeout(800)
		assert.Equal(t, "latest-query", locatorText(t, latest))
	})

	t.Run("search failure preserves matching quick links", func(t *testing.T) {
		page := newPage(t)
		require.NoError(t, page.Route("**/pagefind/pagefind.js", func(route playwright.Route) {
			require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{
				Body: `export async function search() {
					throw new Error("search unavailable");
				}`,
				ContentType: new("application/javascript"),
			}))
		}))
		goto_(t, page, "/")
		require.NoError(t, page.Locator("[data-command-open]").Click())
		require.NoError(t, page.Locator("[data-command-input]").Fill("about profile"))

		status := page.Locator("[data-command-status]")
		assert.Eventually(t, func() bool {
			return strings.Contains(locatorText(t, status), "could not be loaded")
		}, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, 1, locatorCount(t, page.Locator(
			`[data-command-source="quick-link"][href="/about/"]`,
		)))
		assert.Equal(t, "false", locatorAttr(t, page.Locator("[data-command-results]"), "aria-busy"))
	})

	t.Run("controller load failure restores the closed shell", func(t *testing.T) {
		page := newPage(t)
		require.NoError(t, page.Route("**/js/command-palette.min.*.js", func(route playwright.Route) {
			require.NoError(t, route.Abort())
		}))
		goto_(t, page, "/")
		trigger := page.Locator("[data-command-open]")
		require.NoError(t, trigger.Click())

		assert.Eventually(t, func() bool {
			hidden, err := page.Locator("[data-command-palette]").Evaluate(`element => element.hidden`, nil)
			return err == nil && hidden == true
		}, 2*time.Second, 20*time.Millisecond)
		assert.Equal(t, "false", locatorAttr(t, trigger, "aria-expanded"))
		focused, err := trigger.Evaluate(`element => element === document.activeElement`, nil)
		require.NoError(t, err)
		assert.Equal(t, true, focused)
	})
}

func TestCommandPaletteIndexCoverage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		query    string
		selector string
	}{
		{name: "about", query: "oscillate between product", selector: `[href="/about/"]`},
		{name: "blogroll", query: "Anton Zhiyanov", selector: `[href="/blogroll/"]`},
		{name: "talks", query: "Cheap rigor", selector: `[href="/appearances/"]`},
		{name: "go", query: "goroutine", selector: `[href^="/go/"]`},
		{name: "python", query: "decorator", selector: `[href^="/python/"]`},
		{name: "shards", query: "dynamo", selector: `[href^="/shards/"]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, "/")
			require.NoError(t, page.Locator("[data-command-open]").Click())
			require.NoError(t, page.Locator("[data-command-input]").Fill(tc.query))

			result := page.Locator(
				`#command-palette-results [data-command-source="pagefind"]` + tc.selector,
			).First()
			require.NoError(t, result.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			}))
			title, err := result.Locator("strong").TextContent()
			require.NoError(t, err)
			assert.NotEmpty(t, strings.TrimSpace(title))
		})
	}
}

func TestPagefindRootsTrackSiteSections(t *testing.T) {
	t.Parallel()

	var config struct {
		Params struct {
			MainSections []string `yaml:"mainSections"`
			NotesSection string   `yaml:"notesSection"`
		} `yaml:"params"`
	}
	configData, err := os.ReadFile("../config.yml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(configData, &config))

	var pagefind struct {
		Glob string `yaml:"glob"`
	}
	pagefindData, err := os.ReadFile("../pagefind.yml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(pagefindData, &pagefind))

	roots := strings.TrimSuffix(strings.TrimPrefix(pagefind.Glob, "{"), "}/**/*.html")
	got := strings.Split(roots, ",")
	want := append([]string{}, config.Params.MainSections...)
	want = append(
		want,
		config.Params.NotesSection,
		"about",
		"blogroll",
		"appearances",
	)
	assert.ElementsMatch(t, want, got,
		"Pagefind roots should track every writing section and evergreen search page")
}

func TestCommandPaletteResponsiveThemes(t *testing.T) {
	t.Parallel()
	page := newMobilePage(t)
	goto_(t, page, "/")

	icon, err := page.Locator(".command-trigger > svg").IsVisible()
	require.NoError(t, err)
	assert.True(t, icon, "the search icon should remain visible on mobile")
	label, err := page.Locator(".command-trigger__label").IsVisible()
	require.NoError(t, err)
	assert.False(t, label, "the Search label should be hidden on mobile")
	shortcut, err := page.Locator(".command-trigger kbd").IsVisible()
	require.NoError(t, err)
	assert.False(t, shortcut, "desktop keyboard hints should be hidden on mobile")
	accessibleName, err := page.Locator(".command-trigger").GetAttribute("aria-label")
	require.NoError(t, err)
	assert.Equal(t, "Open search", accessibleName)
	triggerBox, err := page.Locator(".command-trigger").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, triggerBox)
	assert.GreaterOrEqual(t, triggerBox.Width, float64(44))
	assert.GreaterOrEqual(t, triggerBox.Height, float64(44))

	fits, err := page.Evaluate(`() => document.documentElement.scrollWidth <= window.innerWidth`)
	require.NoError(t, err)
	assert.Equal(t, true, fits, "the top bar should not overflow the mobile viewport")

	require.NoError(t, page.Locator("[data-command-open]").Click())
	require.NoError(t, page.Locator("[data-command-ready]").WaitFor())
	closeKeyVisible, err := page.Locator(".command-palette__close kbd").IsVisible()
	require.NoError(t, err)
	assert.False(t, closeKeyVisible, "mobile should not show a desktop Escape hint")
	closeIconVisible, err := page.Locator(".command-palette__close-icon").IsVisible()
	require.NoError(t, err)
	assert.True(t, closeIconVisible, "mobile should show a close icon")
	closeSize, err := page.Locator(`button[data-command-close]`).Evaluate(
		`el => [el.offsetWidth, el.offsetHeight]`, nil,
	)
	require.NoError(t, err)
	size := toFloat64Slice(closeSize)
	require.Len(t, size, 2)
	assert.GreaterOrEqual(t, size[0], float64(44))
	assert.GreaterOrEqual(t, size[1], float64(44))

	panel := page.Locator(".command-palette__dialog")
	box, err := panel.BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, box)
	assert.LessOrEqual(t, box.X+box.Width, float64(390))
	assert.LessOrEqual(t, box.Y+box.Height, float64(844))

	light, err := panel.Evaluate(`el => getComputedStyle(el).backgroundColor`, nil)
	require.NoError(t, err)
	_, err = page.Evaluate(
		`() => document.documentElement.setAttribute("data-theme", "dark")`,
	)
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		dark, err := panel.Evaluate(`el => getComputedStyle(el).backgroundColor`, nil)
		return err == nil && dark != light
	}, time.Second, 20*time.Millisecond, "the floating surface should adapt to dark mode")
}

func TestCommandPaletteRespectsReducedMotion(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	require.NoError(t, page.EmulateMedia(playwright.PageEmulateMediaOptions{
		ReducedMotion: playwright.ReducedMotionReduce,
	}))
	goto_(t, page, "/")
	require.NoError(t, page.Locator("[data-command-open]").Click())

	for _, selector := range []string{".command-palette", ".command-palette__dialog"} {
		duration, err := page.Locator(selector).Evaluate(
			`el => getComputedStyle(el).transitionDuration`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "0s", duration, "%s should not transition under reduced motion", selector)
	}
}

func TestCommandPaletteBundleIsMinimal(t *testing.T) {
	t.Parallel()

	required := []string{
		"../public/pagefind/pagefind.js",
		"../public/pagefind/pagefind-worker.js",
		"../public/pagefind/pagefind-entry.json",
		"../public/pagefind/wasm.en.pagefind",
	}
	for _, path := range required {
		_, err := os.Stat(path)
		require.NoErrorf(t, err, "%s is required by the Pagefind API", path)
	}

	pruned := []string{
		"../public/pagefind/pagefind-ui.css",
		"../public/pagefind/pagefind-ui.js",
		"../public/pagefind/pagefind-component-ui.css",
		"../public/pagefind/pagefind-component-ui.js",
		"../public/pagefind/pagefind-highlight.js",
		"../public/pagefind/pagefind-modular-ui.css",
		"../public/pagefind/pagefind-modular-ui.js",
		"../public/pagefind/wasm.unknown.pagefind",
	}
	for _, path := range pruned {
		_, err := os.Stat(path)
		assert.Truef(t, os.IsNotExist(err), "%s should be pruned", path)
	}

	scripts, err := filepath.Glob("../public/js/command-palette.min.*.js")
	require.NoError(t, err)
	require.Len(t, scripts, 1)
	info, err := os.Stat(scripts[0])
	require.NoError(t, err)
	assert.LessOrEqual(t, info.Size(), int64(8*1024),
		"the lazy command controller should stay under 8 KiB minified")

	loaders, err := filepath.Glob("../public/js/command-palette-loader.min.*.js")
	require.NoError(t, err)
	require.Len(t, loaders, 1)
	loaderInfo, err := os.Stat(loaders[0])
	require.NoError(t, err)
	assert.LessOrEqual(t, loaderInfo.Size(), int64(2*1024),
		"the always-on shortcut loader should stay under 2 KiB minified")

	siteScripts, err := filepath.Glob("../public/js/site*.js")
	require.NoError(t, err)
	require.Len(t, siteScripts, 1)
	siteInfo, err := os.Stat(siteScripts[0])
	require.NoError(t, err)
	assert.LessOrEqual(t, siteInfo.Size(), int64(5*1024),
		"the cacheable global UI controller should stay under 5 KiB minified")
	assert.LessOrEqual(t, loaderInfo.Size()+siteInfo.Size(), int64(7*1024),
		"always-on first-party UI JavaScript should stay under 7 KiB minified")

	resp := httpGetResp(t, baseURL+"/search/")
	defer resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode, "the legacy search page should stay removed")
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
