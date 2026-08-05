package site_test

import (
	"fmt"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdversarialHomepageResponsiveMatrix(t *testing.T) {
	viewports := []playwright.Size{
		{Width: 240, Height: 320},
		{Width: 280, Height: 653},
		{Width: 320, Height: 568},
		{Width: 360, Height: 640},
		{Width: 375, Height: 667},
		{Width: 390, Height: 844},
		{Width: 412, Height: 915},
		{Width: 430, Height: 932},
		{Width: 480, Height: 800},
		{Width: 568, Height: 320},
		{Width: 600, Height: 960},
		{Width: 640, Height: 960},
		{Width: 641, Height: 960},
		{Width: 667, Height: 375},
		{Width: 711, Height: 1265},
		{Width: 768, Height: 1024},
		{Width: 820, Height: 1180},
		{Width: 844, Height: 390},
		{Width: 906, Height: 1265},
		{Width: 912, Height: 1368},
		{Width: 960, Height: 1024},
		{Width: 961, Height: 900},
		{Width: 978, Height: 1265},
		{Width: 1024, Height: 600},
		{Width: 1100, Height: 800},
		{Width: 1280, Height: 720},
		{Width: 1366, Height: 768},
		{Width: 1440, Height: 900},
		{Width: 1536, Height: 864},
		{Width: 1920, Height: 1080},
		{Width: 2560, Height: 1440},
		{Width: 3840, Height: 2160},
		{Width: 7680, Height: 4320},
	}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/")

			problems, err := page.Evaluate(`() => {
				const problems = [];
				const rect = selector => document.querySelector(selector)?.getBoundingClientRect();
				const element = selector => document.querySelector(selector);
				const tolerance = 1;
				const withinViewport = (name, box) => {
					if (!box) return problems.push(name + " is missing");
					if (box.left < -tolerance || box.right > innerWidth + tolerance)
						problems.push(name + " escapes horizontally: " + JSON.stringify(box.toJSON()));
				};

				if (document.documentElement.scrollWidth > innerWidth + tolerance)
					problems.push("document has horizontal overflow: " + document.documentElement.scrollWidth + " > " + innerWidth);

				[
					["header", rect(".site-header")],
					["hero", rect(".hero")],
					["utility", rect(".hero__utility")],
					["body", rect(".hero__body")],
					["image stage", rect(".hero__art-stage")],
					["footer", rect(".hero__footer")],
					["recent writing", rect(".recent-writing")]
				].forEach(([name, box]) => withinViewport(name, box));

				const grid = element(".hero__grid");
				const body = rect(".hero__body");
				const stage = rect(".hero__art-stage");
				const artElement = element(".hero__art");
				const art = artElement.getBoundingClientRect();
				const footer = rect(".hero__footer");
				const actions = rect(".hero__actions");
				const image = element(".hero__image");
				if (!image.complete || !image.naturalWidth || !image.naturalHeight)
					problems.push("hero image did not load");
				const imageRect = image.getBoundingClientRect();
				for (const edge of ["left", "right", "top", "bottom"])
					if (Math.abs(imageRect[edge] - stage[edge]) > tolerance)
						problems.push("image misses stage " + edge + " edge");
				const imageStyle = getComputedStyle(image);
				if (imageStyle.objectFit !== "cover") problems.push("hero image must use object-fit cover");
				if (imageStyle.objectPosition !== "50% 100%") problems.push("hero crop anchor drifted: " + imageStyle.objectPosition);

				if (innerWidth <= 960) {
					const actionBox = actions;
					const topGap = stage.top - actionBox.bottom;
					const bottomGap = footer.top - stage.bottom;
					const expectedGap = parseFloat(getComputedStyle(element(".hero__actions")).marginBottom);
					if (Math.abs(topGap - bottomGap) > tolerance)
						problems.push("stacked image gaps are asymmetric: " + topGap + " / " + bottomGap);
					if (Math.abs(topGap - expectedGap) > tolerance)
						problems.push("top image gap ignores art margin: " + topGap + " / " + expectedGap);
					const expectedRatio = innerWidth <= 640 ? 0.9 : 1.6;
					if (Math.abs(stage.width / stage.height - expectedRatio) > 0.015)
						problems.push("stacked image ratio drifted: " + stage.width / stage.height);
				} else {
					if (Math.abs(stage.top - art.top) > tolerance || Math.abs(stage.bottom - art.bottom) > tolerance)
						problems.push("desktop image does not fill its art row");
					for (const selector of [".hero__eyebrow", ".hero__lead", ".hero__tagline", ".hero__actions"]) {
						const box = rect(selector);
						if (box.right > stage.left + tolerance)
							problems.push(selector + " overlaps the desktop image");
					}
				}

				const buttons = [...document.querySelectorAll(".hero__cta")];
				const buttonRects = buttons.map(button => button.getBoundingClientRect());
				buttons.forEach((button, index) => {
					const box = buttonRects[index];
					if (box.left < actions.left - tolerance || box.right > actions.right + tolerance)
						problems.push("CTA " + index + " escapes action row");
					if (button.scrollWidth > button.clientWidth + tolerance)
						problems.push("CTA " + index + " content overflows");
					if (box.height < 44 - tolerance) problems.push("CTA " + index + " is shorter than 44px");
				});
				if (innerWidth > 300 && Math.abs(buttonRects[0].top - buttonRects[1].top) > tolerance)
					problems.push("CTAs do not share a row");
				if (buttonRects[0].right > buttonRects[1].left + tolerance && Math.abs(buttonRects[0].top - buttonRects[1].top) <= tolerance)
					problems.push("CTAs overlap");

				const footerRect = rect(".hero__footer");
				for (const link of document.querySelectorAll(".hero__footer a")) {
					const box = link.getBoundingClientRect();
					if (box.left < footerRect.left - tolerance || box.right > footerRect.right + tolerance)
						problems.push("footer control escapes footer: " + JSON.stringify(box.toJSON()) + " / " + JSON.stringify(footerRect.toJSON()));
				}

				const intro = rect(".home-intro");
				const recent = rect(".recent-writing");
				const introMargin = parseFloat(getComputedStyle(element(".home-intro")).marginBottom);
				if (Math.abs((recent.top - intro.bottom) - introMargin) > tolerance)
					problems.push("post-hero spacing drifted: " + (recent.top - intro.bottom) + " / " + introMargin);

				return problems;
			}`)
			require.NoError(t, err)
			assert.Empty(t, problems, "%dx%d responsive failures: %v", viewport.Width, viewport.Height, problems)
		})
	}
}

