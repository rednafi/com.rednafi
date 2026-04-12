package site_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeadingIDsAreUnique verifies no duplicate heading IDs exist on a page.
// Duplicate IDs break anchor links and TOC navigation.
func TestHeadingIDsAreUnique(t *testing.T) {
	t.Parallel()
	articles := []string{
		"/go/configure-options/",
		"/go/reminiscing-cgi-scripts/",
	}

	for _, url := range articles {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			ids, err := page.Locator("h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]").EvaluateAll(
				`els => els.map(e => e.id)`,
			)
			require.NoError(t, err)
			idList := toStringSlice(ids)

			seen := make(map[string]bool)
			for _, id := range idList {
				if id == "" {
					continue
				}
				assert.False(t, seen[id], "duplicate heading id=%q on %s", id, url)
				seen[id] = true
			}
		})
	}
}

// TestHeadingAnchorsVisibleOnHover verifies anchor links become visible when heading is hovered.
func TestHeadingAnchorsVisibleOnHover(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/configure-options/")

	anchors := page.Locator("article .anchor")
	count, err := anchors.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0)

	// Anchors should be hidden by default (opacity: 0)
	opacity, err := page.Evaluate(
		`() => getComputedStyle(document.querySelector("article .anchor")).opacity`,
	)
	require.NoError(t, err)
	assert.Equal(t, "0", opacity)

	// Hover over the parent heading — anchor should become visible
	heading := page.Locator("article h2").First()
	require.NoError(t, heading.Hover())
	time.Sleep(300 * time.Millisecond) // wait for opacity transition (150ms)
	opacity2, err := heading.Locator(".anchor").Evaluate(
		`el => getComputedStyle(el).opacity`, nil,
	)
	require.NoError(t, err)
	opStr, _ := opacity2.(string)
	assert.NotEqual(t, "0", opStr, "anchor should be visible on heading hover")
}

// TestDeepAnchorLinkNavigation verifies that navigating to a #hash URL
// scrolls to the correct element.
func TestDeepAnchorLinkNavigation(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/configure-options/")

	// Get the first heading with an anchor
	firstAnchorHref, err := page.Locator("article .anchor").First().GetAttribute("href")
	require.NoError(t, err)
	require.NotEmpty(t, firstAnchorHref)

	// Navigate to the page with the hash
	_, err = page.Goto(baseURL + "/go/configure-options/" + firstAnchorHref)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond) // wait for smooth scroll

	// The target element should be near the top of the viewport
	targetID := firstAnchorHref[1:] // strip #
	top, err := page.Evaluate(
		`id => document.getElementById(id)?.getBoundingClientRect().top`, targetID,
	)
	require.NoError(t, err)
	// Should be near the top of viewport (within ~200px)
	assert.Less(t, top.(float64), float64(200), "anchor target should be near top of viewport")
}
