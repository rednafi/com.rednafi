package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWorkspaceStagesSiteCoverForSequoiaOnly(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	coverPath := filepath.Join(tempDir, "cover.png")
	coverBytes := []byte("png")
	if err := os.WriteFile(coverPath, coverBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(defaultContentDir, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	rawPost := `---
title: "Request coalescing with Go singleflight"
aliases: []
---
Body.
`
	if err := os.WriteFile(filepath.Join(defaultContentDir, "go", "example.md"), []byte(rawPost), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("sequoia.json", []byte(`{"contentDir":"old","imagesDir":"static"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".sequoia-state.json", []byte(`{"posts":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stageDir := filepath.Join(tempDir, "stage")
	if err := prepareWorkspace(stageDir, coverPath); err != nil {
		t.Fatal(err)
	}

	stagedPost, err := os.ReadFile(filepath.Join(stageDir, defaultContentDir, "go", "example.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantPost := `---
title: "Request coalescing with Go singleflight"
ogImage: "images/home/cover-39e2ac8de020.png"
aliases: []
---
Body.
`
	if string(stagedPost) != wantPost {
		t.Fatalf("staged content mismatch:\n%s", stagedPost)
	}

	stagedCover, err := os.ReadFile(filepath.Join(stageDir, filepath.FromSlash(defaultSiteCoverRel)))
	if err != nil {
		t.Fatal(err)
	}
	if string(stagedCover) != string(coverBytes) {
		t.Fatalf("staged cover mismatch: %q", stagedCover)
	}

	var config map[string]any
	stagedConfig, err := os.ReadFile(filepath.Join(stageDir, "sequoia.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stagedConfig, &config); err != nil {
		t.Fatal(err)
	}
	if config["contentDir"] != defaultContentDir {
		t.Fatalf("contentDir = %v", config["contentDir"])
	}
	if config["imagesDir"] != defaultImagesDir {
		t.Fatalf("imagesDir = %v", config["imagesDir"])
	}

	rootPost, err := os.ReadFile(filepath.Join(defaultContentDir, "go", "example.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(rootPost) != rawPost {
		t.Fatalf("source content changed:\n%s", rootPost)
	}
}

func TestPrepareWorkspaceDoesNotInjectCoverIntoSectionIndexes(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	coverPath := filepath.Join(tempDir, "cover.png")
	if err := os.WriteFile(coverPath, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	sectionDir := filepath.Join(defaultContentDir, "javascript")
	if err := os.MkdirAll(sectionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rawIndex := `---
title: "JavaScript"
description: "The JavaScript programming language."
---
`
	if err := os.WriteFile(filepath.Join(sectionDir, "_index.md"), []byte(rawIndex), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("sequoia.json", []byte(`{"contentDir":"old","imagesDir":"static"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stageDir := filepath.Join(tempDir, "stage")
	if err := prepareWorkspace(stageDir, coverPath); err != nil {
		t.Fatal(err)
	}

	stagedIndex, err := os.ReadFile(filepath.Join(stageDir, sectionDir, "_index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stagedIndex) != rawIndex {
		t.Fatalf("section index should not be changed:\n%s", stagedIndex)
	}
}

func TestPrepareWorkspaceStripsEmptyAtURI(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	coverPath := filepath.Join(tempDir, "cover.png")
	if err := os.WriteFile(coverPath, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(defaultContentDir, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	rawNew := `---
title: "New post"
aliases: []
atprotoPath: /go/new-post/
atUri: ""
---
Body.
`
	if err := os.WriteFile(filepath.Join(defaultContentDir, "go", "new.md"), []byte(rawNew), 0o644); err != nil {
		t.Fatal(err)
	}
	rawPublished := `---
title: "Published post"
aliases: []
atprotoPath: /go/published-post/
atUri: "at://did:plc:example/site.standard.document/abc"
---
Body.
`
	if err := os.WriteFile(filepath.Join(defaultContentDir, "go", "published.md"), []byte(rawPublished), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("sequoia.json", []byte(`{"contentDir":"old","imagesDir":"static"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stageDir := filepath.Join(tempDir, "stage")
	if err := prepareWorkspace(stageDir, coverPath); err != nil {
		t.Fatal(err)
	}

	stagedNew, err := os.ReadFile(filepath.Join(stageDir, defaultContentDir, "go", "new.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantNew := `---
title: "New post"
ogImage: "images/home/cover-39e2ac8de020.png"
aliases: []
atprotoPath: /go/new-post/
---
Body.
`
	if string(stagedNew) != wantNew {
		t.Fatalf("staged content mismatch:\n%s", stagedNew)
	}

	stagedPublished, err := os.ReadFile(filepath.Join(stageDir, defaultContentDir, "go", "published.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantPublished := `---
title: "Published post"
ogImage: "images/home/cover-39e2ac8de020.png"
aliases: []
atprotoPath: /go/published-post/
atUri: "at://did:plc:example/site.standard.document/abc"
---
Body.
`
	if string(stagedPublished) != wantPublished {
		t.Fatalf("staged published content mismatch:\n%s", stagedPublished)
	}
}

func TestOrphanDocumentsOnlyDeletesUnreferencedDuplicates(t *testing.T) {
	records := []document{
		{URI: "at://did/site.standard.document/live", Path: "/go/gofix/"},
		{URI: "at://did/site.standard.document/dupe1", Path: "/go/gofix/"},
		{URI: "at://did/site.standard.document/dupe2", Path: "/go/gofix/"},
		{URI: "at://did/site.standard.document/other", Path: "/go/other/"},
		{URI: "at://did/site.standard.document/stray", Path: "/go/deleted-post/"},
		{URI: "at://did/site.standard.document/blank", Path: ""},
	}
	referenced := map[string]bool{
		"at://did/site.standard.document/live":  true,
		"at://did/site.standard.document/other": true,
	}

	orphans := orphanDocuments(records, referenced)

	want := []string{
		"at://did/site.standard.document/dupe1",
		"at://did/site.standard.document/dupe2",
	}
	if len(orphans) != len(want) {
		t.Fatalf("orphans = %v, want %v", orphans, want)
	}
	for i, uri := range want {
		if orphans[i].URI != uri {
			t.Fatalf("orphans[%d] = %q, want %q", i, orphans[i].URI, uri)
		}
	}
}

func TestSyncBackAfterPublishPreservesPartialPublishState(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	stageDir := filepath.Join(tempDir, "stage")
	if err := os.MkdirAll(filepath.Join(stageDir, defaultContentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	stagedState := `{"posts":{"content/go/example.md":{"contentHash":"abc","atUri":"at://example","slug":"example"}}}`
	if err := os.WriteFile(filepath.Join(stageDir, ".sequoia-state.json"), []byte(stagedState), 0o644); err != nil {
		t.Fatal(err)
	}

	publishErr := errors.New("publish failed after partial progress")
	err := syncBackAfterPublish(stageDir, func() error {
		return publishErr
	})
	if !errors.Is(err, publishErr) {
		t.Fatalf("syncBackAfterPublish error = %v, want publish error", err)
	}

	rootState, err := os.ReadFile(".sequoia-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(rootState) != stagedState {
		t.Fatalf("root state was not synced after publish failure:\n%s", rootState)
	}
}