func TestAdversarialArticleMetadataResponsiveMatrix(t *testing.T) {
	viewports := []playwright.Size{
		{Width: 240, Height: 320},
		{Width: 280, Height: 653},
		{Width: 320, Height: 568},
		{Width: 390, Height: 844},
		{Width: 568, Height: 320},
		{Width: 640, Height: 960},
		{Width: 641, Height: 960},
		{Width: 768, Height: 1024},
		{Width: 906, Height: 1265},
		{Width: 960, Height: 1024},
		{Width: 961, Height: 900},
		{Width: 1440, Height: 900},
	}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/go/request-coalescing/")

			problems, err := page.Evaluate(`() => {
				const problems = [];
				const tolerance = 1;
				const metadata = document.querySelector(".post-meta");
				const author = document.querySelector(".post-meta-author");
				const details = document.querySelector(".post-meta-details");
				const time = details.querySelector("time");
				const reading = details.querySelector(".post-meta-reading");
				const metaBox = metadata.getBoundingClientRect();
				const containmentBox = metadata.getBoundingClientRect();
				const authorBox = author.getBoundingClientRect();
				const detailsBox = details.getBoundingClientRect();
				const style = getComputedStyle(metadata);

				if (document.documentElement.scrollWidth > innerWidth + tolerance)
					problems.push("document overflows horizontally");
				for (const [name, element] of [["author", author], ["details", details], ["date", time], ["read time", reading]]) {
					const box = element.getBoundingClientRect();
					if (box.left < containmentBox.left - tolerance || box.right > containmentBox.right + tolerance)
						problems.push(name + " escapes article metadata region");
				}
				if (parseFloat(style.fontSize) !== 14 || parseFloat(style.lineHeight) !== 20)
					problems.push("metadata type is not Vercel's 14/20 baseline");
				if (!style.fontFamily.toLowerCase().includes("geist") || style.fontFamily.toLowerCase().includes("mono"))
					problems.push("metadata is not Geist Sans");
				if (style.textTransform !== "none" || style.letterSpacing !== "normal")
					problems.push("metadata has forced caps or tracking");
				if (innerWidth <= 640 && detailsBox.top < authorBox.bottom - tolerance)
					problems.push("phone metadata did not split into two rows");
				if (innerWidth <= 300) {
					const timeBox = time.getBoundingClientRect();
					const readingBox = reading.getBoundingClientRect();
					if (readingBox.top < timeBox.bottom - tolerance)
						problems.push("ultra-narrow date/read time did not stack");
				}
				return problems;
			}`)
			require.NoError(t, err)
			assert.Empty(t, problems, "%dx%d metadata failures: %v", viewport.Width, viewport.Height, problems)
		})
	}
}

