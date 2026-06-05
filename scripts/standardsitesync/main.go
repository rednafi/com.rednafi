package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type siteConfig struct {
	Params struct {
		MainSections []string `yaml:"mainSections"`
		NotesSection string   `yaml:"notesSection"`
	} `yaml:"params"`
}

type publishConfig struct {
	sections     []string
	notesSection string
}

type postFrontmatter struct {
	Slug string `yaml:"slug"`
}

func main() {
	checkOnly := flag.Bool("check", false, "check whether Standard.site frontmatter is current")
	flag.Parse()

	publishing, err := loadPublishConfig("config.yml")
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
		if !publishing.publishes(sectionFor(filePath)) {
			return nil
		}

		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		raw := string(rawBytes)
		next, err := syncAtprotoPath(raw, filePath, publishing.notesSection)
		if err != nil {
			return err
		}
		if next == raw {
			return nil
		}

		changed = append(changed, filePath)
		if !*checkOnly {
			return os.WriteFile(filePath, []byte(next), 0o644)
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

func loadPublishConfig(configPath string) (publishConfig, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return publishConfig{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	var config siteConfig
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return publishConfig{}, fmt.Errorf("parse %s: %w", configPath, err)
	}

	publishing := publishConfig{notesSection: config.Params.NotesSection}
	for _, section := range config.Params.MainSections {
		if section != "" {
			publishing.sections = append(publishing.sections, section)
		}
	}

	if publishing.notesSection != "" {
		publishing.sections = append(publishing.sections, publishing.notesSection)
	}
	if len(publishing.sections) == 0 {
		return publishConfig{}, fmt.Errorf("%s did not define params.mainSections or params.notesSection", configPath)
	}

	return publishing, nil
}

func (config publishConfig) publishes(section string) bool {
	for _, publishedSection := range config.sections {
		if publishedSection == section {
			return true
		}
	}
	return false
}

func syncAtprotoPath(raw, filePath, notesSection string) (string, error) {
	opening, body, closing, rest, err := splitFrontmatter(raw, filePath)
	if err != nil {
		return "", err
	}

	var fm postFrontmatter
	if err := yaml.Unmarshal([]byte(body), &fm); err != nil {
		return "", fmt.Errorf("%s: parse frontmatter: %w", filePath, err)
	}

	slug := strings.TrimSpace(fm.Slug)
	if slug == "" {
		return "", fmt.Errorf("%s: missing slug frontmatter", filePath)
	}

	expected, err := atprotoPathFor(filePath, slug, notesSection)
	if err != nil {
		return "", err
	}

	body = setFrontmatterLine(body, "atprotoPath", expected, "slug")
	return opening + body + closing + rest, nil
}

func splitFrontmatter(raw, filePath string) (string, string, string, string, error) {
	lineEnding := "\n"
	opening := "---\n"
	if strings.HasPrefix(raw, "---\r\n") {
		lineEnding = "\r\n"
		opening = "---\r\n"
	} else if !strings.HasPrefix(raw, opening) {
		return "", "", "", "", fmt.Errorf("%s: missing YAML frontmatter", filePath)
	}

	closing := lineEnding + "---" + lineEnding
	body, rest, ok := strings.Cut(raw[len(opening):], closing)
	if !ok {
		return "", "", "", "", fmt.Errorf("%s: missing YAML frontmatter closing delimiter", filePath)
	}
	return opening, body, closing, rest, nil
}

func setFrontmatterLine(body, key, value, insertAfter string) string {
	lineEnding := "\n"
	if strings.Contains(body, "\r\n") {
		lineEnding = "\r\n"
	}

	line := key + ": " + value
	lines := strings.Split(body, lineEnding)

	for i, current := range lines {
		if strings.HasPrefix(current, key+":") {
			lines[i] = line
			return strings.Join(lines, lineEnding)
		}
	}

	inserted := false
	out := make([]string, 0, len(lines)+1)
	for _, current := range lines {
		out = append(out, current)
		if !inserted && strings.HasPrefix(current, insertAfter+":") {
			out = append(out, line)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, line)
	}
	return strings.Join(out, lineEnding)
}

func atprotoPathFor(filePath, slug, notesSection string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(filepath.ToSlash(filePath), "content/"), "/")
	section := parts[0]

	if section == notesSection {
		if len(parts) < 4 {
			return "", fmt.Errorf("%s: notes posts must live under content/%s/YYYY/MM/", filePath, notesSection)
		}
		return fmt.Sprintf("/%s/%s/%s/%s/", notesSection, parts[1], parts[2], slug), nil
	}

	return fmt.Sprintf("/%s/%s/", section, slug), nil
}

func sectionFor(filePath string) string {
	rel := strings.TrimPrefix(filepath.ToSlash(filePath), "content/")
	section, _, _ := strings.Cut(rel, "/")
	return section
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
