package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultStageDir        = ".cache/sequoia-publish"
	defaultContentDir      = "content"
	defaultImagesDir       = "images"
	defaultSiteCoverSource = ".cache/site-cover/cover-b8f8f0fc773d.png"
	defaultSiteCoverRel    = "images/home/cover-b8f8f0fc773d.png"
	defaultCommand         = "npx -y sequoia-cli@0.5.7 publish"
)

type frontmatter struct {
	AtURI string `yaml:"atUri"`
}

func main() {
	stageDir := flag.String("stage", defaultStageDir, "temporary Sequoia publishing workspace")
	siteCover := flag.String("site-cover", envOrDefault("SEQUOIA_SITE_COVER_SOURCE", defaultSiteCoverSource), "local site cover image to upload as every Standard.site cover")
	publishCmd := flag.String("publish-cmd", envOrDefault("SEQUOIA_CMD", defaultCommand), "Sequoia publish command")
	prepareOnly := flag.Bool("prepare-only", false, "prepare the temporary workspace and exit")
	syncOnly := flag.Bool("sync-only", false, "sync Sequoia state and atUri values back from the temporary workspace")
	flag.Parse()

	if *syncOnly {
		if err := syncBack(*stageDir); err != nil {
			fatal(err)
		}
		return
	}

	if err := prepareWorkspace(*stageDir, *siteCover); err != nil {
		fatal(err)
	}
	if *prepareOnly {
		return
	}

	if err := syncBackAfterPublish(*stageDir, func() error {
		return runPublish(*stageDir, *publishCmd)
	}); err != nil {
		fatal(err)
	}
}

func syncBackAfterPublish(stageDir string, publish func() error) error {
	publishErr := publish()
	syncErr := syncBack(stageDir)
	if publishErr != nil || syncErr != nil {
		if publishErr != nil && syncErr != nil {
			return fmt.Errorf("publish failed: %w; sync back failed: %v", publishErr, syncErr)
		}
		if publishErr != nil {
			return publishErr
		}
		return syncErr
	}
	return nil
}

func prepareWorkspace(stageDir, siteCover string) error {
	if err := os.RemoveAll(stageDir); err != nil {
		return err
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return err
	}

	if err := writeStageConfig(stageDir); err != nil {
		return err
	}
	if err := copyIfExists(".sequoia-state.json", filepath.Join(stageDir, ".sequoia-state.json")); err != nil {
		return err
	}
	if err := stageSiteCover(stageDir, siteCover); err != nil {
		return err
	}
	if err := copyContent(stageDir); err != nil {
		return err
	}
	fmt.Println("Prepared Sequoia workspace.")
	return nil
}

func writeStageConfig(stageDir string) error {
	raw, err := os.ReadFile("sequoia.json")
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	config["contentDir"] = defaultContentDir
	config["imagesDir"] = defaultImagesDir

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(filepath.Join(stageDir, "sequoia.json"), out, 0o644)
}

func stageSiteCover(stageDir, siteCover string) error {
	raw, err := os.ReadFile(siteCover)
	if err != nil {
		return fmt.Errorf("read site cover %s: %w", siteCover, err)
	}
	if len(raw) >= 1_000_000 {
		return fmt.Errorf("site cover %s is %d bytes; Sequoia cover images must be less than 1MB", siteCover, len(raw))
	}
	return writeFile(filepath.Join(stageDir, filepath.FromSlash(defaultSiteCoverRel)), raw, 0o644)
}

func copyContent(stageDir string) error {
	return filepath.WalkDir(defaultContentDir, func(src string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(defaultContentDir, src)
		if err != nil {
			return err
		}
		dst := filepath.Join(stageDir, defaultContentDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if filepath.Ext(src) != ".md" {
			return writeFile(dst, raw, 0o644)
		}
		if filepath.Base(src) == "_index.md" {
			return writeFile(dst, raw, 0o644)
		}
		next := injectSiteCover(string(raw))
		return writeFile(dst, []byte(next), 0o644)
	})
}

func injectSiteCover(raw string) string {
	fmRaw, body, ok := splitFrontmatter(raw)
	if !ok {
		return raw
	}

	ogLine := "ogImage: " + quoted(defaultSiteCoverRel) + "\n"
	if strings.Contains(fmRaw, "\nogImage:") || strings.HasPrefix(fmRaw, "ogImage:") {
		nextFM := replaceFrontmatterScalar(fmRaw, "ogImage", strings.TrimSpace(ogLine))
		return joinFrontmatter(nextFM, body)
	}
	if strings.Contains(fmRaw, "\naliases:") {
		nextFM := strings.Replace(fmRaw, "\naliases:", "\n"+ogLine+"aliases:", 1)
		return joinFrontmatter(nextFM, body)
	}
	return joinFrontmatter(strings.TrimRight(fmRaw, "\n")+"\n"+ogLine, body)
}

func runPublish(stageDir, command string) error {
	cmd := exec.Command("bash", "-lc", command)
	cmd.Dir = stageDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

func syncBack(stageDir string) error {
	if err := copyIfExists(filepath.Join(stageDir, ".sequoia-state.json"), ".sequoia-state.json"); err != nil {
		return err
	}
	return filepath.WalkDir(filepath.Join(stageDir, defaultContentDir), func(stagedPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(stagedPath) != ".md" {
			return nil
		}
		if filepath.Base(stagedPath) == "_index.md" {
			return nil
		}
		rel, err := filepath.Rel(filepath.Join(stageDir, defaultContentDir), stagedPath)
		if err != nil {
			return err
		}
		rootPath := filepath.Join(defaultContentDir, rel)
		atURI, err := frontmatterAtURI(stagedPath)
		if err != nil {
			return fmt.Errorf("%s: %w", stagedPath, err)
		}
		if atURI == "" {
			return nil
		}
		return updateRootAtURI(rootPath, atURI)
	})
}

func frontmatterAtURI(filePath string) (string, error) {
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	fmRaw, _, ok := splitFrontmatter(string(rawBytes))
	if !ok {
		return "", nil
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return "", err
	}
	return strings.TrimSpace(fm.AtURI), nil
}

func updateRootAtURI(filePath, atURI string) error {
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	raw := string(rawBytes)
	fmRaw, body, ok := splitFrontmatter(raw)
	if !ok {
		return nil
	}
	nextFM := replaceFrontmatterScalar(fmRaw, "atUri", "atUri: "+quoted(atURI))
	next := joinFrontmatter(nextFM, body)
	if next == raw {
		return nil
	}
	return os.WriteFile(filePath, []byte(next), 0o644)
}

func replaceFrontmatterScalar(frontmatter, key, replacement string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:.*$`)
	if re.MatchString(frontmatter) {
		return re.ReplaceAllStringFunc(frontmatter, func(string) string {
			return replacement
		})
	}
	return strings.TrimRight(frontmatter, "\n") + "\n" + replacement + "\n"
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

func joinFrontmatter(frontmatter, body string) string {
	return "---\n" + strings.TrimRight(frontmatter, "\n") + "\n---\n" + body
}

func copyIfExists(src, dst string) error {
	raw, err := os.ReadFile(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return writeFile(dst, raw, 0o644)
}

func writeFile(dst string, raw []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, perm)
}

func quoted(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "sequoia:", err)
	os.Exit(1)
}