func TestAdversarialArticleShellResponsiveMatrix(t *testing.T) {
	viewports := []playwright.Size{
		{Width: 240, Height: 320},
		{Width: 280, Height: 653},
		{Width: 320, Height: 568},
		{Width: 390, Height: 844},
		{Width: 568, Height: 320},
		{Width: 640, Height: 960},
		{Width: 641, Height: 960},
		{Width: 768, Height: 1024},
		{Width: 844, Height: 390},
		{Width: 906, Height: 1265},
		{Width: 960, Height: 1024},
		{Width: 961, Height: 900},
		{Width: 1024, Height: 600},
		{Width: 1280, Height: 720},
		{Width: 1440, Height: 900},
		{Width: 1920, Height: 1080},
		{Width: 7680, Height: 4320},
	}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/go/request-coalescing/")
			_, err = page.Locator("details.toc").Evaluate(`el => { el.open = true }`, nil)
			require.NoError(t, err)

			problems, err := page.Evaluate(`() => {
				const problems = [];
				const tolerance = 1;
				const el = selector => document.querySelector(selector);
				const box = selector => el(selector)?.getBoundingClientRect();
				const within = (name, child, parent) => {
					if (!child || !parent) return problems.push(name + " is missing");
					if (child.left < parent.left - tolerance || child.right > parent.right + tolerance)
						problems.push(name + " escapes horizontally");
				};
				if (document.documentElement.scrollWidth > innerWidth + tolerance)
					problems.push("document has horizontal overflow");

				const viewportBox = {left: 0, right: innerWidth};
				const siteHeader = box(".site-header");
				const title = box(".site-title");
				const actions = box(".header-actions");
				within("site header", siteHeader, viewportBox);
				within("site title", title, siteHeader);
				within("header actions", actions, siteHeader);
				if (title.right > actions.left + tolerance) problems.push("site title overlaps header actions");

				const outer = box(".article-body");
				const articleHeader = box(".article-body > header");
				const content = box(".article-content");
				const breadcrumbs = box(".breadcrumbs");
				const toc = box("details.toc");
				const summary = box("details.toc > summary");
				const tocNav = box("details.toc > nav");
				const mobile = innerWidth <= 960;
				within("article shell", outer, viewportBox);
				within("article header", articleHeader, outer);
				within("article content", content, outer);
				within("breadcrumbs", breadcrumbs, articleHeader);
				within("TOC", toc, articleHeader);
				within("TOC summary", summary, toc);
				within("TOC panel", tocNav, toc);

				const crumbStyle = getComputedStyle(el(".breadcrumbs"));
				if (parseFloat(crumbStyle.fontSize) !== 14 || parseFloat(crumbStyle.lineHeight) !== 20)
					problems.push("breadcrumbs are not Vercel's 14/20 baseline");
				if (crumbStyle.fontFamily.toLowerCase().includes("mono") || crumbStyle.textTransform !== "none")
					problems.push("breadcrumbs use the wrong type treatment");
				const summaryStyle = getComputedStyle(el("details.toc > summary"));
				if (parseFloat(summaryStyle.fontSize) !== 14 || parseFloat(summaryStyle.lineHeight) !== 20)
					problems.push("TOC summary is not Vercel's 14/20 baseline");
				if (summary.height < 40 - tolerance) problems.push("TOC target is shorter than 40px");
				if (tocNav.scrollHeight > tocNav.clientHeight && getComputedStyle(el("details.toc > nav")).overflowY !== "auto")
					problems.push("long TOC cannot scroll internally");

				const expectedOuterWidth = Math.min(innerWidth - (mobile ? 40 : 48), 720);
				if (Math.abs(outer.width - expectedOuterWidth) > tolerance)
					problems.push("article shell width drifted: " + outer.width + " / " + expectedOuterWidth);
				const expectedContentWidth = expectedOuterWidth;
				if (Math.abs(content.width - expectedContentWidth) > tolerance)
					problems.push("reading width drifted: " + content.width + " / " + expectedContentWidth);
				return problems;
			}`)
			require.NoError(t, err)
			assert.Empty(t, problems, "%dx%d article-shell failures: %v", viewport.Width, viewport.Height, problems)
		})
	}
}

