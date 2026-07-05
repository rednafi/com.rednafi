package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The wake-mark partial is generated from the cover geometry; a stale copy
// means the landing mark no longer matches the shipped badge.
func TestHeroPartialsInSync(t *testing.T) {
	for _, f := range heroPartials() {
		got, err := os.ReadFile(filepath.Join("..", "..", f.path))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != f.svg {
			t.Errorf("%s is stale; run `go run ./scripts/ogimage -hero`", f.path)
		}
	}
}
