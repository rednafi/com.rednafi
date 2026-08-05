package site_test

import (
	"fmt"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVercelArticleTypographyAndRhythmParity(t *testing.T) {
	for _, viewport := range []playwright.Size{
		{Width: 390, Height: 844},
		{Width: 768, Height: 1024},
		{Width: 960, Height: 1024},
		{Width: 961, Height: 900},
		{Width: 1280, Height: 900},
		{Width: 1440, Height: 900},
	} {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			defer ctx.Close()
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/go/rate-limiting-via-nginx/")

			problems, err := page.Evaluate(`() => {
				const problems = [];
				const mobile = innerWidth <= 960;
				const close = (actual, expected, label, tolerance = 0.11) => {
					if (Math.abs(parseFloat(actual) - expected) > tolerance)
						problems.push(label + ": " + actual + " != " + expected + "px");
				};
				const style = selector => getComputedStyle(document.querySelector(selector));
				const checkType = (selector, size, line, label) => {
					const s = style(selector);
					close(s.fontSize, size, label + " font-size");
					close(s.lineHeight, line, label + " line-height");
				};

				close(style("html").fontSize, 16, "root font-size");
				checkType(".site-title", 16, 32, "site title");
				checkType(".command-trigger", 14, 20, "search trigger");
				checkType(".article-body h1", mobile ? 40 : 48, mobile ? 48 : 56, "article h1");
				checkType(".post-meta", 14, 20, "article metadata");
				// Sole prose exception: mobile body copy is 1.04x Vercel's 16px size;
				// line-height remains Vercel's 24px.
				checkType(".article-content > p", mobile ? 16.64 : 18, mobile ? 24 : 28, "article body");
				checkType(".article-content h2", 32, 35.2, "article h2");
				checkType(".article-content h3", mobile ? 24 : 28, mobile ? 26.4 : 30.8, "article h3");
				checkType(".article-content h4", 16, 17.6, "article h4");

				const h1 = style(".article-body h1");
				close(h1.letterSpacing, mobile ? -2.4 : -2.88, "h1 tracking");
				if (h1.fontWeight !== "450") problems.push("h1 weight: " + h1.fontWeight + " != 450");
				const h2 = style(".article-content h2");
				close(h2.paddingTop, 48, "h2 lead padding");
				close(h2.marginBottom, 24, "h2 trailing rhythm");
				const h3 = style(".article-content h3");
				close(h3.paddingTop, 40, "h3 lead padding");
				close(h3.marginBottom, 24, "h3 trailing rhythm");
				const h4 = style(".article-content h4");
				close(h4.letterSpacing, 1.6, "h4 tracking");
				if (h4.fontWeight !== "450") problems.push("h4 weight: " + h4.fontWeight + " != 450");
				if (!h4.fontFamily.toLowerCase().includes("geist mono")) problems.push("h4 is not Geist Mono");

				checkType(".article-content pre", 16, 24, "code frame");
				// Sole typography exception retained from the reference commit.
				checkType(".article-content pre code", mobile ? 13.5 : 14.3, mobile ? 20 : 21, "fenced code");
				checkType(".article-content p > code", 14, 20, "inline code");
				const pre = style(".article-content pre");
				close(pre.paddingTop, 20, "code top padding");
				close(pre.paddingRight, 0, "code right frame padding");
				close(pre.paddingBottom, 20, "code bottom padding");
				close(pre.paddingLeft, 0, "code left frame padding");
				const line = style(".article-content .highlight .line");
				close(line.paddingLeft, 20, "code row left padding");
				close(line.paddingRight, 20, "code row right padding");
				const inline = style(".article-content p > code");
				close(inline.paddingTop, 4, "inline-code vertical padding");
				close(inline.paddingRight, 5, "inline-code horizontal padding");

				const body = document.querySelector(".article-content").getBoundingClientRect();
				const expectedWidth = Math.min(innerWidth - (mobile ? 40 : 48), 720);
				close(body.width, expectedWidth, "reading column width", 0.6);
				if (Math.abs(body.left - (innerWidth - body.right)) > 0.6)
					problems.push("reading column is not horizontally centered");
				if (mobile) close(style("main").paddingLeft, 20, "mobile rail");
				const block = document.querySelector(".codeblock").getBoundingClientRect();
				const preBox = document.querySelector(".codeblock pre").getBoundingClientRect();
				close(preBox.left - block.left, 2, "code frame inner left inset", 0.6);
				close(block.right - preBox.right, 2, "code frame inner right inset", 0.6);

				return problems;
			}`)
			require.NoError(t, err)
			assert.Empty(t, problems, "Vercel parity drift at %dx%d: %v", viewport.Width, viewport.Height, problems)
		})
	}
}
