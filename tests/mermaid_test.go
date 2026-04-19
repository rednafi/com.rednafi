package site_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mermaidPage struct {
	BlockCount int
	HasFlag    bool
	SourcePath string
	URL        string
}

// TestMermaidMarkdownSafety makes the formatter-sensitive Mermaid shortcode
// layout explicit. If a future edit removes the ignore fence, forgets the
// frontmatter flag, or breaks the shortcode pairing, this test fails before the
// page build/render path is even exercised.
func TestMermaidMarkdownSafety(t *testing.T) {
	t.Parallel()

	pages := scanMermaidPages(t)
	require.NotEmpty(t, pages, "expected at least one Mermaid page in the content tree")

	var violations []string

	for _, page := range pages {
		data, err := os.ReadFile(page.SourcePath)
		require.NoError(t, err)

		lines := strings.Split(string(data), "\n")
		blockCount := 0

		for i := 0; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "{{< mermaid >}}" {
				continue
			}

			blockCount++
			if !nearestNonEmptyAboveIs(lines, i, "<!-- prettier-ignore-start -->") {
				violations = append(
					violations,
					page.SourcePath+":"+strconv.Itoa(i+1)+": Mermaid block must be preceded by prettier-ignore-start",
				)
			}

			end := findMermaidClose(lines, i+1)
			if end == -1 {
				violations = append(
					violations,
					page.SourcePath+":"+strconv.Itoa(i+1)+": Mermaid block is missing a closing shortcode",
				)
				continue
			}

			if !hasNonEmptyLineBetween(lines, i+1, end) {
				violations = append(
					violations,
					page.SourcePath+":"+strconv.Itoa(i+1)+": Mermaid block must contain diagram source lines",
				)
			}

			if !nearestNonEmptyBelowIs(lines, end, "<!-- prettier-ignore-end -->") {
				violations = append(
					violations,
					page.SourcePath+":"+strconv.Itoa(end+1)+": Mermaid block must be followed by prettier-ignore-end",
				)
			}

			i = end
		}

		if page.HasFlag && blockCount == 0 {
			violations = append(
				violations,
				page.SourcePath+": mermaid: true is set but no Mermaid shortcode blocks were found",
			)
		}

		if blockCount > 0 && !page.HasFlag {
			violations = append(
				violations,
				page.SourcePath+": Mermaid shortcode blocks require mermaid: true in frontmatter",
			)
		}

		if blockCount != page.BlockCount {
			violations = append(
				violations,
				page.SourcePath+": Mermaid block count mismatch during scan",
			)
		}
	}

	require.Empty(
		t,
		violations,
		"Mermaid markdown safety checks failed",
	)
}

