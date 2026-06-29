package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	defaultAssetBase = "https://blob.rednafi.com"
	cardDesignID     = "geist-fluid-card-v1"
)

var canonicalKeys = []string{
	"title",
	"slug",
	"date",
	"description",
	"tags",
	"images",
	"aliases",
	"discussions",
	"mermaid",
	"type_label",
	"atprotoPath",
	"atUri",
}

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

type discussion struct {
	Label string
	URL   string
}

type postFrontmatter struct {
	Title       string
	Slug        string
	Date        string
	Description string
	Tags        []string
	Images      []string
	Aliases     []string
	Discussions []discussion
	Mermaid     bool
	TypeLabel   string
	AtprotoPath string
	AtURI       string
}

func main() {
	check := flag.Bool("check", false, "fail if post frontmatter is not canonical")
	assetBaseURL := flag.String("asset-base-url", defaultAssetBase, "public base URL for generated post card image assets")
	flag.Parse()

	publishing, err := loadPublishConfig("config.yml")
	if err != nil {
		fatal(err)
	}

	var changed []string
	err = filepath.WalkDir("content", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(filePath) != ".md" || filepath.Base(filePath) == "_index.md" {
			return nil
		}
		if !slices.Contains(publishing.sections, sectionFor(filePath)) {
			return nil
		}

		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		raw := string(rawBytes)
		next, err := normalizePostFrontmatter(raw, filePath, publishing.notesSection, *assetBaseURL)
		if err != nil {
			return err
		}
		if next == raw {
			return nil
		}

		changed = append(changed, filePath)
		if !*check {
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
	if *check {
		verb = "need"
	}
	fmt.Printf("%d post frontmatter file%s %s normalization\n", len(changed), plural(len(changed)), verb)
	for _, filePath := range changed {
		fmt.Printf("  %s\n", filePath)
	}
	if *check {
		os.Exit(1)
	}
}

func normalizePostFrontmatter(raw, filePath, notesSection, assetBaseURL string) (string, error) {
	fmRaw, body, ok := splitFrontmatter(raw)
	if !ok {
		return "", fmt.Errorf("%s: missing YAML frontmatter", filePath)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmRaw), &doc); err != nil {
		return "", fmt.Errorf("%s: parse frontmatter: %w", filePath, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("%s: frontmatter must be a YAML mapping", filePath)
	}

	values, err := frontmatterMap(filePath, doc.Content[0])
	if err != nil {
		return "", err
	}

	post, err := canonicalPost(filePath, body, notesSection, assetBaseURL, values)
	if err != nil {
		return "", err
	}

	return "---\n" + renderFrontmatter(post) + "---\n" + body, nil
}

func frontmatterMap(filePath string, node *yaml.Node) (map[string]*yaml.Node, error) {
	known := map[string]bool{}
	for _, key := range canonicalKeys {
		known[key] = true
	}

	values := map[string]*yaml.Node{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !known[key] {
			return nil, fmt.Errorf("%s: unknown frontmatter key %q", filePath, key)
		}
		values[key] = node.Content[i+1]
	}
	return values, nil
}

func canonicalPost(filePath, body, notesSection, assetBaseURL string, values map[string]*yaml.Node) (postFrontmatter, error) {
	required := []string{"title", "slug", "date", "description", "tags"}
	for _, key := range required {
		if values[key] == nil {
			return postFrontmatter{}, fmt.Errorf("%s: missing required frontmatter key %q", filePath, key)
		}
	}

	slug := strings.TrimSpace(scalar(values["slug"]))
	if slug == "" {
		return postFrontmatter{}, fmt.Errorf("%s: slug cannot be empty", filePath)
	}

	atprotoPath, err := atprotoPathFor(filePath, slug, notesSection)
	if err != nil {
		return postFrontmatter{}, err
	}

	tags, err := stringSeq(filePath, "tags", values["tags"])
	if err != nil {
		return postFrontmatter{}, err
	}
	if len(tags) == 0 {
		return postFrontmatter{}, fmt.Errorf("%s: tags cannot be empty", filePath)
	}

	aliases, err := optionalStringSeq(filePath, "aliases", values["aliases"])
	if err != nil {
		return postFrontmatter{}, err
	}

	discussions, err := optionalDiscussions(filePath, values["discussions"])
	if err != nil {
		return postFrontmatter{}, err
	}

	return postFrontmatter{
		Title:       strings.TrimSpace(scalar(values["title"])),
		Slug:        slug,
		Date:        strings.TrimSpace(scalar(values["date"])),
		Description: strings.TrimSpace(scalar(values["description"])),
		Tags:        tags,
		Images:      []string{postCardURL(assetBaseURL, atprotoPath, strings.TrimSpace(scalar(values["title"])))},
		Aliases:     aliases,
		Discussions: discussions,
		Mermaid:     containsMermaid(body),
		TypeLabel:   strings.TrimSpace(scalar(values["type_label"])),
		AtprotoPath: atprotoPath,
		AtURI:       strings.TrimSpace(scalar(values["atUri"])),
	}, nil
}

func renderFrontmatter(post postFrontmatter) string {
	var b strings.Builder
	writeKeyValue(&b, "title", quoted(post.Title))
	writeKeyValue(&b, "slug", plainOrQuoted(post.Slug))
	writeKeyValue(&b, "date", post.Date)
	b.WriteString("description: >-\n")
	for _, line := range wrapWords(post.Description, 88) {
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	writeStringSeq(&b, "tags", post.Tags)
	writeStringSeq(&b, "images", post.Images)
	writeStringSeq(&b, "aliases", post.Aliases)
	writeDiscussions(&b, post.Discussions)
	writeKeyValue(&b, "mermaid", strconv.FormatBool(post.Mermaid))
	writeKeyValue(&b, "type_label", quoted(post.TypeLabel))
	writeKeyValue(&b, "atprotoPath", post.AtprotoPath)
	writeKeyValue(&b, "atUri", quoted(post.AtURI))
	return b.String()
}

func writeKeyValue(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteByte('\n')
}

func writeStringSeq(b *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		writeKeyValue(b, key, "[]")
		return
	}
	b.WriteString(key)
	b.WriteString(":\n")
	for _, value := range values {
		b.WriteString("    - ")
		b.WriteString(plainOrQuoted(value))
		b.WriteByte('\n')
	}
}

func writeDiscussions(b *strings.Builder, values []discussion) {
	if len(values) == 0 {
		b.WriteString("discussions: []\n")
		return
	}
	b.WriteString("discussions:\n")
	for _, value := range values {
		b.WriteString("    - label: ")
		b.WriteString(quoted(value.Label))
		b.WriteByte('\n')
		b.WriteString("      url: ")
		b.WriteString(plainOrQuoted(value.URL))
		b.WriteByte('\n')
	}
}

func splitFrontmatter(raw string) (frontmatter, body string, ok bool) {
	const start = "---\n"
	if !strings.HasPrefix(raw, start) {
		return "", "", false
	}
	idx := strings.Index(raw[len(start):], "\n---\n")
	if idx == -1 {
		return "", "", false
	}
	fmEnd := len(start) + idx
	bodyStart := fmEnd + len("\n---\n")
	return raw[len(start):fmEnd], raw[bodyStart:], true
}

func scalar(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

func stringSeq(filePath, key string, node *yaml.Node) ([]string, error) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: %s must be a sequence", filePath, key)
	}
	values := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: %s entries must be scalars", filePath, key)
		}
		values = append(values, strings.TrimSpace(item.Value))
	}
	return values, nil
}

