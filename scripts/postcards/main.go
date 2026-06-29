package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"html"
	"io/fs"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	"gopkg.in/yaml.v3"
)

const (
	designWidth  = 1200
	designHeight = 630
	cardWidth    = 4080
	cardHeight   = 2142
	cardScale    = float64(cardWidth) / float64(designWidth)

	defaultGeist     = "static/fonts/geist-latin.woff2"
	defaultGeistMono = "static/fonts/geist-mono-latin.woff2"
	defaultContent   = "content"
	defaultOutput    = ".cache/postcards/images"
	defaultCoverOut  = ".cache/postcards"
	defaultAssetBase = "https://blob.rednafi.com"
	cardDesignID     = "geist-fluid-card-v1"
	cardEyebrow      = "rednafi.com"
)

type post struct {
	relPath     string
	frontmatter map[string]any
}

type siteConfig struct {
	Params struct {
		MainSections []string `yaml:"mainSections"`
		NotesSection string   `yaml:"notesSection"`
	} `yaml:"params"`
}

type publishConfig struct {
	sections []string
}

func main() {
	var check, coverOnly, missingR2Assets bool
	var geistFont, geistMonoFont, contentDir, outputDir, coverOutputDir, assetBaseURL string
	flag.BoolVar(&check, "check", false, "fail if post card frontmatter is out of date")
	flag.BoolVar(&coverOnly, "cover-only", false, "render only the shared cover assets")
	flag.BoolVar(&missingR2Assets, "missing-r2-assets", false, "render only cards missing from R2")
	flag.BoolVar(&missingR2Assets, "missing-release-assets", false, "deprecated alias for --missing-r2-assets")
	flag.StringVar(&geistFont, "geist-font", defaultGeist, "Geist woff2 font used for generated cards")
	flag.StringVar(&geistMonoFont, "geist-mono-font", defaultGeistMono, "Geist Mono woff2 font used for generated cards")
	flag.StringVar(&contentDir, "content", defaultContent, "content directory")
	flag.StringVar(&outputDir, "output", defaultOutput, "local output directory for generated image assets")
	flag.StringVar(&coverOutputDir, "cover-output", defaultCoverOut, "local output directory for shared cover assets")
	flag.StringVar(&assetBaseURL, "asset-base-url", defaultAssetBase, "public base URL for generated image assets")
	flag.Parse()

	if coverOnly {
		renderer, err := newCardRenderer(geistFont, geistMonoFont)
		if err != nil {
			fatalf("init renderer: %v", err)
		}
		defer renderer.Close()

		if _, err := renderer.writeCoverAssets(coverOutputDir); err != nil {
			fatalf("write cover assets: %v", err)
		}
		fmt.Println("Generated shared cover assets.")
		return
	}

	publishing, err := loadPublishConfig("config.yml")
	if err != nil {
		fatalf("load publish config: %v", err)
	}

	posts, err := loadPosts(contentDir, publishing)
	if err != nil {
		fatalf("load posts: %v", err)
	}

	var changed []string
	var renderQueue []post
	for _, p := range posts {
		key := cardKey(p)
		publicURL := strings.TrimRight(assetBaseURL, "/") + "/" + key
		if !frontmatterImagesCurrent(p.frontmatter["images"], publicURL) {
			changed = append(changed, p.relPath)
			continue
		}
		if missingR2Assets && publicObjectExists(assetBaseURL, key) {
			continue
		}
		renderQueue = append(renderQueue, p)
	}

	if check && len(changed) > 0 {
		fatalf("post card frontmatter is out of date; run go run ./scripts/frontmatter:\n  %s", strings.Join(changed, "\n  "))
	}
	if check {
		fmt.Printf("Checked post card frontmatter for %d posts.\n", len(posts))
		return
	}

	staleCards, err := clearOutput(outputDir)
	if err != nil {
		fatalf("clear output: %v", err)
	}
	changed = append(changed, staleCards...)

	renderer, err := newCardRenderer(geistFont, geistMonoFont)
	if err != nil {
		fatalf("init renderer: %v", err)
	}
	defer renderer.Close()

	coverChanged, err := renderer.writeCoverAssets(coverOutputDir)
	if err != nil {
		fatalf("write cover assets: %v", err)
	}
	if coverChanged {
		changed = append(changed, filepath.ToSlash(filepath.Join(coverOutputDir, "cover.png")))
	}

	if len(renderQueue) == 0 {
		fmt.Println("Generated post cards for 0 posts.")
		return
	}

	for _, p := range renderQueue {
		changedPost, err := processPost(p, renderer, outputDir)
		if err != nil {
			fatalf("%s: %v", p.relPath, err)
		}
		if changedPost {
			changed = append(changed, p.relPath)
		}
	}

	fmt.Printf("Generated post cards for %d posts", len(renderQueue))
	if len(changed) > 0 {
		fmt.Printf(" (%d updated)", len(changed))
	}
	fmt.Println(".")
}

