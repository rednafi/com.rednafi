package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPostFrontmatterConformsToCanonicalFormat(t *testing.T) {
	root := repoRoot(t)

	publishing, err := loadPublishConfig(filepath.Join(root, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var violations []string
	contentDir := filepath.Join(root, "content")
	err = filepath.WalkDir(contentDir, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(filePath) != ".md" || filepath.Base(filePath) == "_index.md" {
			return nil
		}

		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !slices.Contains(publishing.sections, sectionFor(rel)) {
			return nil
		}

		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		raw := string(rawBytes)
		normalized, err := normalizePostFrontmatter(raw, rel, publishing.notesSection, defaultAssetBase)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", rel, err))
			return nil
		}
		if normalized != raw {
			violations = append(violations, rel)
			return nil
		}
		keys, err := frontmatterKeys(raw)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", rel, err))
			return nil
		}
		if !slices.Equal(keys, canonicalKeys) {
			violations = append(violations, fmt.Sprintf("%s: frontmatter keys are %v, want %v", rel, keys, canonicalKeys))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("post frontmatter is not canonical; run go run ./scripts/frontmatter:\n  %s", strings.Join(violations, "\n  "))
	}
}

func frontmatterKeys(raw string) ([]string, error) {
	fmRaw, _, ok := splitFrontmatter(raw)
	if !ok {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmRaw), &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter must be a YAML mapping")
	}

	node := doc.Content[0]
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys, nil
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root")
		}
		dir = parent
	}
}
