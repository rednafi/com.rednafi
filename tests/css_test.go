package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCSSVariablesDefinedInLight verifies all expected CSS custom properties
// are defined in light theme.
func TestCSSVariablesDefinedInLight(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	vars := []string{
		"--bg", "--text", "--muted", "--faint", "--code-bg",
		"--border", "--link", "--visited", "--accent-quote",
		"--radius", "--content-width",
	}

	for _, v := range vars {
		t.Run(v, func(t *testing.T) {
			val, err := page.Evaluate(
				`v => getComputedStyle(document.documentElement).getPropertyValue(v).trim()`, v,
			)
			require.NoError(t, err)
			assert.NotEmpty(t, val, "CSS variable %s not defined", v)
		})
	}
}

// TestCSSVariablesDefinedInDark verifies dark theme overrides all expected variables.
func TestCSSVariablesDefinedInDark(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")
	require.NoError(t, page.Locator("button.theme-toggle").Click())

	expectedDark := map[string]string{
		"--bg":           "#212529",
		"--text":         "#e0e0e0",
		"--code-bg":      "#2c3034",
		"--border":       "#404550",
		"--link":         "#79a8ff",
		"--accent-quote": "#a892d6",
	}

	for v, expected := range expectedDark {
		t.Run(v, func(t *testing.T) {
			val, err := page.Evaluate(
				`v => getComputedStyle(document.documentElement).getPropertyValue(v).trim()`, v,
			)
			require.NoError(t, err)
			assert.Equal(t, expected, val, "dark theme %s mismatch", v)
		})
	}
}

// TestSelectionStyling verifies ::selection uses link color on bg color.
func TestSelectionStyling(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	// ::selection can't be directly queried via getComputedStyle,
	// but we can verify the CSS rule exists in the stylesheet
	hasRule, err := page.Evaluate(`() => {
		for (const sheet of document.styleSheets) {
			try {
				for (const rule of sheet.cssRules) {
					if (rule.selectorText && rule.selectorText.includes("::selection")) {
						return true;
					}
				}
			} catch(e) {}
		}
		return false;
	}`)
	require.NoError(t, err)
	assert.True(t, hasRule.(bool), "::selection CSS rule not found")
}

// TestCodeBlockStyling verifies code blocks have correct font and styling.
func TestCodeBlockStyling(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/configure-options/")

	t.Run("code uses Geist Mono font", func(t *testing.T) {
		font, err := page.Evaluate(
			`() => {
				const code = document.querySelector("pre code");
				if (!code) return "";
				return getComputedStyle(code).fontFamily;
			}`,
		)
		require.NoError(t, err)
		fontStr, _ := font.(string)
		assert.Contains(t, strings.ToLower(fontStr), "geist mono")
	})

	t.Run("pre blocks have border and border-radius", func(t *testing.T) {
		borderStyle, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector("pre")).borderStyle`,
		)
		require.NoError(t, err)
		assert.NotEqual(t, "none", borderStyle)

		radius, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector("pre")).borderRadius`,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, radius)
		assert.NotEqual(t, "0px", radius)
	})

	t.Run("code blocks have overflow-x auto", func(t *testing.T) {
		overflow, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector("pre")).overflowX`,
		)
		require.NoError(t, err)
		assert.Equal(t, "auto", overflow)
	})
}

// TestStickyFooter verifies the page uses flexbox for sticky footer.
func TestStickyFooter(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/about/")

	t.Run("page container uses flex column", func(t *testing.T) {
		display, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector(".page")).display`,
		)
		require.NoError(t, err)
		assert.Equal(t, "flex", display)

		direction, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector(".page")).flexDirection`,
		)
		require.NoError(t, err)
		assert.Equal(t, "column", direction)
	})

	t.Run("page has min-height 100vh", func(t *testing.T) {
		minHeight, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector(".page")).minHeight`,
		)
		require.NoError(t, err)
		assert.Contains(t, minHeight, "px") // 100vh resolves to px
	})

	t.Run("page-content has flex 1", func(t *testing.T) {
		flex, err := page.Evaluate(
			`() => getComputedStyle(document.querySelector(".page-content")).flexGrow`,
		)
		require.NoError(t, err)
		assert.Equal(t, "1", flex)
	})
}
