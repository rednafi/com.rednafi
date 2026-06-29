package site_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const standardSitePublicationURI = "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.publication/3mnl6f7ob462z"
const atprotoDID = "did:plc:fgtm2c26vfcj74rfmeggbyqj"
const postCardsAssetBaseURL = "https://blob.rednafi.com"

type standardSiteConfig struct {
	Params struct {
		MainSections []string `yaml:"mainSections"`
		NotesSection string   `yaml:"notesSection"`
	} `yaml:"params"`
}

type standardSiteSections struct {
	sections     []string
	notesSection string
}

func TestStandardSiteWellKnownEndpoint(t *testing.T) {
	t.Parallel()

	body := httpGet(t, baseURL+"/.well-known/site.standard.publication")
	assert.Equal(t, standardSitePublicationURI, body)
}

func TestATProtoDidWellKnownEndpoint(t *testing.T) {
	t.Parallel()

	body := httpGet(t, baseURL+"/.well-known/atproto-did")
	assert.Equal(t, atprotoDID, body)
}

func TestGitHubPagesServesWellKnownEndpoint(t *testing.T) {
	t.Parallel()

	_, err := os.Stat("../static/.nojekyll")
	require.NoError(t, err, "GitHub Pages needs .nojekyll to serve .well-known files")

	workflow, err := os.ReadFile("../.github/workflows/ci.yml")
	require.NoError(t, err)
	assert.Contains(
		t,
		string(workflow),
		"include-hidden-files: true",
		"upload-pages-artifact excludes .well-known unless hidden files are included",
	)
}

func TestStandardSitePublicationLinkTags(t *testing.T) {
	t.Parallel()

	pages := []string{"/", "/go/", "/tags/", "/tags/go/", "/go/anemic-stack-traces/", "/shards/2026/04/dynamo/"}
	for _, url := range pages {
		t.Run(url, func(t *testing.T) {
			page := newPage(t)
			goto_(t, page, url)

			href, err := page.Locator(`link[rel="site.standard.publication"]`).GetAttribute("href")
			require.NoError(t, err)
			assert.Equal(t, standardSitePublicationURI, href)
		})
	}
}

