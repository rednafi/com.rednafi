// Generate the brand badge + OG cover. Default run renders a proof sheet of
// badge candidates (sizes 16-128 on light/dark) and one cover mock per
// variant into .cache/ogimage/. `-final <variant>` renders the shipping cover
// at 4080x2142, optimizes it, and names it cover-<sha256-12>.png.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mxschmitt/playwright-go"
)

const (
	ink   = "#0a0a0a" // site dark --bg; badge tile + carve + cover bg
	paper = "#ffffff" // badge filler disc
	text  = "#fafafa" // cover title
	faint = "#a1a1a1" // cover domain kicker
)

var variants = []string{"ripple", "wake", "spiral", "particles"}

func main() {
	final := flag.String("final", "", "variant to render as the shipping cover")
	favOnly := flag.String("favicon", "", "variant to render as favicons only (leaves the cover alone)")
	flag.Parse()

	for _, v := range []string{*final, *favOnly} {
		if v != "" && !slices.Contains(variants, v) {
			log.Fatalf("unknown variant %q; valid: %s", v, strings.Join(variants, ", "))
		}
	}

	outDir := ".cache/ogimage"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, f := range []string{"geist-latin.woff2", "geist-mono-latin.woff2"} {
		raw, err := os.ReadFile(filepath.Join("static/fonts", f))
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outDir, f), raw, 0o644); err != nil {
			log.Fatal(err)
		}
	}

	pw, err := playwright.Run()
	if err != nil {
		log.Fatal(err)
	}
	defer pw.Stop()
	browser, err := pw.Chromium.Launch()
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close()

	if *favOnly != "" {
		writeFavicons(browser, outDir, *favOnly)
		return
	}
	if *final != "" {
		renderFinal(browser, outDir, *final)
		return
	}

	shoot(browser, outDir, "sheet.html", sheetHTML(), 1180, 980, 2, "sheet.png")
	for _, v := range variants {
		shoot(browser, outDir, "cover-"+v+".html", coverHTML(v), 1200, 630, 2, "cover-"+v+".png")
	}
	fmt.Println("wrote", outDir+"/sheet.png", "and cover mocks")
}

