package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownMirrorsExist(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		contains []string
	}{
		{
			name: "home",
			path: "/index.md",
			contains: []string{
				"# Redowan's Reflections",
				"Source: https://rednafi.com/",
				"## Recent posts",
			},
		},
		{
			name: "article",
			path: "/go/anemic-stack-traces/index.md",
			contains: []string{
				"# Anemic stack traces in Go",
				"Source: https://rednafi.com/go/anemic-stack-traces/",
				"Tags: Go, Error Handling, Logging",
			},
		},
		{
			name: "archive",
			path: "/archive/index.md",
			contains: []string{
				"# Archive",
				"Chronological archive of published writing.",
			},
		},
		{
			name: "search",
			path: "/search/index.md",
			contains: []string{
				"# Search",
				"excluded from indexing",
				"https://rednafi.com/llms.txt",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := httpGet(t, baseURL+tc.path)
			for _, want := range tc.contains {
				assert.Contains(t, body, want)
			}
		})
	}
}

func TestMarkdownAlternateLinks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "home",
			path: "/",
			want: "https://rednafi.com/index.md",
		},
		{
			name: "article",
			path: "/go/anemic-stack-traces/",
			want: "https://rednafi.com/go/anemic-stack-traces/index.md",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, tc.path)

			hrefs, err := page.Locator(`link[rel="alternate"][type="text/markdown"]`).EvaluateAll(
				`els => els.map(e => e.getAttribute("href"))`,
			)
			require.NoError(t, err)

			found := false
			for _, href := range toStringSlice(hrefs) {
				if strings.Contains(href, tc.want) {
					found = true
					break
				}
			}

			assert.True(t, found, "expected markdown alternate %s on %s", tc.want, tc.path)
		})
	}
}

func TestMarkdownMirrorsNormalizeShortcodes(t *testing.T) {
	t.Parallel()

	t.Run("mermaid blocks stay readable", func(t *testing.T) {
		body := httpGet(t, baseURL+"/system/tap-compare-testing/index.md")
		assert.Contains(t, body, "```mermaid")
		assert.NotContains(t, body, "{{< mermaid >}}")
		assert.NotContains(t, body, "{{</ mermaid >}}")
	})

	t.Run("youtube embeds become plain links", func(t *testing.T) {
		body := httpGet(t, baseURL+"/misc/notes-on-event-driven-systems/index.md")
		assert.Contains(t, body, "[YouTube video](https://www.youtube.com/watch?v=qcJASFx-F5g)")
		assert.NotContains(t, body, "{{< youtube")
	})
}