func TestAdversarialContinuousResponsiveSweep(t *testing.T) {
	widthSet := map[int]bool{}
	for width := 240; width <= 1600; width += 13 {
		widthSet[width] = true
	}
	for _, width := range []int{240, 300, 320, 340, 360, 390, 639, 640, 641, 959, 960, 961, 1023, 1024, 1099, 1100, 1101, 1279, 1280, 1281, 1439, 1440, 1441, 1600} {
		widthSet[width] = true
	}
	widths := make([]int, 0, len(widthSet))
	for width := 240; width <= 1600; width++ {
		if widthSet[width] {
			widths = append(widths, width)
		}
	}

	for _, target := range []struct {
		name string
		path string
	}{
		{name: "home", path: "/"},
		{name: "article", path: "/go/request-coalescing/"},
	} {
		t.Run(target.name, func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
				Viewport: &playwright.Size{Width: 240, Height: 900},
			})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, target.path)

			for _, width := range widths {
				require.NoError(t, page.SetViewportSize(width, 900))
				problems, err := page.Evaluate(`target => {
					const problems = [];
					const tolerance = 1;
					const box = selector => document.querySelector(selector)?.getBoundingClientRect();
					const insideViewport = (name, rect) => {
						if (!rect) return problems.push(name + " is missing");
						if (rect.left < -tolerance || rect.right > innerWidth + tolerance)
							problems.push(name + " escapes viewport");
					};
					if (document.documentElement.scrollWidth > innerWidth + tolerance)
						problems.push("document overflow: " + document.documentElement.scrollWidth + " > " + innerWidth);
					const title = box(".site-title");
					const actions = box(".header-actions");
					insideViewport("site title", title);
					insideViewport("header actions", actions);
					if (title.right > actions.left + tolerance) problems.push("header controls overlap the site title");

					if (target === "home") {
						for (const selector of [".hero", ".hero__body", ".hero__actions", ".hero__art-stage", ".hero__footer", ".recent-writing"])
							insideViewport(selector, box(selector));
						const stage = box(".hero__art-stage");
						if (innerWidth > 960) {
							for (const selector of [".hero__eyebrow", ".hero__lead", ".hero__tagline", ".hero__actions"])
								if (box(selector).right > stage.left + tolerance) problems.push(selector + " overlaps image");
						}
					} else {
						for (const selector of [".article-body", ".article-body > header", ".breadcrumbs", ".post-meta", ".toc", ".article-content"])
							insideViewport(selector, box(selector));
						const content = box(".article-content");
						for (const block of document.querySelectorAll(".article-content :is(.codeblock, pre, table, .mermaid, img, video, iframe)")) {
							const rect = block.getBoundingClientRect();
							if (rect.left < content.left - tolerance || rect.right > content.right + tolerance)
								problems.push(block.tagName.toLowerCase() + " escapes reading column");
						}
						const mobile = innerWidth <= 960;
						const type = selector => getComputedStyle(document.querySelector(selector));
						if (parseFloat(type(".article-body h1").fontSize) !== (mobile ? 40 : 48)) problems.push("h1 breakpoint drift");
						if (Math.abs(parseFloat(type(".article-content > p").fontSize) - (mobile ? 16.64 : 18)) > 0.05) problems.push("body breakpoint drift");
						if (Math.abs(parseFloat(type(".article-content pre code").fontSize) - (mobile ? 13.4 : 14.4)) > 0.05) problems.push("fenced-code breakpoint drift");
					}
					return problems;
				}`, target.name)
				require.NoError(t, err)
				assert.Empty(t, problems, "%s responsive failures at %dpx: %v", target.name, width, problems)
			}
		})
	}
}