func renderFinal(browser playwright.Browser, outDir, variant string) {
	// 1200x630 layout at dsf 3.4 = 4080x2142, the dimensions head.html emits.
	// Favicons are separate; regenerate them with -favicon.
	shoot(browser, outDir, "cover-final.html", coverHTML(variant), 1200, 630, 3.4, "cover-final.png")

	full := filepath.Join(outDir, "cover-final.png")
	if err := run("oxipng", "-o", "max", "--strip", "safe", full); err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		log.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	hashed := filepath.Join(outDir, "cover-"+hash[:12]+".png")
	if err := os.WriteFile(hashed, raw, 0o644); err != nil {
		log.Fatal(err)
	}
	writeCrops(full, outDir)
	shoot(browser, outDir, "cover-proof.html", proofHTML("cover-final.png"), 1360, 1420, 2, "cover-proof.png")

	badge := badgeSVG(variant, 512, ink, paper)
	if err := os.WriteFile(filepath.Join(outDir, "badge-"+variant+".svg"), []byte(badge), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("cover:", hashed)
	fmt.Println("sha256:", hash)
}

// faviconSVG is the favicon-specific mark, planetscale-style: a full-bleed
// black rounded-square tile with the white mark disc inside. The square fills
// the canvas so the tab icon matches github/planetscale optical size; the
// white disc carries contrast on light AND dark tab bars (no scheme flip).
// Disc = 62.5% of the tile — planetscale's measured white/tile ratio — so the
// black guard around it reads as thick as theirs.
func faviconSVG(variant string) string {
	carve := `<g fill="` + ink + `">
    <circle cx="204.8" cy="288.8" r="42.4"/>
    <circle cx="288.8" cy="252.8" r="25.6"/>
    <circle cx="334.4" cy="210.4" r="16"/>
  </g>`
	if variant == "wake" {
		// reflection strips clipped 10px inside the disc so its bottom rim
		// stays closed (a strip touching the edge melts the disc into the tile)
		carve = `<defs><clipPath id="w"><circle cx="256" cy="256" r="150"/></clipPath></defs>
  <g clip-path="url(#w)" fill="` + ink + `">
    <rect x="0" y="291" width="512" height="30"/>
    <rect x="0" y="345" width="512" height="24"/>
    <rect x="0" y="393" width="512" height="18"/>
  </g>`
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512" role="img" aria-labelledby="t">
  <title id="t">rednafi.com</title>
  <rect width="512" height="512" rx="120" fill="` + ink + `"/>
  <circle cx="256" cy="256" r="160" fill="` + paper + `"/>
  ` + carve + `
</svg>`
}

// writeFavicons overwrites static/favicon.{svg,png} with the favicon mark.
// The PNG renders the light-scheme (ink) variant on transparency.
func writeFavicons(browser playwright.Browser, outDir, variant string) {
	svg := faviconSVG(variant)
	if err := os.WriteFile("static/favicon.svg", []byte(svg+"\n"), 0o644); err != nil {
		log.Fatal(err)
	}

	html := `<!doctype html><meta charset="utf-8">
<style>* { margin: 0; } body { width: 512px; height: 512px; background: transparent; }</style>
<body>` + svg + `</body>`
	htmlPath := filepath.Join(outDir, "favicon.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		log.Fatal(err)
	}
	abs, err := filepath.Abs(htmlPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport:          &playwright.Size{Width: 512, Height: 512},
		DeviceScaleFactor: playwright.Float(2),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := page.Goto("file://" + abs); err != nil {
		log.Fatal(err)
	}
	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path:           new("static/favicon.png"),
		OmitBackground: new(true),
	}); err != nil {
		log.Fatal(err)
	}
	if err := run("oxipng", "-o", "max", "--strip", "safe", "static/favicon.png"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("favicon: static/favicon.svg static/favicon.png")
}

// writeCrops emits pixel-exact center crops at every aspect ratio social
// cards use. Ratios wider than the 1.91:1 source (2:1) trim height; narrower
// ones (16:9 down to 1:1) trim width.
func writeCrops(coverPath, outDir string) {
	f, err := os.Open(coverPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	b := img.Bounds()
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		log.Fatal("cover image does not support cropping")
	}
	ratios := map[string]float64{
		"crop-2x1.png":  2,        // x/twitter large card
		"crop-16x9.png": 16.0 / 9, // google discover, embeds
		"crop-3x2.png":  3.0 / 2,  //
		"crop-4x3.png":  4.0 / 3,  //
		"crop-1x1.png":  1,        // whatsapp/imessage thumbs
	}
	for name, ratio := range ratios {
		w, h := b.Dx(), int(float64(b.Dx())/ratio)
		if h > b.Dy() {
			w, h = int(float64(b.Dy())*ratio), b.Dy()
		}
		x0 := b.Min.X + (b.Dx()-w)/2
		y0 := b.Min.Y + (b.Dy()-h)/2
		out, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(out, sub.SubImage(image.Rect(x0, y0, x0+w, y0+h))); err != nil {
			log.Fatal(err)
		}
		if err := out.Close(); err != nil {
			log.Fatal(err)
		}
	}
}

// proofHTML frames the final cover the way platforms render it: center-crop
// (object-fit: cover) at each card ratio, at feed size and thumbnail size,
// so sharpness and crop safety are checked in one screenshot.
func proofHTML(coverName string) string {
	type frame struct {
		label string
		w, h  int
	}
	cards := []frame{
		{"1.91:1 og · facebook/linkedin/slack", 640, 335},
		{"2:1 x large card", 640, 320},
		{"16:9 discover/discord", 640, 360},
		{"3:2", 540, 360},
		{"4:3", 440, 330},
		{"1:1 whatsapp/imessage", 335, 335},
	}
	thumbs := []frame{
		{"1.91:1 · 200w", 200, 105},
		{"2:1 · 200w", 200, 100},
		{"16:9 · 200w", 200, 113},
		{"4:3 · 160w", 160, 120},
		{"1:1 · 120w", 120, 120},
		{"1.91:1 · 96w timeline", 96, 50},
	}
	var b strings.Builder
	b.WriteString(`<!doctype html>
<meta charset="utf-8">
<style>
  * { margin: 0; box-sizing: border-box; }
  body { background: #e5e5e5; font: 12px/1.4 monospace; color: #333; padding: 20px; }
  .row { display: flex; flex-wrap: wrap; gap: 20px; align-items: flex-start; margin-bottom: 20px; }
  figure { display: flex; flex-direction: column; gap: 4px; }
  img { display: block; object-fit: cover; object-position: center; border-radius: 8px; }
</style>
<body>`)
	for _, group := range [][]frame{cards, thumbs} {
		b.WriteString(`<div class="row">`)
		for _, fr := range group {
			fmt.Fprintf(&b, `<figure><img src=%q width="%d" height="%d"><figcaption>%s</figcaption></figure>`,
				coverName, fr.w, fr.h, fr.label)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</body>`)
	return b.String()
}

func shoot(browser playwright.Browser, outDir, htmlName, html string, w, h int, dsf float64, outName string) {
	htmlPath := filepath.Join(outDir, htmlName)
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		log.Fatal(err)
	}
	abs, err := filepath.Abs(htmlPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport:          &playwright.Size{Width: w, Height: h},
		DeviceScaleFactor: new(dsf),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := page.Goto("file://" + abs); err != nil {
		log.Fatal(err)
	}
	if _, err := page.Evaluate(`async () => { await document.fonts.ready; }`); err != nil {
		log.Fatal(err)
	}
	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: new(filepath.Join(outDir, outName)),
	}); err != nil {
		log.Fatal(err)
	}
}

