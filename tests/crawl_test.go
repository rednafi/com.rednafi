package site_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCrawlAllInternalLinks spiders every internal link found on key pages
// and verifies they all return 200. This catches broken links site-wide.
func TestCrawlAllInternalLinks(t *testing.T) {
	t.Parallel()
	seedPages := []string{
		"/", "/about/", "/archive/", "/tags/", "/python/", "/go/",
		"/misc/", "/appearances/", "/blogroll/", "/maxims/",
	}

	seen := make(map[string]bool)
	var mu sync.Mutex

	page := newPage(t)

	for _, seed := range seedPages {
		goto_(t, page, seed)
		hrefs, err := page.Locator("a[href]").EvaluateAll(
			`els => els.map(e => e.getAttribute("href"))`,
		)
		require.NoError(t, err)

		for _, h := range toStringSlice(hrefs) {
			// Normalize: strip production domain, keep only local paths
			h = strings.Replace(h, "https://rednafi.com", "", 1)
			if !strings.HasPrefix(h, "/") {
				continue
			}
			// Skip anchors-only, mailto, external
			if strings.HasPrefix(h, "//") {
				continue
			}
			// Deduplicate
			mu.Lock()
			if seen[h] {
				mu.Unlock()
				continue
			}
			seen[h] = true
			mu.Unlock()
		}
	}

	t.Logf("crawling %d unique internal links", len(seen))

	// Test all discovered links in parallel with bounded concurrency
	sem := make(chan struct{}, 20)
	var wg sync.WaitGroup
	var failures []string
	var failMu sync.Mutex

	for link := range seen {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := http.Get(baseURL + link)
			if err != nil {
				failMu.Lock()
				failures = append(failures, link+" (error: "+err.Error()+")")
				failMu.Unlock()
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				failMu.Lock()
				failures = append(failures, link+" (status: "+http.StatusText(resp.StatusCode)+")")
				failMu.Unlock()
			}
		})
	}
	wg.Wait()

	assert.Empty(t, failures, "broken internal links:\n%s", strings.Join(failures, "\n"))
}

// TestAllArchiveLinksResolve verifies every post link on the archive page returns 200.
func TestAllArchiveLinksResolve(t *testing.T) {
	t.Parallel()
	page := newPage(t)
	goto_(t, page, "/archive/")

	hrefs, err := page.Locator(".archive-month .post a").EvaluateAll(
		`els => els.map(e => e.getAttribute("href"))`,
	)
	require.NoError(t, err)
	hrefList := toStringSlice(hrefs)
	t.Logf("checking %d archive links", len(hrefList))
	assert.Greater(t, len(hrefList), 50, "archive should have many posts")

	sem := make(chan struct{}, 20)
	var wg sync.WaitGroup
	var failures []string
	var mu sync.Mutex

	for _, h := range hrefList {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := http.Get(resolveURL(h))
			if err != nil {
				mu.Lock()
				failures = append(failures, h+" (error)")
				mu.Unlock()
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				mu.Lock()
				failures = append(failures, h+" ("+http.StatusText(resp.StatusCode)+")")
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	assert.Empty(t, failures, "broken archive links:\n%s", strings.Join(failures, "\n"))
}

// TestPaginationLinksResolve verifies all pagination pages return 200.
func TestPaginationLinksResolve(t *testing.T) {
	t.Parallel()
	for i := range 5 {
		i++ // 1-based
		url := "/"
		if i > 1 {
			url = fmt.Sprintf("/page/%d/", i)
		}
		t.Run(url, func(t *testing.T) {
			resp := httpGetResp(t, baseURL+url)
			assert.Equal(t, 200, resp.StatusCode, "%s returned %d", url, resp.StatusCode)
			resp.Body.Close()
		})
	}
}
