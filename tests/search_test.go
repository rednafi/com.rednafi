package site_test

import (
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchFunctionality tests that Pagefind search actually works end-to-end.
func TestSearchFunctionality(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/search/")

	// Wait for pagefind to load — use generous timeout since Pagefind JS
	// loads deferred and initializes async.
	err := page.Locator(".pagefind-ui__search-input").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err)

	t.Run("search returns results for known term", func(t *testing.T) {
		input := page.Locator(".pagefind-ui__search-input")
		require.NoError(t, input.Fill("python"))
		// Wait for results — Pagefind fetches index chunks async
		err := page.Locator(".pagefind-ui__result").First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(10000),
		})
		require.NoError(t, err)

		count, err := page.Locator(".pagefind-ui__result").Count()
		require.NoError(t, err)
		assert.Greater(t, count, 0, "search for 'python' should return results")
	})

	t.Run("clearing search removes results", func(t *testing.T) {
		input := page.Locator(".pagefind-ui__search-input")
		require.NoError(t, input.Fill(""))
		// Wait for results container to empty
		time.Sleep(500 * time.Millisecond)
		count, err := page.Locator(".pagefind-ui__result").Count()
		require.NoError(t, err)
		assert.Equal(t, 0, count, "empty search should show no results")
	})

	t.Run("pagefind exposes a single search landmark", func(t *testing.T) {
		wrapperRole, err := page.Locator("#search").GetAttribute("role")
		require.NoError(t, err)
		assert.Empty(t, wrapperRole)

		count, err := page.Locator(`[role="search"]`).Count()
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("query is reflected in the URL", func(t *testing.T) {
		input := page.Locator(".pagefind-ui__search-input")
		require.NoError(t, input.Fill("postgres"))
		// the input handler writes ?q= via history.replaceState on the next tick,
		// so poll the URL instead of reading it synchronously (avoids a race)
		assert.Eventually(t, func() bool {
			return strings.Contains(page.URL(), "q=postgres")
		}, 5*time.Second, 50*time.Millisecond, "query should be reflected in the URL")
	})
}

// TestSearchIndexCoversAllSections verifies content from every main section
// (including shards) appears in the Pagefind index. If the pagefind.yml glob
// misses a section, that content silently disappears from search.
func TestSearchIndexCoversAllSections(t *testing.T) {
	t.Parallel()

	// Each entry: a unique term that only appears in that section's content.
	sectionTerms := map[string]string{
		"go":     "goroutine",
		"python": "decorator",
		"shards": "dynamo",
	}

	for section, term := range sectionTerms {
		t.Run(section+"/"+term, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, "/search/")

			err := page.Locator(".pagefind-ui__search-input").WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			})
			require.NoError(t, err)

			require.NoError(t, page.Locator(".pagefind-ui__search-input").Fill(term))
			err = page.Locator(".pagefind-ui__result").First().WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			})
			require.NoError(t, err)

			count, err := page.Locator(".pagefind-ui__result").Count()
			require.NoError(t, err)
			assert.Greater(t, count, 0,
				"search for %q (section %s) should return results — check pagefind.yml glob",
				term, section)
		})
	}
}

// TestSearchPageCSS verifies pagefind CSS overrides are applied.
func TestSearchPageCSS(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/search/")

	err := page.Locator(".pagefind-ui__search-input").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err)

	t.Run("pagefind uses site font", func(t *testing.T) {
		font, err := page.Evaluate(
			`() => getComputedStyle(document.getElementById("search")).getPropertyValue("--pagefind-ui-font")`,
		)
		require.NoError(t, err)
		assert.Contains(t, font, "Geist")
	})

	t.Run("search highlights use Geist amber", func(t *testing.T) {
		input := page.Locator(".pagefind-ui__search-input")
		require.NoError(t, input.Fill("foo"))
		err := page.Locator("#search mark").First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(10000),
		})
		require.NoError(t, err)

		light, err := page.Locator("#search mark").First().Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "rgb(255, 221, 143)", light)

		require.NoError(t, page.Locator("button[data-theme-set='dark']").Click())
		dark, err := page.Locator("#search mark").First().Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "rgb(107, 65, 5)", dark)
	})
}
