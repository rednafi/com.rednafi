package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDarkThemeVisitedLinkColor verifies that the dark theme defines --visited.
// Geist links never recolor once visited, so --visited matches the dark --link
// (Geist Blue 900) rather than a separate visited hue.
func TestDarkThemeVisitedLinkColor(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	require.NoError(t, themeButton(t, page, "dark").Click())

	visited, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--visited").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#52a8ff", visited, "dark theme visited link color")
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
	assert.Equal(t, "#f2f2f2", lightCodeBg)

	require.NoError(t, themeButton(t, page, "dark").Click())

	darkCodeBg, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--code-bg").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#1a1a1a", darkCodeBg)
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

	require.NoError(t, themeButton(t, page, "dark").Click())

	darkMuted, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--muted").trim()`,
	)
	require.NoError(t, err)

	assert.NotEqual(t, lightMuted, darkMuted,
		"muted color should change between themes")
	// Dark muted should be lighter than light muted for contrast
	assert.Equal(t, "#a1a1a1", darkMuted)
}

// TestDarkThemeBorderColor verifies borders adapt to dark theme.
func TestDarkThemeBorderColor(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	require.NoError(t, themeButton(t, page, "dark").Click())

	border, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).getPropertyValue("--border").trim()`,
	)
	require.NoError(t, err)
	assert.Equal(t, "#2e2e2e", border, "dark theme border color")
}
