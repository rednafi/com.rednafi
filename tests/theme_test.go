package site_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