// TestMermaidPagesRenderAndRerender verifies that Mermaid pages emit the
// runtime script, render all shortcode blocks to SVG, and keep the rerender
// hook alive for theme toggles.
func TestMermaidPagesRenderAndRerender(t *testing.T) {
	t.Parallel()

	pages := scanMermaidPages(t)
	require.NotEmpty(t, pages, "expected Mermaid pages to test")

	for _, tc := range pages {
		name := strings.Trim(tc.URL, "/")
		if name == "" {
			name = "home"
		}

		t.Run(name, func(t *testing.T) {
			requirePage(t, tc.URL)

			page := newPage(t)
			goto_(t, page, tc.URL)

			_, err := page.WaitForFunction(
				`() => typeof window.__mermaidRerender === 'function' &&
					Array.from(document.querySelectorAll('article .mermaid')).every(el =>
						el.getAttribute('data-source') && el.querySelector('svg')
					)`,
				nil,
			)
			require.NoError(t, err)

			scriptCount, err := page.Locator(`script[src*="mermaid.min.js"]`).Count()
			require.NoError(t, err)
			assert.Equal(t, 1, scriptCount, "Mermaid runtime script should be included once")

			blockCount, err := page.Locator("article .mermaid").Count()
			require.NoError(t, err)
			assert.Equal(t, tc.BlockCount, blockCount, "rendered Mermaid block count")

			svgCount, err := page.Locator("article .mermaid svg").Count()
			require.NoError(t, err)
			assert.Equal(t, tc.BlockCount, svgCount, "each Mermaid block should render to SVG")

			require.NoError(t, page.Locator("button.theme-toggle").Click())

			_, err = page.WaitForFunction(
				`() => document.documentElement.getAttribute('data-theme') === 'dark' &&
					typeof window.__mermaidRerender === 'function' &&
					Array.from(document.querySelectorAll('article .mermaid')).every(el =>
						el.getAttribute('data-source') && el.querySelector('svg')
					)`,
				nil,
			)
			require.NoError(t, err)

			darkTheme, err := page.Evaluate(
				`() => document.documentElement.getAttribute("data-theme")`,
			)
			require.NoError(t, err)
			assert.Equal(t, "dark", darkTheme)

			require.NoError(t, page.Locator("button.theme-toggle").Click())

			_, err = page.WaitForFunction(
				`() => document.documentElement.getAttribute('data-theme') === 'light' &&
					typeof window.__mermaidRerender === 'function' &&
					Array.from(document.querySelectorAll('article .mermaid')).every(el =>
						el.getAttribute('data-source') && el.querySelector('svg')
					)`,
				nil,
			)
			require.NoError(t, err)
		})
	}
}

func scanMermaidPages(t *testing.T) []mermaidPage {
	t.Helper()

	var pages []mermaidPage

	err := filepath.WalkDir("../content", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		text := string(data)
		blockCount := strings.Count(text, "{{< mermaid >}}")
		hasFlag := frontmatterValue(text, "mermaid") == "true"

		if blockCount == 0 && !hasFlag {
			return nil
		}

		rel, err := filepath.Rel("../content", path)
		if err != nil {
			return err
		}

		pages = append(pages, mermaidPage{
			BlockCount: blockCount,
			HasFlag:    hasFlag,
			SourcePath: path,
			URL:        contentURL(rel, text),
		})
		return nil
	})
	require.NoError(t, err)

	sort.Slice(pages, func(i, j int) bool {
		return pages[i].URL < pages[j].URL
	})

	return pages
}

func contentURL(relPath string, content string) string {
	if url := frontmatterValue(content, "url"); url != "" {
		if strings.HasSuffix(url, "/") {
			return url
		}
		return url + "/"
	}

	dir := filepath.Dir(relPath)
	base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	slug := frontmatterValue(content, "slug")
	if slug == "" {
		slug = base
	}

	if base == "_index" {
		if dir == "." {
			return "/"
		}
		return "/" + filepath.ToSlash(dir) + "/"
	}

	if dir == "." {
		return "/" + slug + "/"
	}
	return "/" + filepath.ToSlash(filepath.Join(dir, slug)) + "/"
}

func frontmatterValue(content string, key string) string {
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}

	rest := strings.TrimPrefix(content, "---\n")
	before, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return ""
	}

	for line := range strings.SplitSeq(before, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}

		trimmed := strings.TrimSpace(line)
		prefix := key + ":"
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}

		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		return strings.Trim(value, `"'`)
	}

	return ""
}

func nearestNonEmptyAboveIs(lines []string, idx int, want string) bool {
	for i := idx - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return strings.TrimSpace(lines[i]) == want
	}
	return false
}

func nearestNonEmptyBelowIs(lines []string, idx int, want string) bool {
	for i := idx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return strings.TrimSpace(lines[i]) == want
	}
	return false
}

func findMermaidClose(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		switch strings.TrimSpace(lines[i]) {
		case "{{</ mermaid >}}", "{{< /mermaid >}}":
			return i
		}
	}
	return -1
}

func hasNonEmptyLineBetween(lines []string, start, end int) bool {
	for i := start; i < end; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return true
		}
	}
	return false
}
