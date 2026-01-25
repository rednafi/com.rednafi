//go:build ignore

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

var errRequestTimeout = errors.New("request timeout")

type result struct {
	url    string
	status int
	err    error
}

func main() {
	baseURL := flag.String("url", "http://localhost:1313", "Base URL to test")
	contentDir := flag.String("content", "content", "Content directory")
	maxWorkers := flag.Int("workers", 100, "Max concurrent requests")
	timeout := flag.Duration("timeout", 10*time.Second, "Request timeout")
	flag.Parse()

	// Handle interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("Testing URLs against: %s\n", *baseURL)
	fmt.Println(strings.Repeat("=", 50))

	urls := collectURLs(*contentDir)
	fmt.Printf("\nCollected %d URLs to test\n", len(urls))
	fmt.Println(strings.Repeat("-", 50))

	results := testURLs(ctx, *baseURL, urls, *maxWorkers, *timeout)

	var passed, failed int
	var failedURLs []result

	for _, r := range results {
		if r.status == 200 {
			fmt.Printf("%s✓%s %s\n", colorGreen, colorReset, r.url)
			passed++
		} else {
			errMsg := ""
			if r.err != nil {
				errMsg = fmt.Sprintf(", err: %v", r.err)
			}
			fmt.Printf("%s✗%s %s (status: %d%s)\n", colorRed, colorReset, r.url, r.status, errMsg)
			failed++
			failedURLs = append(failedURLs, r)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Results: %s%d passed%s, %s%d failed%s\n",
		colorGreen, passed, colorReset,
		colorRed, failed, colorReset)

	if failed > 0 {
		fmt.Println("\nFailed URLs:")
		for _, r := range failedURLs {
			fmt.Printf("  - %s (status: %d)\n", r.url, r.status)
		}
		os.Exit(1)
	}
}

func collectURLs(contentDir string) []string {
	urlSet := make(map[string]struct{})

	_ = filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if strings.HasSuffix(path, "_index.md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fm := extractFrontmatter(string(content))

		// Get aliases
		for _, alias := range fm.aliases {
			url := strings.TrimSuffix(alias, "/") + "/"
			urlSet[url] = struct{}{}
		}

		// Get canonical URL
		dir := filepath.Dir(path)
		section := filepath.Base(dir)
		if section == "content" {
			return nil
		}

		slug := fm.slug
		if slug == "" {
			slug = strings.TrimSuffix(filepath.Base(path), ".md")
		}
		if slug == "_index" || slug == "about" {
			return nil
		}

		canonical := fmt.Sprintf("/%s/%s/", section, slug)
		urlSet[canonical] = struct{}{}

		return nil
	})

	urls := make([]string, 0, len(urlSet))
	for url := range urlSet {
		urls = append(urls, url)
	}
	sort.Strings(urls)
	return urls
}

type frontmatter struct {
	slug    string
	aliases []string
}

func extractFrontmatter(content string) frontmatter {
	var fm frontmatter

	if !strings.HasPrefix(content, "---") {
		return fm
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fm
	}

	scanner := bufio.NewScanner(strings.NewReader(parts[1]))
	inAliases := false

	slugRe := regexp.MustCompile(`^slug:\s*(.+)$`)
	aliasRe := regexp.MustCompile(`^\s*-\s*(/[^\s]+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "slug:") {
			if m := slugRe.FindStringSubmatch(line); m != nil {
				fm.slug = strings.TrimSpace(m[1])
			}
			inAliases = false
			continue
		}

		if strings.HasPrefix(line, "aliases:") {
			inAliases = true
			continue
		}

		if inAliases {
			if m := aliasRe.FindStringSubmatch(line); m != nil {
				fm.aliases = append(fm.aliases, m[1])
			} else if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				inAliases = false
			}
		}
	}

	return fm
}

func testURLs(ctx context.Context, baseURL string, urls []string, maxWorkers int, timeout time.Duration) []result {
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	results := make([]result, len(urls))

	client := &http.Client{}

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				results[idx] = result{url: u, status: 0, err: ctx.Err()}
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			reqCtx, cancel := context.WithTimeoutCause(ctx, timeout, errRequestTimeout)
			defer cancel()

			fullURL := baseURL + u
			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, nil)
			if err != nil {
				results[idx] = result{url: u, status: 0, err: err}
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				cause := context.Cause(reqCtx)
				if cause != nil && errors.Is(cause, errRequestTimeout) {
					results[idx] = result{url: u, status: 0, err: errRequestTimeout}
				} else {
					results[idx] = result{url: u, status: 0, err: err}
				}
				return
			}
			defer resp.Body.Close()

			results[idx] = result{url: u, status: resp.StatusCode}
		}(i, url)
	}

	wg.Wait()
	return results
}