func optionalStringSeq(filePath, key string, node *yaml.Node) ([]string, error) {
	if node == nil {
		return nil, nil
	}
	return stringSeq(filePath, key, node)
}

func optionalDiscussions(filePath string, node *yaml.Node) ([]discussion, error) {
	if node == nil {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: discussions must be a sequence", filePath)
	}
	var values []discussion
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: discussion entries must be mappings", filePath)
		}
		entry := discussion{}
		for i := 0; i < len(item.Content); i += 2 {
			switch item.Content[i].Value {
			case "label":
				entry.Label = strings.TrimSpace(item.Content[i+1].Value)
			case "url":
				entry.URL = strings.TrimSpace(item.Content[i+1].Value)
			default:
				return nil, fmt.Errorf("%s: unknown discussion key %q", filePath, item.Content[i].Value)
			}
		}
		if entry.Label == "" || entry.URL == "" {
			return nil, fmt.Errorf("%s: discussion entries need label and url", filePath)
		}
		values = append(values, entry)
	}
	return values, nil
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

func postCardURL(assetBaseURL, atprotoPath, title string) string {
	return strings.TrimRight(assetBaseURL, "/") + "/" + postCardKey(atprotoPath, title)
}

func postCardKey(atprotoPath, title string) string {
	clean := path.Clean("/" + atprotoPath)
	clean = strings.Trim(clean, "/")
	clean = strings.TrimSuffix(clean, "/index")
	clean = slugPath(clean)
	sum := sha256.Sum256([]byte(cardDesignID + "\n" + clean + "\n" + title))
	return clean + "/cover-" + hex.EncodeToString(sum[:])[:12] + ".png"
}

func sectionFor(filePath string) string {
	rel := strings.TrimPrefix(filepath.ToSlash(filePath), "content/")
	section, _, _ := strings.Cut(rel, "/")
	return section
}

func containsMermaid(body string) bool {
	return strings.Contains(body, "{{< mermaid >}}") || strings.Contains(body, "```mermaid")
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

var slugPartReplacer = strings.NewReplacer("_", "-")

func slugPath(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		part = slugPartReplacer.Replace(part)
		part = collapseSlugPart(part)
		if part == "" {
			part = "post"
		}
		parts[i] = part
	}
	return strings.Join(parts, "/")
}

func collapseSlugPart(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func plainOrQuoted(value string) string {
	if value == "" {
		return `""`
	}
	if isPlainSafe(value) {
		return value
	}
	return quoted(value)
}

func quoted(value string) string {
	return strconv.Quote(value)
}

func isPlainSafe(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) {
			return false
		}
		switch r {
		case ':', '[', ']', '{', '}', ',', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
			return false
		}
	}
	return true
}

func wrapWords(value string, width int) []string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, line)
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
