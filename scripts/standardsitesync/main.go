package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type hugoConfig struct {
	Params struct {
		MainSections []string `json:"mainsections"`
		NotesSection string   `json:"notessection"`
	} `json:"params"`
}

var (
	frontmatterRE = regexp.MustCompile(`(?s)^(---\r?\n)(.*?)(\r?\n---\r?\n)`)
	atprotoPathRE = regexp.MustCompile(`(?m)^atprotoPath:.*$`)
	slugLineRE    = regexp.MustCompile(`(?m)^slug:.*$`)
)

type frontmatter struct {
	opening string
	body    string
	closing string
	rest    string
}

func main() {
	checkOnly := flag.Bool("check", false, "check whether Standard.site frontmatter is current")
	flag.Parse()

	publishedSections, notesSection, err := loadPublishedSections()
	if err != nil {
		fatal(err)
	}

	var changed []string
	err = filepath.WalkDir("content", func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(filePath) != ".md" || filepath.Base(filePath) == "_index.md" {
			return nil
		}

		section := sectionFor(filePath)
		if !publishedSections[section] {
			return nil
		}

		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		raw := string(rawBytes)
		next, err := syncAtprotoPath(raw, filePath, publishedSections, notesSection)
		if err != nil {
			return err
		}
		if next == raw {
			return nil
		}

		changed = append(changed, filePath)
		if !*checkOnly {
			if err := os.WriteFile(filePath, []byte(next), 0o644); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fatal(err)
	}

	if len(changed) == 0 {
		return
	}

	verb := "updated"
	if *checkOnly {
		verb = "need"
	}
	fmt.Printf("%d Standard.site frontmatter file%s %s\n", len(changed), plural(len(changed)), verb)
	for _, filePath := range changed {
		fmt.Printf("  %s\n", filePath)
	}
	if *checkOnly {
		os.Exit(1)
	}
}

func loadPublishedSections() (map[string]bool, string, error) {
	output, err := exec.Command("hugo", "config", "--format", "json").Output()
	if err != nil {
		return nil, "", fmt.Errorf("read Hugo config: %w", err)
	}

	var config hugoConfig
	if err := json.Unmarshal(output, &config); err != nil {
		return nil, "", fmt.Errorf("parse Hugo config JSON: %w", err)
	}

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
	if len(sections) == 0 {
		return nil, "", fmt.Errorf("Hugo config did not define params.mainSections or params.notesSection")
	}

	return sections, notesSection, nil
}

func syncAtprotoPath(raw, filePath string, publishedSections map[string]bool, notesSection string) (string, error) {
	fm, err := parseFrontmatter(raw, filePath)
	if err != nil {
		return "", err
	}

	slug := readScalar(fm.body, "slug")
	if slug == "" {
		return "", fmt.Errorf("%s: missing slug frontmatter", filePath)
	}

	expectedPath, err := atprotoPathFor(filePath, slug, publishedSections, notesSection)
	if err != nil {
		return "", err
	}
	if expectedPath == "" {
		return raw, nil
	}

	body := fm.body
	if atprotoPathRE.MatchString(body) {
		body = atprotoPathRE.ReplaceAllString(body, "atprotoPath: "+expectedPath)
	} else {
		body = slugLineRE.ReplaceAllString(body, "$0\natprotoPath: "+expectedPath)
	}

	return fm.opening + body + fm.closing + fm.rest, nil
}

func parseFrontmatter(raw, filePath string) (frontmatter, error) {
	match := frontmatterRE.FindStringSubmatchIndex(raw)
	if match == nil {
		return frontmatter{}, fmt.Errorf("%s: missing YAML frontmatter", filePath)
	}

	return frontmatter{
		opening: raw[match[2]:match[3]],
		body:    raw[match[4]:match[5]],
		closing: raw[match[6]:match[7]],
		rest:    raw[match[1]:],
	}, nil
}

func readScalar(body, field string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `:\s*["']?([^"'\n#]+?)["']?\s*$`)
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func atprotoPathFor(filePath, slug string, publishedSections map[string]bool, notesSection string) (string, error) {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	if len(parts) < 2 {
		return "", nil
	}

	section := parts[1]
	if !publishedSections[section] {
		return "", nil
	}

	if section == notesSection {
		if len(parts) < 5 {
			return "", fmt.Errorf("%s: notes posts must live under content/%s/YYYY/MM/", filePath, notesSection)
		}
		return fmt.Sprintf("/%s/%s/%s/%s/", notesSection, parts[2], parts[3], slug), nil
	}

	return fmt.Sprintf("/%s/%s/", section, slug), nil
}

func sectionFor(filePath string) string {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