// badgeSVG draws the mark: a tile circle and carve in fg, filler disc in bg.
// Brand normal is fg=ink bg=white; the favicon inverts them. All geometry is
// primitives in a 512 viewBox; size is the rendered width. The favicon mark
// itself lives in faviconSVG, not here.
func badgeSVG(variant string, size int, fg, bg string) string {
	var carve string
	switch variant {
	case "ripple":
		// rings spreading from a low pole, clipped by the disc: a drop hitting
		// still water.
		carve = `<g clip-path="url(#disc)" fill="none" stroke="` + fg + `" stroke-width="26">
      <circle cx="256" cy="322" r="28" fill="` + fg + `" stroke="none"/>
      <circle cx="256" cy="322" r="88"/>
      <circle cx="256" cy="322" r="148"/>
      <circle cx="256" cy="322" r="208"/>
    </g>`
	case "wake":
		// solid disc above a waterline, sliced into fading strips below it: the
		// disc's own reflection.
		var rects strings.Builder
		y, gap := 298.0, 13.0
		for y < 460 {
			rects.WriteString(fmt.Sprintf(`<rect x="0" y="%.0f" width="512" height="%.0f"/>`, y, gap))
			strip := 27.0 - (y-298.0)*0.13
			if strip < 6 {
				strip = 6
			}
			y += gap + strip
			gap *= 1.28
		}
		carve = `<g clip-path="url(#disc)" fill="` + fg + `">` + rects.String() + `</g>`
	case "spiral":
		carve = `<path d="` + spiralPath(300, 256, 256) + `" fill="none" stroke="` + fg +
			`" stroke-width="34" stroke-linecap="round"/>`
	case "particles":
		// three phi-scaled particles cascading through the disc.
		carve = `<g fill="` + fg + `">
      <circle cx="190" cy="298" r="54"/>
      <circle cx="298" cy="252" r="33"/>
      <circle cx="356" cy="198" r="20"/>
    </g>`
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 512 512">
  <defs><clipPath id="disc"><circle cx="256" cy="256" r="204"/></clipPath></defs>
  <circle cx="256" cy="256" r="256" fill="%s"/>
  <circle cx="256" cy="256" r="204" fill="%s"/>
  %s
</svg>`, size, size, fg, bg, carve)
}

// spiralPath returns a golden (logarithmic) spiral polyline, bbox-centered at
// (cx, cy) and scaled so its longest side is fit pixels.
func spiralPath(fit, cx, cy float64) string {
	const b = 0.30635 // ln(phi) / (pi/2): golden growth per quarter turn
	const turns = 2.25
	n := 480
	xs, ys := make([]float64, n+1), make([]float64, n+1)
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for i := 0; i <= n; i++ {
		th := turns * 2 * math.Pi * float64(i) / float64(n)
		r := math.Exp(b * th)
		xs[i], ys[i] = r*math.Cos(th), -r*math.Sin(th)
		minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
		minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
	}
	scale := fit / math.Max(maxX-minX, maxY-minY)
	offX := cx - (minX+maxX)/2*scale
	offY := cy - (minY+maxY)/2*scale
	var d strings.Builder
	for i := 0; i <= n; i++ {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&d, "%s%.1f %.1f", cmd, xs[i]*scale+offX, ys[i]*scale+offY)
	}
	return d.String()
}

// gridSVG draws the cover background: a monochrome 80s wireframe floor,
// faded near the horizon. No glow — tried, rejected as a smudge.
func gridSVG() string {
	const w, h, cx, hy = 1200.0, 630.0, 600.0, 470.0
	var b strings.Builder

	// floor: rows compress quadratically toward the horizon at hy, verticals
	// fan out from the vanishing point to evenly spaced feet on the bottom edge
	var lines strings.Builder
	const rows, footGap = 9, 96.0
	for i := 1; i <= rows; i++ {
		t := float64(i) / float64(rows)
		y := hy + (h-hy)*t*t
		fmt.Fprintf(&lines, `<line x1="0" y1="%.1f" x2="%.0f" y2="%.1f"/>`, y, w, y)
	}
	for k := -14; k <= 14; k++ {
		fmt.Fprintf(&lines, `<line x1="%.0f" y1="%.1f" x2="%.1f" y2="%.0f"/>`, cx, hy, cx+float64(k)*footGap, h)
	}

	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="fadeDown" x1="0" y1="470" x2="0" y2="630" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#fff" stop-opacity="0"/>
      <stop offset="0.45" stop-color="#fff" stop-opacity="0.55"/>
      <stop offset="1" stop-color="#fff" stop-opacity="1"/>
    </linearGradient>
    <mask id="mDown"><rect width="1200" height="630" fill="url(#fadeDown)"/></mask>
  </defs>
  <g mask="url(#mDown)" stroke="` + text + `" stroke-width="1.5" opacity="0.32">` + lines.String() + `</g>
</svg>`)
	return b.String()
}

