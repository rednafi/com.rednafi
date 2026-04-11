package site_test

import (
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
		Timeout: new(10000.0),
	})
	require.NoError(t, err)

	t.Run("search returns results for known term", func(t *testing.T) {
		input := page.Locator(".pagefind-ui__search-input")
		require.NoError(t, input.Fill("python"))
		// Wait for results — Pagefind fetches index chunks async
		err := page.Locator(".pagefind-ui__result").First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: new(10000.0),
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

	t.Run("search has role=search", func(t *testing.T) {
		role, err := page.Locator("#search").GetAttribute("role")
		require.NoError(t, err)
		assert.Equal(t, "search", role)
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
				Timeout: new(10000.0),
			})
			require.NoError(t, err)

			require.NoError(t, page.Locator(".pagefind-ui__search-input").Fill(term))
			err = page.Locator(".pagefind-ui__result").First().WaitFor(playwright.LocatorWaitForOptions{
				Timeout: new(10000.0),
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
		Timeout: new(10000.0),
	})
	require.NoError(t, err)

	t.Run("pagefind uses site font", func(t *testing.T) {
		font, err := page.Evaluate(
			`() => getComputedStyle(document.getElementById("search")).getPropertyValue("--pagefind-ui-font")`,
		)
		require.NoError(t, err)
		assert.Contains(t, font, "Geist")
	})
}
