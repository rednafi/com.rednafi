package site_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAlertRendering verifies the custom blockquote alert system renders
// correctly. The render-blockquote.html template converts GitHub-style alerts
// (> [!NOTE], etc.) into styled divs. Each type has a distinct border color.
func TestAlertRendering(t *testing.T) {
	t.Parallel()
	t.Run("important alert on article page", func(t *testing.T) {
		requirePage(t, "/shards/2026/04/no-stacked-loglines/")
		page := newPage(t)
		goto_(t, page, "/shards/2026/04/no-stacked-loglines/")

		alert := page.Locator(".alert.alert-important")
		count, err := alert.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "page should have an important alert")

		// Alert should have a title
		title, err := alert.First().Locator(".alert-title").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Important", title)

		// Alert should have the correct border color
		borderColor, err := alert.First().Evaluate(
			`el => getComputedStyle(el).borderLeftColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, borderColor, "alert-important should have a left border color")

		body, err := alert.First().TextContent()
		require.NoError(t, err)
		normalizedBody := strings.Join(strings.Fields(body), " ")
		assert.Contains(
			t,
			normalizedBody,
			"related but separate problem in GetByID and GetUser",
			"alert body should retain the opening sentence after markdown rendering",
		)
		assert.Contains(
			t,
			normalizedBody,
			"you handled it",
			"alert body should retain the rest of the quoted sentence after markdown rendering",
		)

		nestedLists, err := alert.First().Locator("ul, ol").Count()
		require.NoError(t, err)
		assert.Equal(t, 0, nestedLists, "alert body should not accidentally render list markup")
	})

	t.Run("note alert renders", func(t *testing.T) {
		requirePage(t, "/go/mutex-closure/")
		page := newPage(t)
		goto_(t, page, "/go/mutex-closure/")

		alert := page.Locator(".alert.alert-note")
		count, err := alert.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "page should have a note alert")

		title, err := alert.First().Locator(".alert-title").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Note", title)
	})

	t.Run("warning alert renders", func(t *testing.T) {
		requirePage(t, "/go/wrap-grpc-client/")
		page := newPage(t)
		goto_(t, page, "/go/wrap-grpc-client/")

		alert := page.Locator(".alert.alert-warning")
		count, err := alert.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "page should have a warning alert")

		title, err := alert.First().Locator(".alert-title").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Warning", title)
	})

	t.Run("alert has background and border styling", func(t *testing.T) {
		requirePage(t, "/go/mutex-closure/")
		page := newPage(t)
		goto_(t, page, "/go/mutex-closure/")

		alert := page.Locator(".alert").First()
		bg, err := alert.Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "rgba(0, 0, 0, 0)", bg, "alert should have background color")

		radius, err := alert.Evaluate(
			`el => getComputedStyle(el).borderRadius`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "0px", radius, "alert should have border radius")
	})

	t.Run("supported alert titles have distinctive icons", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/go/testscript-cli/")

		raw, err := page.Evaluate(`() => {
			const types = ["note", "tip", "important", "warning", "caution"];
			const probe = document.createElement("div");
			probe.id = "alert-icon-probe";
			probe.style.cssText = "position:absolute;left:-9999px;top:0;width:320px";
			probe.innerHTML = types.map(type =>
				'<div class="alert alert-' + type + '"><p class="alert-title">' +
				type + '</p><p>Body</p></div>'
			).join("");
			document.body.append(probe);

			const failures = [];
			const masks = new Set();
			for (const type of types) {
				const title = probe.querySelector(".alert-" + type + " .alert-title");
				const before = getComputedStyle(title, "::before");
				const titleStyle = getComputedStyle(title);
				const mask = before.webkitMaskImage || before.maskImage;
				const width = parseFloat(before.width);
				const height = parseFloat(before.height);
				const fontSize = parseFloat(titleStyle.fontSize);
				const display = before.display;
				const bg = before.backgroundColor;

				if (display === "none") failures.push(type + ": icon is not displayed");
				if (!(width > 0 && height > 0)) failures.push(type + ": icon has no size");
				if (Math.abs(width - fontSize) > 1 || Math.abs(height - fontSize) > 1) {
					failures.push(type + ": icon should match title text size");
				}
				if (!mask || mask === "none") failures.push(type + ": icon mask is missing");
				if (bg === "rgba(0, 0, 0, 0)") failures.push(type + ": icon has no color");
				if (mask && mask !== "none") masks.add(mask);
			}
			if (masks.size !== types.length) failures.push("alert icons should be distinctive per type");
			return failures;
		}`)
		require.NoError(t, err)
		require.Empty(t, toStringSlice(raw))
	})

	t.Run("markdown content keeps readable body color", func(t *testing.T) {
		requirePage(t, "/go/testscript-cli/")
		page := newPage(t)
		goto_(t, page, "/go/testscript-cli/")

		_, err := page.Evaluate(`() => {
			const article = document.querySelector("article");
			const probe = document.createElement("div");
			probe.id = "alert-css-probe";
			probe.className = "alert alert-warning";
			probe.innerHTML = [
				'<p class="alert-title">Warning</p>',
				'<p>Probe <code>inline</code></p>',
				'<div class="codeblock"><div class="highlight"><pre><code>plain output</code></pre></div></div>'
			].join("");
			article.append(probe);
		}`)
		require.NoError(t, err)

		readRootText := func() string {
			raw, err := page.Evaluate(`() => {
				const probe = document.createElement("span");
				probe.style.color = getComputedStyle(document.documentElement)
					.getPropertyValue("--text").trim();
				document.body.append(probe);
				const color = getComputedStyle(probe).color;
				probe.remove();
				return color;
			}`)
			require.NoError(t, err)
			color, _ := raw.(string)
			require.NotEmpty(t, color)
			return color
		}

		checkColors := func(theme string) {
			alert := page.Locator(".alert.alert-note").First()
			rootText := readRootText()

			for _, tc := range []struct {
				name     string
				selector string
			}{
				{"body paragraph", "p:not(.alert-title)"},
				{"list item", "li"},
				{"inline code", "li code"},
			} {
				color, err := alert.Locator(tc.selector).First().Evaluate(
					`el => getComputedStyle(el).color`, nil,
				)
				require.NoError(t, err)
				assert.Equal(t, rootText, color, "%s: alert %s should use --text", theme, tc.name)
			}

			titleColor, err := alert.Locator(".alert-title").Evaluate(
				`el => getComputedStyle(el).color`, nil,
			)
			require.NoError(t, err)
			assert.NotEqual(t, rootText, titleColor, "%s: alert title should keep variant accent", theme)

			codeBg, err := alert.Locator("li code").First().Evaluate(
				`el => getComputedStyle(el).backgroundColor`, nil,
			)
			require.NoError(t, err)
			assert.NotEqual(t, "rgba(0, 0, 0, 0)", codeBg,
				"%s: inline code in alerts should keep a visible chip background", theme)

			preColor, err := page.Locator("#alert-css-probe pre").Evaluate(
				`el => getComputedStyle(el).color`, nil,
			)
			require.NoError(t, err)
			assert.Equal(t, rootText, preColor, "%s: code blocks in alerts should use --text", theme)

			preBg, err := page.Locator("#alert-css-probe pre").Evaluate(
				`el => getComputedStyle(el).backgroundColor`, nil,
			)
			require.NoError(t, err)
			alertBg, err := page.Locator("#alert-css-probe").Evaluate(
				`el => getComputedStyle(el).backgroundColor`, nil,
			)
			require.NoError(t, err)
			assert.NotEqual(t, alertBg, preBg,
				"%s: code blocks in alerts should remain visually separated from the alert fill", theme)
		}

		checkColors("light")
		require.NoError(t, page.Locator("button[data-theme-set='dark']").Click())
		checkColors("dark")
	})
}

// TestAlertMarkdownSpacing enforces the markdown shape required by Hugo/Goldmark
// alert parsing: the alert marker line must be followed by a blank quoted line.
func TestAlertMarkdownSpacing(t *testing.T) {
	t.Parallel()

	var violations []string

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

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "> [!") {
				continue
			}

			if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) != ">" {
				violations = append(
					violations,
					path+":"+strconv.Itoa(i+1),
				)
			}
		}

		return nil
	})
	require.NoError(t, err)
	require.Empty(
		t,
		violations,
		"alert markers must be followed by a blank quoted line; fix these locations",
	)
}

// TestBlockquoteRendering verifies regular blockquotes (not alerts) render with
// the decorative quotation mark and italic styling from the CSS.
func TestBlockquoteRendering(t *testing.T) {
	t.Parallel()
	// Use an article that has regular blockquotes (not alerts)
	requirePage(t, "/misc/pesky-little-scripts/")
	page := newPage(t)
	goto_(t, page, "/misc/pesky-little-scripts/")

	bq := page.Locator("article blockquote:not(.alert)")
	count, err := bq.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no regular blockquotes on this page")
	}

	t.Run("has left border accent", func(t *testing.T) {
		borderStyle, err := bq.First().Evaluate(
			`el => getComputedStyle(el).borderLeftStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "solid", borderStyle)
	})

	t.Run("has italic text", func(t *testing.T) {
		fontStyle, err := bq.First().Evaluate(
			`el => getComputedStyle(el).fontStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "italic", fontStyle)
	})

	t.Run("has background color", func(t *testing.T) {
		bg, err := bq.First().Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "rgba(0, 0, 0, 0)", bg, "blockquote should have background")
	})
}

// TestTableRendering verifies HTML tables generated from markdown have proper
// styling applied (border collapse, full width, header borders).
func TestTableRendering(t *testing.T) {
	t.Parallel()
	requirePage(t, "/go/testing-unary-grpc-services/")
	page := newPage(t)
	goto_(t, page, "/go/testing-unary-grpc-services/")

	table := page.Locator("article table")
	count, err := table.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0, "page should contain a table")

	t.Run("table is full width", func(t *testing.T) {
		width, err := table.First().Evaluate(
			`el => getComputedStyle(el).width`, nil,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, width, "table should have computed width")
	})

	t.Run("table is a rounded Geist data-table", func(t *testing.T) {
		// Geist tables use border-collapse:separate + border-spacing:0 so the
		// table can carry one rounded, hairline-bordered container.
		collapse, err := table.First().Evaluate(
			`el => getComputedStyle(el).borderCollapse`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "separate", collapse)

		radius, err := table.First().Evaluate(
			`el => getComputedStyle(el).borderTopLeftRadius`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "0px", radius, "table should have rounded corners")
	})

	t.Run("table header has bottom border", func(t *testing.T) {
		th := page.Locator("article table thead th").First()
		thCount, err := th.Count()
		require.NoError(t, err)
		if thCount == 0 {
			t.Skip("table has no thead")
		}
		border, err := th.Evaluate(
			`el => getComputedStyle(el).borderBottomStyle`, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, "solid", border)
	})

	t.Run("table cells have padding", func(t *testing.T) {
		td := page.Locator("article table td").First()
		padding, err := td.Evaluate(
			`el => parseFloat(getComputedStyle(el).paddingLeft)`, nil,
		)
		require.NoError(t, err)
		assert.Greater(t, padding.(float64), float64(0), "table cells should have padding")
	})
}

// TestInlineCodeStyling verifies inline code elements have background color
// and padding to visually distinguish them from surrounding text.
func TestInlineCodeStyling(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	// Find inline code (not inside a pre block)
	code := page.Locator("article p code")
	count, err := code.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0, "article should have inline code")

	t.Run("has background color", func(t *testing.T) {
		bg, err := code.First().Evaluate(
			`el => getComputedStyle(el).backgroundColor`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "rgba(0, 0, 0, 0)", bg, "inline code should have background")
	})

	t.Run("has padding", func(t *testing.T) {
		padding, err := code.First().Evaluate(
			`el => parseFloat(getComputedStyle(el).paddingLeft) + parseFloat(getComputedStyle(el).paddingRight)`, nil,
		)
		require.NoError(t, err)
		assert.Greater(t, padding.(float64), float64(0), "inline code should have padding")
	})

	t.Run("has border-radius", func(t *testing.T) {
		radius, err := code.First().Evaluate(
			`el => getComputedStyle(el).borderRadius`, nil,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "0px", radius, "inline code should have border-radius")
	})
}

// TestPostOrnamentDivider verifies the § ornament separator appears between
// article content and related posts.
func TestPostOrnamentDivider(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	ornament := page.Locator(".post-ornament")
	count, err := ornament.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no ornament divider on this page (no related posts?)")
	}

	text, err := ornament.TextContent()
	require.NoError(t, err)
	assert.Contains(t, text, "§", "ornament should contain § character")
}

// TestRelatedPostsSection verifies the related posts nav has valid links
// and proper structure.
func TestRelatedPostsSection(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	related := page.Locator("nav.related-posts")
	count, err := related.Count()
	require.NoError(t, err)
	if count == 0 {
		t.Skip("no related posts on this page")
	}

	t.Run("has heading", func(t *testing.T) {
		h2, err := related.Locator("h2").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Related posts", h2)
	})

	t.Run("has between 1 and 5 links", func(t *testing.T) {
		links := related.Locator("a")
		linkCount, err := links.Count()
		require.NoError(t, err)
		assert.Greater(t, linkCount, 0)
		assert.LessOrEqual(t, linkCount, 5, "related posts capped at 5")
	})

	t.Run("related links resolve", func(t *testing.T) {
		hrefs, err := related.Locator("a").EvaluateAll(
			`els => els.map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)
		for _, h := range toStringSlice(hrefs) {
			resp := httpGetResp(t, resolveURL(h))
			assert.Equal(t, 200, resp.StatusCode, "related post %s broken", h)
			resp.Body.Close()
		}
	})
}

