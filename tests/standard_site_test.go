package site_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const standardSitePublicationURI = "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.publication/self"

type standardSiteHugoConfig struct {
	Params struct {
		MainSections []string `json:"mainsections"`
		NotesSection string   `json:"notessection"`
	} `json:"params"`
}

func TestStandardSiteWellKnownEndpoint(t *testing.T) {
	t.Parallel()

	body := strings.TrimSpace(httpGet(t, baseURL+"/.well-known/site.standard.publication"))
	assert.Equal(t, standardSitePublicationURI, body)
}

func TestStandardSitePublicationLinkTags(t *testing.T) {
	t.Parallel()

	pages := []string{"/", "/go/anemic-stack-traces/", "/shards/2026/04/dynamo/"}
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

func TestStandardSiteFrontmatterPaths(t *testing.T) {
	t.Parallel()

	standardSiteSections, notesSection := loadStandardSiteSections(t)
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
		if !standardSiteSections[section] {
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
		if section == notesSection {
			if len(parts) < 4 {
				wrong = append(wrong, filePath+": notes posts must live under content/"+notesSection+"/YYYY/MM/")
				return nil
			}
			want = "/" + notesSection + "/" + parts[1] + "/" + parts[2] + "/" + slug + "/"
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

func loadStandardSiteSections(t *testing.T) (map[string]bool, string) {
	t.Helper()

	cmd := exec.Command("hugo", "config", "--format", "json")
	cmd.Dir = ".."
	output, err := cmd.Output()
	require.NoError(t, err)

	var config standardSiteHugoConfig
	require.NoError(t, json.Unmarshal(output, &config))

	sections := make(map[string]bool)
	for _, section := range config.Params.MainSections {
		if section != "" {
			sections[section] = true
		}
	}
	notesSection := config.Params.NotesSection
	if notesSection != "" {
		sections[notesSection] = true
	}

	require.NotEmpty(t, sections, "Hugo config should define publishable sections")
	return sections, notesSection
}

func frontmatterScalar(t *testing.T, filePath, key string) string {
	t.Helper()

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	match := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*"?([^"\n]+)"?\s*$`).FindSubmatch(data)
	if len(match) < 2 {
		return ""
	}

	return strings.TrimSpace(string(match[1]))
}