type cardRenderer struct {
	coverSVG      string
	backgroundSVG string
	fontCSS       string
	pw            *playwright.Playwright
	browser       playwright.Browser
	context       playwright.BrowserContext
	page          playwright.Page
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "postcards: "+format+"\n", args...)
	os.Exit(1)
}

func newCardRenderer(geistFont, geistMonoFont string) (*cardRenderer, error) {
	cover := brandCoverSVG()
	faces, err := fontFace("Geist", geistFont)
	if err != nil {
		return nil, err
	}
	monoFace, err := fontFace("Geist Mono", geistMonoFont)
	if err != nil {
		return nil, err
	}
	faces += monoFace

	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: new(true)})
	if err != nil {
		_ = pw.Stop()
		return nil, err
	}
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport:          &playwright.Size{Width: designWidth, Height: designHeight},
		DeviceScaleFactor: new(cardScale),
	})
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, err
	}
	page, err := context.NewPage()
	if err != nil {
		_ = context.Close()
		_ = browser.Close()
		_ = pw.Stop()
		return nil, err
	}
	return &cardRenderer{
		coverSVG:      cover,
		backgroundSVG: stripCoverText(cover),
		fontCSS:       faces,
		pw:            pw,
		browser:       browser,
		context:       context,
		page:          page,
	}, nil
}

func (r *cardRenderer) Close() {
	if r.context != nil {
		_ = r.context.Close()
	}
	if r.browser != nil {
		_ = r.browser.Close()
	}
	if r.pw != nil {
		_ = r.pw.Stop()
	}
}

func fontFace(family, filePath string) (string, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(raw)
	return fmt.Sprintf(`@font-face{font-family:%q;font-style:normal;font-weight:400 700;src:url(data:font/woff2;base64,%s) format("woff2")}`, family, enc), nil
}

func loadPublishConfig(configPath string) (publishConfig, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return publishConfig{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	var config siteConfig
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return publishConfig{}, fmt.Errorf("parse %s: %w", configPath, err)
	}

	var publishing publishConfig
	for _, section := range config.Params.MainSections {
		if section != "" {
			publishing.sections = append(publishing.sections, section)
		}
	}
	if config.Params.NotesSection != "" {
		publishing.sections = append(publishing.sections, config.Params.NotesSection)
	}
	if len(publishing.sections) == 0 {
		return publishConfig{}, fmt.Errorf("%s did not define params.mainSections or params.notesSection", configPath)
	}
	return publishing, nil
}

