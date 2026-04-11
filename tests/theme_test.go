package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDarkThemeVisitedLinkColor verifies that visited links use the dark theme
// color palette, not the light theme purple.
func TestDarkThemeVisitedLinkColor(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	require.NoError(t, page.Locator("button.theme-toggle").Click())

	visited, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--visited").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#b4a7d6", visited, "dark theme visited link color")
}

// TestDarkThemeBlockquoteAccent verifies the blockquote accent color changes
// in dark mode.
func TestDarkThemeBlockquoteAccent(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	lightAccent, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--accent-quote").trim()`,
	)
	require.NoError(t, err)

	require.NoError(t, page.Locator("button.theme-toggle").Click())

	darkAccent, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--accent-quote").trim()`,
	)
	require.NoError(t, err)

	assert.NotEqual(t, lightAccent, darkAccent,
		"blockquote accent should change between themes")
	assert.Equal(t, "#a892d6", darkAccent)
}

// TestDarkThemeInlineCodeBg verifies inline code background changes in dark mode.
// We check the CSS variable since the transition animation means getComputedStyle
// on the element itself may return an interpolated mid-transition value.
func TestDarkThemeInlineCodeBg(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	lightCodeBg, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--code-bg").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#f2f1eb", lightCodeBg)

	require.NoError(t, page.Locator("button.theme-toggle").Click())

	darkCodeBg, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--code-bg").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#2c3034", darkCodeBg)
}

// TestThemeTransitionsAreSmooth verifies CSS transitions are applied during
// theme toggle (not instant color changes). The CSS specifies:
// html { transition: background-color .2s ease, color .2s ease; }
func TestThemeTransitionsAreSmooth(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	transition, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).transition`,
	)
	require.NoError(t, err)
	transStr, _ := transition.(string)

	assert.Contains(t, transStr, "background-color",
		"html should have background-color transition")
	assert.Contains(t, transStr, "color",
		"html should have color transition")
}

// TestDarkThemeMutedTextColor verifies secondary text (muted, faint)
// colors adapt to dark theme for readability.
func TestDarkThemeMutedTextColor(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	lightMuted, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--muted").trim()`,
	)
	require.NoError(t, err)

	require.NoError(t, page.Locator("button.theme-toggle").Click())

	darkMuted, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--muted").trim()`,
	)
	require.NoError(t, err)

	assert.NotEqual(t, lightMuted, darkMuted,
		"muted color should change between themes")
	// Dark muted should be lighter than light muted for contrast
	assert.Equal(t, "#aaa", darkMuted)
}

// TestDarkThemeBorderColor verifies borders adapt to dark theme.
func TestDarkThemeBorderColor(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	require.NoError(t, page.Locator("button.theme-toggle").Click())

	border, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--border").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#404550", border, "dark theme border color")
}
