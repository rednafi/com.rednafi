// Command lintcodeblocks normalizes whitespace in the site's Markdown.
//
// Markdown files under content/ must use spaces, not tabs (tabs render
// inconsistently inside fenced code blocks). By default it rewrites every
// offending file in place, replacing each tab with four spaces. With
// --check it touches nothing and exits non-zero if any file contains a tab,
// which is what CI / pre-commit hooks want.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const contentDir = "content"

var tab = []byte("\t")
var fourSpaces = []byte("    ")

// fixTabs replaces every tab with four spaces.
func fixTabs(b []byte) []byte {
	return bytes.ReplaceAll(b, tab, fourSpaces)
}

// markdownFiles returns every *.md file under dir, recursively.
func markdownFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".md" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func main() {
	check := flag.Bool("check", false, "report files containing tabs and exit non-zero instead of fixing them")
	flag.Parse()

	files, err := markdownFiles(contentDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	failed := false
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !bytes.Contains(content, tab) {
			continue
		}

		if *check {
			fmt.Printf("ERROR: %s contains tabs\n", file)
			failed = true
			continue
		}

		info, err := os.Stat(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(file, fixTabs(content), info.Mode().Perm()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Fixed tabs in: %s\n", file)
	}

	if failed {
		os.Exit(1)
	}
	if *check {
		fmt.Println("All markdown files pass linting")
	}
}