func loadPosts(contentDir string, publishing publishConfig) ([]post, error) {
	var posts []post
	err := filepath.WalkDir(contentDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(filePath) != ".md" {
			return nil
		}

		rel, err := filepath.Rel(contentDir, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if filepath.Base(rel) == "_index.md" || !slices.Contains(publishing.sections, sectionFor(rel)) {
			return nil
		}

		raw, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		fm, _, ok := splitFrontmatter(string(raw))
		if !ok {
			return nil
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
			return fmt.Errorf("%s: parse frontmatter: %w", rel, err)
		}
		if draft, _ := data["draft"].(bool); draft {
			return nil
		}
		if stringValue(data, "title") == "" || stringValue(data, "date") == "" {
			return nil
		}

		posts = append(posts, post{
			relPath:     rel,
			frontmatter: data,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(posts, func(a, b post) int {
		return strings.Compare(a.relPath, b.relPath)
	})
	return posts, nil
}

func sectionFor(filePath string) string {
	section, _, _ := strings.Cut(filepath.ToSlash(filePath), "/")
	return section
}

func splitFrontmatter(raw string) (frontmatter, body string, ok bool) {
	const start = "---\n"
	if !strings.HasPrefix(raw, start) {
		return "", "", false
	}
	idx := strings.Index(raw[len(start):], "\n---\n")
	if idx == -1 {
		return "", "", false
	}
	fmEnd := len(start) + idx
	bodyStart := fmEnd + len("\n---\n")
	return raw[len(start):fmEnd], raw[bodyStart:], true
}

func processPost(p post, renderer *cardRenderer, outputDir string) (bool, error) {
	cardRel := cardPath(p, outputDir)

	cardBytes, err := renderer.renderCard(stringValue(p.frontmatter, "title"))
	if err != nil {
		return false, err
	}
	wrote, err := writeIfChanged(cardRel, cardBytes, 0o644)
	if err != nil {
		return false, err
	}
	return wrote, nil
}

func frontmatterImagesCurrent(value any, imageURL string) bool {
	values := stringSlice(value)
	return len(values) == 1 && values[0] == imageURL
}

func clearOutput(outputDir string) ([]string, error) {
	var removed []string
	err := filepath.WalkDir(outputDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			return nil
		}
		removed = append(removed, filepath.ToSlash(filePath))
		return os.Remove(filePath)
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return removed, nil
}

func cardPath(p post, outputDir string) string {
	return path.Join(outputDir, cardKey(p))
}

func cardKey(p post) string {
	rawPath := stringValue(p.frontmatter, "atprotoPath")
	if rawPath == "" {
		rawPath = "/" + strings.TrimSuffix(p.relPath, filepath.Ext(p.relPath))
	}
	return postCardKey(rawPath, stringValue(p.frontmatter, "title"))
}

func postCardKey(atprotoPath, title string) string {
	clean := cleanPostPath(atprotoPath)
	sum := sha256.Sum256([]byte(cardDesignID + "\n" + clean + "\n" + title))
	return clean + "/cover-" + hex.EncodeToString(sum[:])[:12] + ".png"
}

func cleanPostPath(rawPath string) string {
	clean := path.Clean("/" + rawPath)
	clean = strings.Trim(clean, "/")
	clean = strings.TrimSuffix(clean, "/index")
	if clean == "" {
		clean = "post"
	}
	return slugPath(clean)
}

func publicObjectExists(assetBaseURL, key string) bool {
	url := strings.TrimRight(assetBaseURL, "/") + "/" + key
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

var slugPartPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugPath(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		part = strings.ReplaceAll(part, "_", "-")
		part = slugPartPattern.ReplaceAllString(part, "-")
		part = strings.Trim(part, "-")
		if part == "" {
			part = "post"
		}
		parts[i] = part
	}
	return strings.Join(parts, "/")
}

var coverTextPattern = regexp.MustCompile(`(?s)\n\s*<text\b[^>]*>.*?</text>`)

func stripCoverText(svg string) string {
	return coverTextPattern.ReplaceAllString(svg, "")
}

func (r *cardRenderer) writeCoverAssets(outputDir string) (bool, error) {
	svgPath := filepath.Join(outputDir, "cover.svg")
	pngPath := filepath.Join(outputDir, "cover.png")

	svgChanged, err := writeIfChanged(svgPath, []byte(r.coverSVG), 0o644)
	if err != nil {
		return false, err
	}

	pngBytes, err := r.renderCover()
	if err != nil {
		return false, err
	}
	pngChanged, err := writeIfChanged(pngPath, pngBytes, 0o644)
	if err != nil {
		return false, err
	}

	return svgChanged || pngChanged, nil
}

func (r *cardRenderer) renderCover() ([]byte, error) {
	if err := r.page.SetContent(r.coverHTML(), playwright.PageSetContentOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
	}); err != nil {
		return nil, err
	}
	if _, err := r.page.Evaluate(`async () => {
		await document.fonts.load('600 84px Geist');
		await document.fonts.load('500 26px "Geist Mono"');
		await document.fonts.ready;
		return true;
	}`); err != nil {
		return nil, err
	}
	return r.page.Screenshot(playwright.PageScreenshotOptions{
		FullPage:       new(false),
		OmitBackground: new(false),
		Type:           playwright.ScreenshotTypePng,
	})
}

func (r *cardRenderer) coverHTML() string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><style>%s
*{margin:0;padding:0;box-sizing:border-box}
html,body{width:%dpx;height:%dpx;overflow:hidden;background:#0a0a0a}
svg{display:block;width:%dpx;height:%dpx}
</style></head><body>%s</body></html>`,
		r.fontCSS,
		designWidth,
		designHeight,
		designWidth,
		designHeight,
		r.coverSVG,
	)
}

func (r *cardRenderer) renderCard(title string) ([]byte, error) {
	if err := r.page.SetContent(r.cardHTML(title), playwright.PageSetContentOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
	}); err != nil {
		return nil, err
	}
	if _, err := r.page.Evaluate(`async () => {
		await document.fonts.load('600 84px Geist');
		await document.fonts.load('500 26px "Geist Mono"');
		await document.fonts.ready;

		const ns = 'http://www.w3.org/2000/svg';
		const title = document.getElementById('post-title');
		const raw = title.dataset.title;
		const maxWidth = 884;
		const maxLines = 3;
		let size = 84;
		const min = 46;

		const measure = (text, currentSize) => {
			const t = document.createElementNS(ns, 'text');
			t.setAttribute('x', '152');
			t.setAttribute('y', '338');
			t.setAttribute('fill', '#ededed');
			t.setAttribute('font-family', 'Geist, sans-serif');
			t.setAttribute('font-size', String(currentSize));
			t.setAttribute('font-weight', '600');
			t.setAttribute('letter-spacing', String(-2 * currentSize / 84));
			t.textContent = text;
			title.appendChild(t);
			const width = t.getComputedTextLength();
			t.remove();
			return width;
		};

		const wrap = (currentSize) => {
			const words = raw.trim().split(/\s+/);
			const lines = [];
			let line = '';
			for (const word of words) {
				const candidate = line ? line + ' ' + word : word;
				if (!line || measure(candidate, currentSize) <= maxWidth) {
					line = candidate;
					continue;
				}
				lines.push(line);
				line = word;
			}
			if (line) lines.push(line);
			return lines;
		};

		let lines = wrap(size);
		const fits = () => lines.length <= maxLines && lines.every(line => measure(line, size) <= maxWidth);
		while (size > min && !fits()) {
			size -= 2;
			lines = wrap(size);
		}

		if (lines.length > maxLines) {
			lines = lines.slice(0, maxLines);
			let last = lines[lines.length - 1];
			while (last.length > 1 && measure(last.replace(/[\s.,:;-]+$/, '') + '...', size) > maxWidth) {
				const parts = last.trim().split(/\s+/);
				if (parts.length <= 1) {
					last = last.slice(0, -1);
				} else {
					last = parts.slice(0, -1).join(' ');
				}
			}
			lines[lines.length - 1] = last.replace(/[\s.,:;-]+$/, '') + '...';
		}

		const lineGap = size * 88 / 84;
		const firstY = 338 - ((lines.length - 2) * lineGap / 2);
		for (let i = 0; i < lines.length; i++) {
			const t = document.createElementNS(ns, 'text');
			t.setAttribute('x', '152');
			t.setAttribute('y', String(firstY + i * lineGap));
			t.setAttribute('fill', '#ededed');
			t.setAttribute('font-family', 'Geist, sans-serif');
			t.setAttribute('font-size', String(size));
			t.setAttribute('font-weight', '600');
			t.setAttribute('letter-spacing', String(-2 * size / 84));
			t.textContent = lines[i];
			title.appendChild(t);
		}
		return true;
	}`); err != nil {
		return nil, err
	}
	return r.page.Screenshot(playwright.PageScreenshotOptions{
		FullPage:       new(false),
		OmitBackground: new(false),
		Type:           playwright.ScreenshotTypePng,
	})
}

func (r *cardRenderer) cardHTML(title string) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><style>%s
*{margin:0;padding:0;box-sizing:border-box}
html,body{width:%dpx;height:%dpx;overflow:hidden;background:#0a0a0a}
.card{position:relative;width:%dpx;height:%dpx;overflow:hidden;background:#0a0a0a;color:#ededed}
.card svg{position:absolute;inset:0;display:block;width:%dpx;height:%dpx}
.card .overlay{position:absolute;inset:0;display:block;width:%dpx;height:%dpx}
</style></head><body><main class="card">%s<svg class="overlay" xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630"><text x="156" y="250" fill="#ededed" fill-opacity="0.46" font-family="Geist Mono, monospace" font-size="26" font-weight="500" letter-spacing="0.5">%s</text><g id="post-title" data-title="%s"></g></svg></main></body></html>`,
		r.fontCSS,
		designWidth,
		designHeight,
		designWidth,
		designHeight,
		designWidth,
		designHeight,
		designWidth,
		designHeight,
		r.backgroundSVG,
		cardEyebrow,
		html.EscapeString(title),
	)
}

const (
	coverFrameX0 = 96.0
	coverFrameX1 = 1104.0
)

func brandCoverSVG() string {
	const (
		bg         = "#0a0a0a"
		ink        = "#ededed"
		grid       = "#1c1c1c"
		frame      = "#2b2b2b"
		crossColor = "#5c5c5c"
		eye        = "#ededed"
		eyeOpacity = 0.46
		flowScale  = 0.0024
		opacity0   = 0.15
		opacity1   = 0.31
		stroke     = 1.0
	)

	rng := rand.New(rand.NewSource(7))
	streaks := make([]string, 0, 150)
	for tries := 0; len(streaks) < 150 && tries < 90000; tries++ {
		x := randomRange(rng, coverFrameX0, coverFrameX1)
		y := randomRange(rng, 96, 528)
		density := flowDensity(x, y)
		if density <= 0 || rng.Float64() > density {
			continue
		}
		points := flowLine(
			x,
			y,
			rng.Intn(71)+70,
			rng.Intn(36)+35,
			5.0,
			flowScale,
		)
		streaks = append(streaks, fmt.Sprintf(`<path d="%s" opacity="%.3f"/>`, svgPath(points), opacity0+opacity1*density))
	}

	burst := fmt.Sprintf(
		`<g clip-path="url(#frame-clip)" fill="none" stroke="%s" stroke-width="%.1f" stroke-linecap="round" stroke-linejoin="round">%s</g>`,
		ink,
		stroke,
		strings.Join(streaks, ""),
	)
	crosses := svgCross(96, 96, crossColor) + svgCross(1104, 528, crossColor)

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630" role="img" aria-labelledby="t d">
  <title id="t">Redowan's Reflections cover image</title>
  <desc id="d">A Geist social preview for rednafi.com: a swirling curl-noise fluid on a line grid.</desc>
  <defs>
    <clipPath id="frame-clip"><rect x="96" y="96" width="1008" height="432"/></clipPath>
    <pattern id="grid" width="48" height="48" patternUnits="userSpaceOnUse">
      <path d="M48 0H0V48" fill="none" stroke="%s" stroke-width="1"/>
    </pattern>
    <linearGradient id="leftfade" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color="%s" stop-opacity="0.96"/>
      <stop offset="0.5" stop-color="%s" stop-opacity="0"/>
    </linearGradient>
  </defs>

  <rect width="1200" height="630" fill="%s"/>

  <g clip-path="url(#frame-clip)"><rect x="96" y="96" width="1008" height="432" fill="url(#grid)"/></g>
  %s
  <g clip-path="url(#frame-clip)"><rect x="96" y="96" width="1008" height="432" fill="url(#leftfade)"/></g>
  <rect x="96" y="96" width="1008" height="432" fill="none" stroke="%s" stroke-width="1.5"/>
  %s

  <text x="156" y="250" fill="%s" fill-opacity="%.2f" font-family="Geist Mono, monospace" font-size="26" font-weight="500" letter-spacing="0.5">rednafi.com</text>
  <text x="152" y="338" fill="%s" font-family="Geist, sans-serif" font-size="84" font-weight="600" letter-spacing="-2">Redowan's</text>
  <text x="152" y="426" fill="%s" font-family="Geist, sans-serif" font-size="84" font-weight="600" letter-spacing="-2">Reflections</text>
</svg>
`,
		grid,
		bg,
		bg,
		bg,
		burst,
		frame,
		crosses,
		eye,
		eyeOpacity,
		ink,
		ink,
	)
}

func svgCross(x, y int, color string) string {
	return fmt.Sprintf(`<path d="M%d %dh26M%d %dv26" fill="none" stroke="%s" stroke-width="2.5"/>`, x-13, y, x, y-13, color)
}

func randomRange(rng *rand.Rand, min, max float64) float64 {
	return min + rng.Float64()*(max-min)
}

func flowDensity(x, y float64) float64 {
	nx := (x - coverFrameX0) / (coverFrameX1 - coverFrameX0)
	ny := (y - 96) / 432.0
	edges := smooth(0.02, 0.10, ny) * smooth(0.02, 0.10, 1-ny)
	return smooth(0.26, 0.95, nx) * edges
}

func smooth(a, b, x float64) float64 {
	var t float64
	if a != b {
		t = (x - a) / (b - a)
	}
	t = math.Max(0, math.Min(1, t))
	return t * t * (3 - 2*t)
}

func flowLine(x, y float64, forward, backward int, step, scale float64) [][2]float64 {
	fwd := [][2]float64{{x, y}}
	cx, cy := x, y
	for range forward {
		dx, dy := curl(cx*scale, cy*scale, 0.6, 2)
		cx += dx * step
		cy += dy * step
		fwd = append(fwd, [2]float64{cx, cy})
	}

	cx, cy = x, y
	bwd := make([][2]float64, 0, backward)
	for range backward {
		dx, dy := curl(cx*scale, cy*scale, 0.6, 2)
		cx -= dx * step
		cy -= dy * step
		bwd = append(bwd, [2]float64{cx, cy})
	}
	slices.Reverse(bwd)
	return append(bwd, fwd...)
}

func curl(x, y, epsilon float64, octaves int) (float64, float64) {
	dydp := (fbm(x, y+epsilon, octaves) - fbm(x, y-epsilon, octaves)) / (2 * epsilon)
	dxdp := (fbm(x+epsilon, y, octaves) - fbm(x-epsilon, y, octaves)) / (2 * epsilon)
	return dydp, -dxdp
}

func fbm(x, y float64, octaves int) float64 {
	sum, amp, freq := 0.0, 0.5, 1.0
	for range octaves {
		sum += amp * valueNoise(x*freq, y*freq)
		freq *= 2
		amp *= 0.5
	}
	return sum
}

func valueNoise(x, y float64) float64 {
	xi, yi := math.Floor(x), math.Floor(y)
	xf, yf := x-xi, y-yi
	u, v := smoothstep(xf), smoothstep(yf)
	a := hashNoise(xi, yi)
	b := hashNoise(xi+1, yi)
	c := hashNoise(xi, yi+1)
	d := hashNoise(xi+1, yi+1)
	return a + (b-a)*u + (c-a)*v + (a-b-c+d)*u*v
}

func smoothstep(t float64) float64 {
	return t * t * (3 - 2*t)
}

func hashNoise(x, y float64) float64 {
	n := math.Sin(x*127.1+y*311.7) * 43758.5453
	return n - math.Floor(n)
}

func svgPath(points [][2]float64) string {
	var b strings.Builder
	b.WriteString("M")
	for _, point := range points {
		fmt.Fprintf(&b, " %.1f %.1f", point[0], point[1])
	}
	return b.String()
}

func stringValue(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprint(v)
	}
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func writeIfChanged(filePath string, data []byte, perm fs.FileMode) (bool, error) {
	if fileBytesEqual(filePath, data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(filePath, data, perm)
}

func fileBytesEqual(filePath string, data []byte) bool {
	current, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	return bytes.Equal(current, data)
}
