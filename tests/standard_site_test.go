package site_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const standardSitePublicationURI = "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.publication/3mnl6f7ob462z"

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
	for _, publishedSection := range sections.sections {
		if publishedSection == section {
			return true
		}
	}
	return false
}

func frontmatterScalar(t *testing.T, filePath, key string) string {
	t.Helper()

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	body, err := frontmatterBody(string(data))
	require.NoError(t, err)

	var values map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(body), &values))

	value, ok := values[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
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