func TestAdversarialDPRAndThemeReflow(t *testing.T) {
	cases := []struct {
		width, height int
		dpr           float64
		dark          bool
	}{
		{width: 390, height: 844, dpr: 1, dark: false},
		{width: 390, height: 844, dpr: 2, dark: true},
		{width: 390, height: 844, dpr: 3, dark: false},
		{width: 844, height: 390, dpr: 3, dark: true},
		{width: 960, height: 540, dpr: 2, dark: true},
		{width: 961, height: 540, dpr: 2, dark: false},
		{width: 1440, height: 900, dpr: 2, dark: true},
	}

	for _, testCase := range cases {
		for _, path := range []string{"/", "/go/request-coalescing/"} {
			name := fmt.Sprintf("%s-%dx%d-dpr%g-dark%t", path, testCase.width, testCase.height, testCase.dpr, testCase.dark)
			t.Run(name, func(t *testing.T) {
				colorScheme := playwright.ColorSchemeLight
				if testCase.dark {
					colorScheme = playwright.ColorSchemeDark
				}
				ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
					Viewport:          &playwright.Size{Width: testCase.width, Height: testCase.height},
					DeviceScaleFactor: new(testCase.dpr),
					ColorScheme:       colorScheme,
				})
				require.NoError(t, err)
				t.Cleanup(func() { ctx.Close() })
				page, err := ctx.NewPage()
				require.NoError(t, err)
				goto_(t, page, path)

				problems, err := page.Evaluate(`expectedDark => {
					const problems = [];
					if (document.documentElement.scrollWidth > innerWidth + 1)
						problems.push("horizontal overflow");
					if (devicePixelRatio < 1) problems.push("invalid DPR");
					const expectedTheme = expectedDark ? "dark" : "light";
					if (document.documentElement.dataset.theme !== expectedTheme)
						problems.push("theme mismatch: " + document.documentElement.dataset.theme);
					for (const selector of [".site-header", "main", "footer.site-footer"]) {
						const box = document.querySelector(selector)?.getBoundingClientRect();
						if (!box || box.left < -1 || box.right > innerWidth + 1)
							problems.push(selector + " escapes viewport");
					}
					return problems;
				}`, testCase.dark)
				require.NoError(t, err)
				assert.Empty(t, problems, "%s failures: %v", name, problems)
			})
		}
	}
}

func TestAdversarialCommandPaletteViewportMatrix(t *testing.T) {
	viewports := []playwright.Size{
		{Width: 240, Height: 320},
		{Width: 280, Height: 653},
		{Width: 320, Height: 568},
		{Width: 390, Height: 844},
		{Width: 568, Height: 320},
		{Width: 711, Height: 1265},
		{Width: 844, Height: 390},
		{Width: 906, Height: 1265},
		{Width: 978, Height: 1265},
		{Width: 1024, Height: 600},
		{Width: 1440, Height: 900},
		{Width: 3840, Height: 2160},
	}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dx%d", viewport.Width, viewport.Height), func(t *testing.T) {
			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{Viewport: &viewport})
			require.NoError(t, err)
			t.Cleanup(func() { ctx.Close() })
			page, err := ctx.NewPage()
			require.NoError(t, err)
			goto_(t, page, "/")
			require.NoError(t, page.Locator("[data-command-open]").Click())

			problems, err := page.Evaluate(`() => {
				const problems = [];
				const dialog = document.querySelector("[data-command-palette]");
				const panel = document.querySelector(".command-palette__dialog").getBoundingClientRect();
				const body = document.querySelector(".command-palette__body");
				if (!dialog.open || !dialog.matches(":modal")) problems.push("palette is not a native modal");
				if (panel.left < -1 || panel.right > innerWidth + 1 || panel.top < -1 || panel.bottom > innerHeight + 1)
					problems.push("palette escapes viewport: " + JSON.stringify(panel.toJSON()));
				if (document.documentElement.scrollWidth > innerWidth + 1) problems.push("palette causes horizontal overflow");
				if (body.clientHeight > body.scrollHeight) problems.push("palette body has invalid scroll geometry");
				if (document.querySelectorAll("[data-command-source='quick-link']").length !== 7)
					problems.push("quick links are incomplete");
				if (document.activeElement !== document.querySelector("[data-command-input]"))
					problems.push("search input did not receive focus");
				return problems;
			}`)
			require.NoError(t, err)
			assert.Empty(t, problems, "%dx%d palette failures: %v", viewport.Width, viewport.Height, problems)
		})
	}
}
