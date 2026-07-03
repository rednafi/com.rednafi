package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlugPath(t *testing.T) {
	cases := map[string]string{
		"My_Image Name":    "my-image-name",
		"a/B_c":            "a/b-c",
		"  spaced  ":       "spaced",
		"":                 "image",
		"///":              "image/image/image/image",
		"singleflight-2x":  "singleflight-2x",
		"UPPER.case.png":   "upper-case-png",
		"shards/2026/wk01": "shards/2026/wk01",
	}
	for in, want := range cases {
		if got := slugPath(in); got != want {
			t.Errorf("slugPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectImageExt(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 512)...)
	jpg := append([]byte{0xff, 0xd8, 0xff, 0xe0}, make([]byte, 512)...)
	gif := append([]byte("GIF89a"), make([]byte, 512)...)
	cases := []struct {
		raw  []byte
		want string
	}{
		{png, ".png"},
		{jpg, ".jpg"},
		{gif, ".gif"},
		{[]byte("plain text, not an image"), ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := detectImageExt(c.raw); got != c.want {
			t.Errorf("detectImageExt(%d bytes) = %q, want %q", len(c.raw), got, c.want)
		}
	}
}

func TestContentType(t *testing.T) {
	cases := map[string]string{
		"a/b.svg":  "image/svg+xml; charset=utf-8",
		"a/b.jpg":  "image/jpeg",
		"a/b.jpeg": "image/jpeg",
		"a/b.png":  "image/png",
		"a/b.zzz":  "application/octet-stream",
	}
	for in, want := range cases {
		if got := contentType(in); got != want {
			t.Errorf("contentType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalDir(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("content/go/my_post.md", "---\ntitle: \"T\"\n---\nBody.\n")
	write("content/python/sluggy.md", "---\ntitle: \"T\"\nslug: custom-slug\n---\nBody.\n")
	write("content/shards/2026/06/note_one.md", "---\ntitle: \"T\"\n---\nBody.\n")

	cases := []struct {
		post, want string
	}{
		{"content/go/my_post.md", "go/my-post"},
		{"content/python/sluggy.md", "python/custom-slug"},
		{"content/shards/2026/06/note_one.md", "shards/2026/06/note-one"},
		{"config.yml", "about"},
	}
	for _, c := range cases {
		got, err := canonicalDir(c.post)
		if err != nil {
			t.Errorf("canonicalDir(%q): %v", c.post, err)
			continue
		}
		if got != c.want {
			t.Errorf("canonicalDir(%q) = %q, want %q", c.post, got, c.want)
		}
	}

	if _, err := canonicalDir("static/img.png"); err == nil {
		t.Error("canonicalDir outside content/ should error")
	}
}

func TestBlobURLs(t *testing.T) {
	raw := `![a](https://blob.rednafi.com/go/post/img-abc.png) and
[b]: https://blob.rednafi.com/python/under_score/img.png "t"`
	urls := blobURLs(raw)
	if len(urls) != 2 {
		t.Fatalf("blobURLs found %d URLs, want 2: %v", len(urls), urls)
	}
	violations, err := nonCanonicalURLsInFile(writeTemp(t, raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("want 1 underscore violation, got %d: %v", len(violations), violations)
	}
}

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "post.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
