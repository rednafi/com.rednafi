package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultStageDir        = ".cache/sequoia-publish"
	defaultContentDir      = "content"
	defaultImagesDir       = "images"
	defaultSiteCoverSource = ".cache/site-cover/cover-39e2ac8de020.png"
	defaultSiteCoverRel    = "images/home/cover-39e2ac8de020.png"
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

	if err := reconcileDocuments(); err != nil {
		fmt.Println("Skipping document reconcile:", err)
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
		next := stripEmptyAtURI(injectSiteCover(string(raw)))
		return writeFile(dst, []byte(next), 0o644)
	})
}

var emptyAtURIRe = regexp.MustCompile(`(?m)^atUri:\s*(""|'')?\s*$\n?`)

func stripEmptyAtURI(raw string) string {
	fmRaw, body, ok := splitFrontmatter(raw)
	if !ok {
		return raw
	}
	nextFM := emptyAtURIRe.ReplaceAllString(fmRaw, "")
	if nextFM == fmRaw {
		return raw
	}
	return joinFrontmatter(nextFM, body)
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

type document struct {
	URI  string
	Path string
}

func reconcileDocuments() error {
	identifier := strings.TrimSpace(os.Getenv("ATP_IDENTIFIER"))
	password := strings.TrimSpace(os.Getenv("ATP_APP_PASSWORD"))
	if identifier == "" || password == "" {
		return errors.New("ATP credentials not set")
	}
	referenced, err := stateAtURIs(".sequoia-state.json")
	if err != nil {
		return err
	}
	did, err := publicationDID("sequoia.json")
	if err != nil {
		return err
	}
	pds, err := resolvePDS(did)
	if err != nil {
		return err
	}
	records, err := listDocuments(pds, did)
	if err != nil {
		return err
	}
	orphans := orphanDocuments(records, referenced)
	if len(orphans) == 0 {
		fmt.Println("No orphaned Standard.site documents.")
		return nil
	}
	token, err := createSession(pds, identifier, password)
	if err != nil {
		return err
	}
	for _, doc := range orphans {
		rkey := doc.URI[strings.LastIndex(doc.URI, "/")+1:]
		err := httpJSON("POST", pds+"/xrpc/com.atproto.repo.deleteRecord", map[string]string{
			"repo":       did,
			"collection": "site.standard.document",
			"rkey":       rkey,
		}, token, nil)
		if err != nil {
			return err
		}
		fmt.Println("Deleted orphaned document:", doc.URI)
	}
	return nil
}

func orphanDocuments(records []document, referenced map[string]bool) []document {
	byPath := map[string]bool{}
	for _, r := range records {
		if r.Path != "" && referenced[r.URI] {
			byPath[r.Path] = true
		}
	}
	var orphans []document
	for _, r := range records {
		if r.Path == "" || referenced[r.URI] {
			continue
		}
		if byPath[r.Path] {
			orphans = append(orphans, r)
		}
	}
	return orphans
}

func stateAtURIs(statePath string) (map[string]bool, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var state struct {
		Posts map[string]struct {
			AtURI string `json:"atUri"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	referenced := map[string]bool{}
	for _, post := range state.Posts {
		if post.AtURI != "" {
			referenced[post.AtURI] = true
		}
	}
	return referenced, nil
}

func publicationDID(configPath string) (string, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	var config struct {
		PublicationURI string `json:"publicationUri"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", err
	}
	rest, ok := strings.CutPrefix(config.PublicationURI, "at://")
	if !ok {
		return "", fmt.Errorf("unexpected publicationUri: %q", config.PublicationURI)
	}
	did, _, _ := strings.Cut(rest, "/")
	if !strings.HasPrefix(did, "did:") {
		return "", fmt.Errorf("unexpected publication DID: %q", did)
	}
	return did, nil
}

func resolvePDS(did string) (string, error) {
	var doc struct {
		Service []struct {
			ID              string `json:"id"`
			ServiceEndpoint string `json:"serviceEndpoint"`
		} `json:"service"`
	}
	if err := httpJSON("GET", "https://plc.directory/"+did, nil, "", &doc); err != nil {
		return "", err
	}
	for _, svc := range doc.Service {
		if svc.ID == "#atproto_pds" {
			return svc.ServiceEndpoint, nil
		}
	}
	return "", fmt.Errorf("no PDS endpoint for %s", did)
}

func listDocuments(pds, did string) ([]document, error) {
	var docs []document
	cursor := ""
	for {
		listURL := pds + "/xrpc/com.atproto.repo.listRecords?repo=" + url.QueryEscape(did) +
			"&collection=site.standard.document&limit=100"
		if cursor != "" {
			listURL += "&cursor=" + url.QueryEscape(cursor)
		}
		var page struct {
			Records []struct {
				URI   string `json:"uri"`
				Value struct {
					Path string `json:"path"`
				} `json:"value"`
			} `json:"records"`
			Cursor string `json:"cursor"`
		}
		if err := httpJSON("GET", listURL, nil, "", &page); err != nil {
			return nil, err
		}
		for _, r := range page.Records {
			docs = append(docs, document{URI: r.URI, Path: r.Value.Path})
		}
		if page.Cursor == "" || len(page.Records) == 0 {
			return docs, nil
		}
		cursor = page.Cursor
	}
}

func createSession(pds, identifier, password string) (string, error) {
	var session struct {
		AccessJwt string `json:"accessJwt"`
	}
	err := httpJSON("POST", pds+"/xrpc/com.atproto.server.createSession", map[string]string{
		"identifier": identifier,
		"password":   password,
	}, "", &session)
	if err != nil {
		return "", err
	}
	return session.AccessJwt, nil
}

func httpJSON(method, requestURL string, body any, token string, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s: %s", method, requestURL, resp.Status, truncate(raw, 200))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func truncate(raw []byte, limit int) string {
	if len(raw) > limit {
		return string(raw[:limit]) + "..."
	}
	return string(raw)
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
