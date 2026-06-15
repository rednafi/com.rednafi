package site_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relLuminance returns the WCAG relative luminance of a #rgb or #rrggbb color.
func relLuminance(hex string) float64 {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	channel := func(s string) float64 {
		n, _ := strconv.ParseInt(s, 16, 0)
		c := float64(n) / 255.0
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(hex[0:2]) + 0.7152*channel(hex[2:4]) + 0.0722*channel(hex[4:6])
}

// contrastRatio returns the WCAG 2.x contrast ratio between two colors.
func contrastRatio(a, b string) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestColorContrastAA pins the site's text colours to WCAG AA on their actual
// background in BOTH themes. This is a stated hard requirement (the owner cares
// about readability), so it must be enforced, not assumed — a future palette
// tweak that drops --link/--muted/--faint below 4.5:1 will fail here.
func TestColorContrastAA(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	read := func(name string) string {
		v, err := page.Evaluate(
			`n => getComputedStyle(document.documentElement).getPropertyValue(n).trim()`, name,
		)
		require.NoError(t, err)
		s, _ := v.(string)
		return s
	}

	check := func(theme string) {
		bg := read("--bg")
		for _, c := range []struct {
			name string
			min  float64
		}{
			{"--text", 7.0},  // body/titles — comfortably AAA
			{"--link", 4.5},  // AA normal text
			{"--muted", 4.5}, // secondary text
			{"--faint", 4.5}, // tertiary text (dates, eyebrows, footer)
		} {
			fg := read(c.name)
			ratio := contrastRatio(fg, bg)
			assert.GreaterOrEqualf(t, ratio, c.min,
				"%s theme: %s (%s) on --bg (%s) is %.2f:1, below %.1f:1",
				theme, c.name, fg, bg, ratio, c.min)
		}
	}

	check("light")
	require.NoError(t, page.Locator("button[data-theme-set='dark']").Click())
	check("dark")
}

// TestFocusRingIsOpaque verifies the Geist focus ring on controls is a fully
// opaque accent ring (var(--ring)), not a faint translucent glow — a visible
// keyboard-focus indicator (WCAG 2.4.11). The theme toggle is borderless, so its
// focus box-shadow is the ring alone (no drop shadow to muddy the assertion).
func TestFocusRingIsOpaque(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	toggle := page.Locator("button[data-theme-set='dark']")
	require.NoError(t, toggle.Focus())

	bs, err := toggle.Evaluate(`el => getComputedStyle(el).boxShadow`, nil)
	require.NoError(t, err)
	s, _ := bs.(string)

	assert.NotEqual(t, "none", s, "focused control should show a box-shadow focus ring")
	assert.Contains(t, s, "rgb(", "focus ring should paint an opaque colour")
	assert.NotContains(t, s, "rgba(",
		"focus ring should be fully opaque, not a translucent color-mix glow")
}

// TestCodeBlockPaddingBalanced verifies code blocks use the generous, balanced
// Geist padding (~16px) rather than a cramped vertical gutter.
func TestCodeBlockPaddingBalanced(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	pad, err := page.Evaluate(`() => {
		const pre = document.querySelector("article pre");
		return pre ? getComputedStyle(pre).paddingTop : "";
	}`)
	require.NoError(t, err)
	s, _ := pad.(string)
	require.NotEmpty(t, s, "article should have a code block")
	top, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, top, 14.0,
		"code block vertical padding should be generous (Geist ~16px)")
}