func TestStandardSiteUsesMetadataOnly(t *testing.T) {
	t.Parallel()

	t.Run("homepage exposes publication metadata without visible controls", func(t *testing.T) {
		page := newPage(t)
		goto_(t, page, "/")

		publication, err := page.Locator(`link[rel="site.standard.publication"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Equal(t, standardSitePublicationURI, publication)
		assert.Equal(t, 0, mustCount(t, page.Locator("sequoia-subscribe")))
		assert.Equal(t, 0, mustCount(t, page.Locator("sequoia-recommend")))
		assert.Equal(t, 0, mustCount(t, page.Locator(`script[type="module"][src*="sequoia-subscribe"]`)))
	})

	t.Run("article exposes document metadata without visible controls", func(t *testing.T) {
		contentPath := "../content/go/anemic_stack_traces.md"
		atURI := frontmatterScalar(t, contentPath, "atUri")
		require.NotEmpty(t, atURI)

		page := newPage(t)
		goto_(t, page, "/go/anemic-stack-traces/")

		publication, err := page.Locator(`link[rel="site.standard.publication"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Equal(t, standardSitePublicationURI, publication)
		document, err := page.Locator(`link[rel="site.standard.document"]`).GetAttribute("href")
		require.NoError(t, err)
		assert.Equal(t, atURI, document)
		assert.Equal(t, 0, mustCount(t, page.Locator("sequoia-subscribe")))
		assert.Equal(t, 0, mustCount(t, page.Locator("sequoia-recommend")))
		assert.Equal(t, 0, mustCount(t, page.Locator(`script[type="module"][src*="sequoia-subscribe"]`)))
	})

	t.Run("list pages expose publication metadata without visible controls", func(t *testing.T) {
		for _, url := range []string{"/go/", "/tags/go/"} {
			t.Run(url, func(t *testing.T) {
				page := newPage(t)
				goto_(t, page, url)

				publication, err := page.Locator(`link[rel="site.standard.publication"]`).GetAttribute("href")
				require.NoError(t, err)
				assert.Equal(t, standardSitePublicationURI, publication)
				assert.Equal(t, 0, mustCount(t, page.Locator("sequoia-subscribe")))
				assert.Equal(t, 0, mustCount(t, page.Locator("sequoia-recommend")))
				assert.Equal(t, 0, mustCount(t, page.Locator(`script[type="module"][src*="sequoia-subscribe"]`)))
			})
		}
	})
}

func TestRSSAutodiscoveryOnArticlePages(t *testing.T) {
	t.Parallel()

	page := newPage(t)
	goto_(t, page, "/go/anemic-stack-traces/")

	rss := page.Locator(`link[rel="alternate"][type="application/rss+xml"][href="https://rednafi.com/index.xml"]`)
	assert.Equal(t, 1, mustCount(t, rss))
}

func TestSequoiaConfigMaximizesDiscovery(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../sequoia.json")
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(raw, &config))

	assert.Equal(t, "https://tangled.org/stevedylan.dev/sequoia/raw/main/sequoia.schema.json", config["$schema"])
	assert.Equal(t, "rednafi.com", config["identity"])
	assert.Equal(t, true, config["autoSync"])
	assert.Equal(t, true, config["publishContent"])

	bluesky, ok := config["bluesky"].(map[string]any)
	require.True(t, ok, "sequoia.json should configure Bluesky posting")
	assert.Equal(t, false, bluesky["enabled"])
	assert.Equal(t, float64(30), bluesky["maxAgeDays"])

	_, ok = config["ui"]
	assert.False(t, ok, "Sequoia should publish metadata only; no visible in-site UI components")
}

func TestStandardSiteDocumentLinkTagsWhenPublished(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"../content/go/anemic_stack_traces.md": "/go/anemic-stack-traces/",
		"../content/shards/2026/04/dynamo.md":  "/shards/2026/04/dynamo/",
	}

	for contentPath, url := range cases {
		t.Run(url, func(t *testing.T) {
			atURI := frontmatterScalar(t, contentPath, "atUri")
			if atURI == "" {
				t.Skip("post has not been published to Standard.site yet")
			}

			page := newPage(t)
			goto_(t, page, url)

			href, err := page.Locator(`link[rel="site.standard.document"]`).GetAttribute("href")
			require.NoError(t, err)
			assert.Equal(t, atURI, href)
		})
	}
}

func TestStandardSitePreviewCardUsesSiteImage(t *testing.T) {
	t.Parallel()

	page := newPage(t)
	goto_(t, page, "/misc/standard-site/")

	cardImage, err := page.Locator(".bskycard__media img").GetAttribute("src")
	require.NoError(t, err)
	ogImage, err := page.Locator(`meta[property="og:image"]`).GetAttribute("content")
	require.NoError(t, err)
	assert.Equal(t, ogImage, cardImage)
}

func TestStandardSitePostImagesFeedHugoAndSequoia(t *testing.T) {
	t.Parallel()

	contentPath := "../content/go/request_coalescing.md"
	images := frontmatterStringSlice(t, contentPath, "images")
	require.Len(t, images, 1)
	expectedURL := images[0]
	assert.True(t, strings.HasPrefix(expectedURL, postCardsAssetBaseURL+"/go/request-coalescing/cover-"))
	assert.True(t, strings.HasSuffix(expectedURL, ".png"))

	_, err := os.Stat(filepath.Join("..", "public/images/go/request-coalescing"))
	assert.True(t, os.IsNotExist(err), "generated card binaries should not be deployed through GitHub Pages")

	page := newPage(t)
	goto_(t, page, "/go/request-coalescing/")
	metaImage, err := page.Locator(`meta[property="og:image"]`).GetAttribute("content")
	require.NoError(t, err)
	assert.Equal(t, expectedURL, metaImage)
	metaWidth, err := page.Locator(`meta[property="og:image:width"]`).GetAttribute("content")
	require.NoError(t, err)
	assert.Equal(t, "4080", metaWidth)
	metaHeight, err := page.Locator(`meta[property="og:image:height"]`).GetAttribute("content")
	require.NoError(t, err)
	assert.Equal(t, "2142", metaHeight)
}

