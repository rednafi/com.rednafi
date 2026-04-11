package site_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hugo date format from config: "January 2, 2006"
var dateFormatRe = regexp.MustCompile(`^(January|February|March|April|May|June|July|August|September|October|November|December) \d{1,2}, \d{4}$`)
var isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// TestDateFormatConsistency verifies all visible dates on the homepage use the configured format.
func TestDateFormatConsistency(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/")

	times := page.Locator("time")
	count, err := times.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0)

	for i := range min(count, 20) {
		t.Run("time element", func(t *testing.T) {
			// Visible text should match "Month Day, Year"
			text, err := times.Nth(i).TextContent()
			require.NoError(t, err)
			assert.Regexp(t, dateFormatRe, text, "time[%d] visible text %q doesn't match format", i, text)

			// datetime attribute should be ISO 8601
			dt, err := times.Nth(i).GetAttribute("datetime")
			require.NoError(t, err)
			assert.Regexp(t, isoDateRe, dt, "time[%d] datetime %q not ISO format", i, dt)
		})
	}
}

// TestArticleDateFormat verifies the single article page date format.
func TestArticleDateFormat(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	meta := page.Locator(".post-meta time")
	text, err := meta.TextContent()
	require.NoError(t, err)
	assert.Regexp(t, dateFormatRe, text)

	dt, err := meta.GetAttribute("datetime")
	require.NoError(t, err)
	assert.Regexp(t, isoDateRe, dt)
}

// TestArchiveDateFormat verifies archive page dates.
func TestArchiveDateFormat(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/archive/")

	times := page.Locator(".post time")
	count, err := times.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0)

	for i := range min(count, 10) {
		text, err := times.Nth(i).TextContent()
		require.NoError(t, err)
		assert.Regexp(t, dateFormatRe, text, "archive time[%d] format mismatch", i)
	}
}