func coverHTML(variant string) string {
	return `<!doctype html>
<meta charset="utf-8">
<style>
  @font-face { font-family: Geist; src: url("geist-latin.woff2") format("woff2"); font-weight: 100 900; }
  @font-face { font-family: "Geist Mono"; src: url("geist-mono-latin.woff2") format("woff2"); font-weight: 100 900; }
  * { margin: 0; }
  body {
    width: 1200px; height: 630px; background: ` + ink + `;
    position: relative;
    display: grid; place-items: center;
    font-family: Geist, sans-serif; text-rendering: optimizeLegibility;
  }
  .grid { position: absolute; inset: 0; }
  .lockup { position: relative; display: flex; flex-direction: column; align-items: center; }
  h1 {
    /* 500 on ink ≈ the site's 450 display weight (white-on-dark reads heavier) */
    margin-top: 40px; color: ` + text + `; font-size: 84px; font-weight: 500;
    letter-spacing: -0.045em; line-height: 1.04; text-align: center;
  }
  .domain {
    margin-top: 30px; font-family: "Geist Mono", monospace; font-size: 24px;
    color: ` + faint + `; letter-spacing: 0.04em;
  }
</style>
<body>
  <div class="grid">` + gridSVG() + `</div>
  <div class="lockup">
    ` + badgeSVG(variant, 144, ink, paper) + `
    <h1>Redowan&rsquo;s<br>Reflections</h1>
    <div class="domain">rednafi.com</div>
  </div>
</body>`
}

// sheetHTML lays every variant out at 128/64/32/16 on light and dark tiles.
func sheetHTML() string {
	var rows strings.Builder
	for _, v := range variants {
		rows.WriteString(`<div class="row"><div class="label">` + v + `</div>`)
		for _, bg := range []string{"#ffffff", ink} {
			rows.WriteString(`<div class="tile" style="background:` + bg + `">`)
			for _, size := range []int{128, 64, 32, 16} {
				rows.WriteString(badgeSVG(v, size, ink, paper))
			}
			rows.WriteString(`</div>`)
		}
		rows.WriteString(`</div>`)
	}
	return `<!doctype html>
<meta charset="utf-8">
<style>
  @font-face { font-family: "Geist Mono"; src: url("geist-mono-latin.woff2") format("woff2"); font-weight: 100 900; }
  * { margin: 0; box-sizing: border-box; }
  body { background: #888; padding: 24px; font-family: "Geist Mono", monospace; }
  .row { display: flex; align-items: center; gap: 16px; margin-bottom: 24px; }
  .label { width: 100px; font-size: 14px; }
  .tile { display: flex; align-items: center; gap: 20px; padding: 20px; border-radius: 8px; }
  svg { display: block; }
</style>
<body>` + rows.String() + `</body>`
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
