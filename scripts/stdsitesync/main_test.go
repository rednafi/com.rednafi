package main

import (
	"strings"
	"testing"
)

func TestSyncAtprotoPathNoopPreservesFile(t *testing.T) {
	raw := `---
title: Existing metadata
slug: existing-metadata
atprotoPath: /misc/existing-metadata/
aliases:
    - /misc/existing_metadata/
---

# Existing metadata

Body bytes should stay exactly as they are.
`

	got, err := syncAtprotoPath(raw, "content/misc/existing-metadata.md", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Fatal("already-current file should be returned byte-for-byte")
	}
}

func TestSyncAtprotoPathInsertsAtprotoPathAndPreservesMarkdownBody(t *testing.T) {
	markdownBody := "\n# Insert metadata\r\n\r\n```yaml\n---\nnot: frontmatter\n---\n```\n"
	raw := `---
title: Insert metadata
slug: insert-metadata
aliases:
    - /misc/insert_metadata/
---
` + markdownBody

	got, err := syncAtprotoPath(raw, "content/misc/insert-metadata.md", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, markdownBody) {
		t.Fatalf("markdown body changed:\n%s", got)
	}
	if !strings.Contains(got, "atprotoPath: /misc/insert-metadata/") {
		t.Fatalf("atprotoPath was not inserted:\n%s", got)
	}
}

func TestSyncAtprotoPathUpdatesAtprotoPathAndPreservesMarkdownBody(t *testing.T) {
	markdownBody := "\n# Update metadata\n\nThe body must not be normalized.\n"
	raw := `---
title: Update metadata
slug: update-metadata
atprotoPath: /misc/wrong/
---
` + markdownBody

	got, err := syncAtprotoPath(raw, "content/misc/update-metadata.md", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, markdownBody) {
		t.Fatalf("markdown body changed:\n%s", got)
	}
	if strings.Contains(got, "atprotoPath: /misc/wrong/") {
		t.Fatalf("old atprotoPath remained:\n%s", got)
	}
	if !strings.Contains(got, "atprotoPath: /misc/update-metadata/") {
		t.Fatalf("new atprotoPath missing:\n%s", got)
	}
}