func TestStandardSiteFrontmatterPaths(t *testing.T) {
	t.Parallel()

	standardSiteSections := loadStandardSiteSections(t)
	var missing []string
	var wrong []string

	err := filepath.WalkDir("../content", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(filePath) != ".md" || filepath.Base(filePath) == "_index.md" {
			return nil
		}

		rel, err := filepath.Rel("../content", filePath)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		section := parts[0]
		if !standardSiteSections.publishes(section) {
			return nil
		}

		slug := frontmatterScalar(t, filePath, "slug")
		if slug == "" {
			missing = append(missing, filePath+":slug")
			return nil
		}

		got := frontmatterScalar(t, filePath, "atprotoPath")
		if got == "" {
			missing = append(missing, filePath+":atprotoPath")
			return nil
		}

		want := "/" + section + "/" + slug + "/"
		if section == standardSiteSections.notesSection {
			if len(parts) < 4 {
				wrong = append(wrong, filePath+": notes posts must live under content/"+standardSiteSections.notesSection+"/YYYY/MM/")
				return nil
			}
			want = "/" + standardSiteSections.notesSection + "/" + parts[1] + "/" + parts[2] + "/" + slug + "/"
		}

		if got != want {
			wrong = append(wrong, filePath+": got "+got+", want "+want)
		}

		return nil
	})
	require.NoError(t, err)
	require.Empty(t, missing, "publishable posts need Standard.site frontmatter")
	require.Empty(t, wrong, "Standard.site paths must match Hugo canonical paths")
}

func loadStandardSiteSections(t *testing.T) standardSiteSections {
	t.Helper()

	output, err := os.ReadFile("../config.yml")
	require.NoError(t, err)

	var config standardSiteConfig
	require.NoError(t, yaml.Unmarshal(output, &config))

	standardSiteSections := standardSiteSections{notesSection: config.Params.NotesSection}
	for _, section := range config.Params.MainSections {
		if section != "" {
			standardSiteSections.sections = append(standardSiteSections.sections, section)
		}
	}
	if standardSiteSections.notesSection != "" {
		standardSiteSections.sections = append(standardSiteSections.sections, standardSiteSections.notesSection)
	}

	require.NotEmpty(t, standardSiteSections.sections, "Hugo config should define publishable sections")
	return standardSiteSections
}

func (sections standardSiteSections) publishes(section string) bool {
	return slices.Contains(sections.sections, section)
}

func frontmatterScalar(t *testing.T, filePath, key string) string {
	t.Helper()

	values := frontmatterValues(t, filePath)
	value, ok := values[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func frontmatterStringSlice(t *testing.T, filePath, key string) []string {
	t.Helper()

	values := frontmatterValues(t, filePath)
	raw, ok := values[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		out = append(out, strings.TrimSpace(fmt.Sprint(value)))
	}
	return out
}

func frontmatterValues(t *testing.T, filePath string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	body, err := frontmatterBody(string(data))
	require.NoError(t, err)

	var values map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(body), &values))
	return values
}

func mustCount(t *testing.T, locator interface{ Count() (int, error) }) int {
	t.Helper()

	count, err := locator.Count()
	require.NoError(t, err)
	return count
}

func frontmatterBody(raw string) (string, error) {
	opening, lineEnding, ok := frontmatterOpening(raw)
	if !ok {
		return "", os.ErrInvalid
	}

	body, _, ok := strings.Cut(raw[len(opening):], lineEnding+"---"+lineEnding)
	if !ok {
		return "", os.ErrInvalid
	}
	return body, nil
}

func frontmatterOpening(raw string) (string, string, bool) {
	switch {
	case strings.HasPrefix(raw, "---\r\n"):
		return "---\r\n", "\r\n", true
	case strings.HasPrefix(raw, "---\n"):
		return "---\n", "\n", true
	default:
		return "", "", false
	}
}
