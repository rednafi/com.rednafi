package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixTabs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"single tab", "a\tb", "a    b"},
		{"leading tab", "\tcode", "    code"},
		{"multiple tabs", "\t\tx", "        x"},
		{"no tabs", "plain text\n", "plain text\n"},
		{"tabs across lines", "line1\n\tline2\n", "line1\n    line2\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(fixTabs([]byte(c.in))); got != c.want {
				t.Errorf("fixTabs(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.md"), "x")
	mustWrite(t, filepath.Join(dir, "nested", "b.md"), "y")
	mustWrite(t, filepath.Join(dir, "skip.txt"), "z")

	files, err := markdownFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("found %d markdown files, want 2: %v", len(files), files)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