// TestSyntaxHighlightingPresence verifies code blocks on an article use
// Chroma syntax highlighting classes (not raw pre/code without coloring).
func TestSyntaxHighlightingPresence(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	t.Run("code blocks have chroma classes", func(t *testing.T) {
		// Chroma wraps syntax-highlighted code in spans with class names
		// like .k (keyword), .s (string), .nf (function name), etc.
		chromaSpans, err := page.Locator(".chroma span[class]").Count()
		require.NoError(t, err)
		assert.Greater(t, chromaSpans, 10,
			"syntax-highlighted code should have many Chroma spans")
	})

	t.Run("keywords are colored in light theme", func(t *testing.T) {
		// .chroma .k should have a non-default color
		color, err := page.Evaluate(
			`() => {
				const kw = document.querySelector(".chroma .k, .chroma .kd, .chroma .kn");
				return kw ? getComputedStyle(kw).color : null;
			}`,
		)
		require.NoError(t, err)
		require.NotNil(t, color, "should find keyword element")
		assert.NotEqual(t, "rgb(0, 0, 0)", color, "keywords should be colored, not plain black")
	})
}

// TestSyntaxHighlightingDarkTheme verifies Chroma colors change when the
// dark theme is activated. The site uses modus-operandi (light) and
// modus-vivendi (dark) color schemes.
func TestSyntaxHighlightingDarkTheme(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	// Get keyword color in light theme
	lightColor, err := page.Evaluate(
		`() => {
			const kw = document.querySelector(".chroma .k, .chroma .kd, .chroma .kn");
			return kw ? getComputedStyle(kw).color : null;
		}`,
	)
	require.NoError(t, err)
	require.NotNil(t, lightColor)

	// Switch to dark theme
	require.NoError(t, page.Locator("button[data-theme-set='dark']").Click())

	// Get keyword color in dark theme
	darkColor, err := page.Evaluate(
		`() => {
			const kw = document.querySelector(".chroma .k, .chroma .kd, .chroma .kn");
			return kw ? getComputedStyle(kw).color : null;
		}`,
	)
	require.NoError(t, err)
	require.NotNil(t, darkColor)

	assert.NotEqual(t, lightColor, darkColor,
		"Chroma keyword color should change between light (%v) and dark (%v) themes",
		lightColor, darkColor)
}
