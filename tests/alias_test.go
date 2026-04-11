package site_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAliasRedirectPages verifies that Hugo alias pages exist and contain
// a meta refresh pointing to the correct canonical URL. Hugo generates 251
// alias pages — if the alias mechanism breaks, hundreds of inbound links
// from external sites, search engines, and bookmarks will 404.
func TestAliasRedirectPages(t *testing.T) {
	t.Parallel()
	// Sample of known aliases → their canonical targets.
	// These represent the old underscore-style URLs that were migrated to hyphenated slugs.
	aliases := map[string]string{
		"/misc/dns_record_to_share_text/":       "/misc/dns-record-to-share-text/",
		"/misc/http_requests_via_dev_tcp/":       "/misc/http-requests-via-dev-tcp/",
		"/misc/dynamic_menu_with_select_in_bash/": "/misc/dynamic-menu-with-select-in-bash/",
		"/misc/pesky_little_scripts/":            "/misc/pesky-little-scripts/",
		"/misc/terminal_text_formatting_with_tput/": "/misc/terminal-text-formatting-with-tput/",
	}

	for alias, target := range aliases {
		t.Run(alias, func(t *testing.T) {
			body := httpGet(t, baseURL+alias)
			// Hugo alias pages contain: <meta http-equiv="refresh" content="0; url=...">
			assert.Contains(t, body, "http-equiv=refresh",
				"alias %s should contain meta refresh", alias)
			assert.Contains(t, body, target,
				"alias %s should redirect to %s", alias, target)
		})
	}
}

// TestAliasTargetsReturn200 verifies the canonical targets of alias redirects
// actually exist and serve content.
func TestAliasTargetsReturn200(t *testing.T) {
	t.Parallel()
	targets := []string{
		"/misc/dns-record-to-share-text/",
		"/misc/http-requests-via-dev-tcp/",
		"/misc/dynamic-menu-with-select-in-bash/",
		"/misc/pesky-little-scripts/",
		"/misc/terminal-text-formatting-with-tput/",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+target)
			assert.Equal(t, 200, resp.StatusCode, "alias target %s should return 200", target)
			resp.Body.Close()
		})
	}
}

// TestAliasPageHasCanonicalLink verifies alias pages declare the correct
// canonical URL so search engines consolidate link equity.
func TestAliasPageHasCanonicalLink(t *testing.T) {
	t.Parallel()
	body := httpGet(t, baseURL+"/misc/dns_record_to_share_text/")
	require.Contains(t, body, "canonical")
	// The canonical href should point to the new slug
	assert.True(t,
		strings.Contains(body, `href=https://rednafi.com/misc/dns-record-to-share-text/`) ||
			strings.Contains(body, `href="https://rednafi.com/misc/dns-record-to-share-text/"`),
		"alias canonical should point to target URL",
	)
}
