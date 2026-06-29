package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultPublicBase = "https://blob.rednafi.com"
	defaultBucket     = "blog"
	defaultWrangler   = "npx -y wrangler@4.83.0"
	immutableCache    = "public, max-age=31536000, immutable"
)

var legacyURLPattern = regexp.MustCompile(`https://blob\.rednafi\.com/static/images/[^\s\]\)"<>]+`)

type occurrence struct {
	filePath string
	oldURL   string
}

type migration struct {
	oldURL  string
	oldKey  string
	newURL  string
	newKey  string
	tmpFile string
}

func main() {
	check := flag.Bool("check", false, "fail if image URLs are not canonical")
	migrateLegacy := flag.Bool("migrate-legacy", false, "migrate legacy blob.rednafi.com/static/images URLs")
	purgeOld := flag.Bool("purge-old", false, "delete old R2 objects after successful migration")
	postPath := flag.String("post", "", "content markdown file for a new post image upload")
	filePath := flag.String("file", "", "local image file to upload")
	imageName := flag.String("name", "", "semantic kebab-case image name")
	publicBase := flag.String("public-base", defaultPublicBase, "public R2 custom-domain base URL")
	bucket := flag.String("bucket", defaultBucket, "R2 bucket name")
	wrangler := flag.String("wrangler", defaultWrangler, "wrangler command")
	flag.Parse()

	if *check {
		if err := checkCanonicalMedia(); err != nil {
			fatal(err)
		}
		return
	}
	if *migrateLegacy {
		if err := migrateLegacyURLs(*publicBase, *bucket, *wrangler, *purgeOld); err != nil {
			fatal(err)
		}
		return
	}
	if *postPath != "" || *filePath != "" || *imageName != "" {
		if err := uploadPostImage(*publicBase, *bucket, *wrangler, *postPath, *filePath, *imageName); err != nil {
			fatal(err)
		}
		return
	}
	fatal(fmt.Errorf("pass --check, --migrate-legacy, or --post/--file/--name"))
}

func uploadPostImage(publicBase, bucket, wrangler, postPath, sourcePath, imageName string) error {
	if postPath == "" || sourcePath == "" || imageName == "" {
		return fmt.Errorf("--post, --file, and --name are required for image upload")
	}
	if strings.Contains(imageName, "_") {
		return fmt.Errorf("--name must use kebab-case, not underscores")
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	ext := detectImageExt(raw)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(sourcePath))
	}
	if ext == "" {
		return fmt.Errorf("%s: cannot detect image type", sourcePath)
	}

	tmpdir, err := os.MkdirTemp("", "rednafi-media-upload-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	tmpFile := filepath.Join(tmpdir, canonicalImageName(imageName)+ext)
	if err := os.WriteFile(tmpFile, raw, 0o644); err != nil {
		return err
	}
	if err := optimizeLosslessly(tmpFile, ext); err != nil {
		return err
	}

	optimized, err := os.ReadFile(tmpFile)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(optimized)
	hash := hex.EncodeToString(sum[:])[:12]

	dir, err := canonicalDir(postPath)
	if err != nil {
		return err
	}
	key := path.Join(dir, canonicalImageName(imageName)+"-"+hash+ext)
	if !publicObjectExists(publicBase, key) {
		if err := putR2Object(wrangler, bucket, key, tmpFile, contentType(key)); err != nil {
			return err
		}
	}

	fmt.Println(strings.TrimRight(publicBase, "/") + "/" + key)
	return nil
}

func migrateLegacyURLs(publicBase, bucket, wrangler string, purgeOld bool) error {
	occurrences, err := findLegacyURLs()
	if err != nil {
		return err
	}
	if len(occurrences) == 0 {
		fmt.Println("No legacy R2 image URLs found.")
		return nil
	}

	tmpdir, err := os.MkdirTemp("", "rednafi-media-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	byURL := map[string]occurrence{}
	for _, occ := range occurrences {
		if _, ok := byURL[occ.oldURL]; !ok {
			byURL[occ.oldURL] = occ
		}
	}

	urls := make([]string, 0, len(byURL))
	for oldURL := range byURL {
		urls = append(urls, oldURL)
	}
	slices.Sort(urls)

	var migrations []migration
	for _, oldURL := range urls {
		occ := byURL[oldURL]
		mig, err := prepareMigration(tmpdir, publicBase, bucket, wrangler, occ)
		if err != nil {
			return err
		}
		migrations = append(migrations, mig)
	}

	if err := rewriteReferences(migrations); err != nil {
		return err
	}

	remaining, err := findLegacyURLs()
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("refusing to purge old images; %d legacy URL(s) still remain", len(remaining))
	}

	if purgeOld {
		for _, mig := range migrations {
			if err := deleteR2Object(wrangler, bucket, mig.oldKey); err != nil {
				return err
			}
		}
	}

	fmt.Printf("Migrated %d R2 image object(s)", len(migrations))
	if purgeOld {
		fmt.Print(" and purged old keys")
	}
	fmt.Println(".")
	return nil
}

func prepareMigration(tmpdir, publicBase, bucket, wrangler string, occ occurrence) (migration, error) {
	oldKey, err := objectKey(occ.oldURL, publicBase)
	if err != nil {
		return migration{}, err
	}

	ext := strings.ToLower(path.Ext(oldKey))
	if ext == "" {
		return migration{}, fmt.Errorf("%s: missing image extension", occ.oldURL)
	}

	tmpFile := filepath.Join(tmpdir, strings.ReplaceAll(oldKey, "/", "__"))
	if err := getR2Object(wrangler, bucket, oldKey, tmpFile); err != nil {
		return migration{}, err
	}
	raw, err := os.ReadFile(tmpFile)
	if err != nil {
		return migration{}, err
	}
	if detected := detectImageExt(raw); detected != "" && detected != ext {
		ext = detected
		tmpWithExt := strings.TrimSuffix(tmpFile, path.Ext(tmpFile)) + ext
		if err := os.Rename(tmpFile, tmpWithExt); err != nil {
			return migration{}, err
		}
		tmpFile = tmpWithExt
	}

	if err := optimizeLosslessly(tmpFile, ext); err != nil {
		return migration{}, err
	}

	raw, err = os.ReadFile(tmpFile)
	if err != nil {
		return migration{}, err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])[:12]
	newKey, err := canonicalKey(occ.filePath, oldKey, ext, hash)
	if err != nil {
		return migration{}, err
	}

	if !publicObjectExists(publicBase, newKey) {
		if err := putR2Object(wrangler, bucket, newKey, tmpFile, contentType(newKey)); err != nil {
			return migration{}, err
		}
	}

	return migration{
		oldURL:  occ.oldURL,
		oldKey:  oldKey,
		newURL:  strings.TrimRight(publicBase, "/") + "/" + newKey,
		newKey:  newKey,
		tmpFile: tmpFile,
	}, nil
}

func findLegacyURLs() ([]occurrence, error) {
	var occurrences []occurrence
	roots := []string{"content", "config.yml"}
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			found, err := legacyURLsInFile(root)
			if err != nil {
				return nil, err
			}
			occurrences = append(occurrences, found...)
			continue
		}
		err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(filePath) != ".md" {
				return nil
			}
			found, err := legacyURLsInFile(filePath)
			if err != nil {
				return err
			}
			occurrences = append(occurrences, found...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return occurrences, nil
}

func legacyURLsInFile(filePath string) ([]occurrence, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	matches := legacyURLPattern.FindAllString(string(raw), -1)
	out := make([]occurrence, 0, len(matches))
	for _, match := range matches {
		out = append(out, occurrence{filePath: filePath, oldURL: match})
	}
	return out, nil
}

func rewriteReferences(migrations []migration) error {
	files := map[string]bool{}
	occurrences, err := findLegacyURLs()
	if err != nil {
		return err
	}
	replacements := map[string]string{}
	for _, mig := range migrations {
		replacements[mig.oldURL] = mig.newURL
	}
	for _, occ := range occurrences {
		files[occ.filePath] = true
	}

	paths := make([]string, 0, len(files))
	for filePath := range files {
		paths = append(paths, filePath)
	}
	slices.Sort(paths)

	for _, filePath := range paths {
		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		next := string(rawBytes)
		for oldURL, newURL := range replacements {
			next = strings.ReplaceAll(next, oldURL, newURL)
		}
		if next != string(rawBytes) {
			if err := os.WriteFile(filePath, []byte(next), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func objectKey(rawURL, publicBase string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	base, err := url.Parse(publicBase)
	if err != nil {
		return "", err
	}
	if u.Scheme != base.Scheme || u.Host != base.Host {
		return "", fmt.Errorf("%s: URL is not under %s", rawURL, publicBase)
	}
	return strings.TrimPrefix(u.EscapedPath(), "/"), nil
}

func canonicalKey(filePath, oldKey, ext, hash string) (string, error) {
	dir, err := canonicalDir(filePath)
	if err != nil {
		return "", err
	}

	base := strings.TrimSuffix(path.Base(oldKey), path.Ext(oldKey))
	name := canonicalImageName(base)
	return path.Join(dir, name+"-"+hash+ext), nil
}

func canonicalDir(filePath string) (string, error) {
	rel := filepath.ToSlash(filePath)
	if rel == "config.yml" {
		return "about", nil
	}
	if !strings.HasPrefix(rel, "content/") {
		return "", fmt.Errorf("%s: cannot derive media directory", filePath)
	}

	fm, err := readFrontmatter(filePath)
	if err != nil {
		return "", err
	}
	slug := slugPath(strings.TrimSpace(scalar(fm["slug"])))
	if slug == "" {
		slug = slugPath(strings.TrimSuffix(path.Base(rel), path.Ext(rel)))
	}

	parts := strings.Split(strings.TrimPrefix(rel, "content/"), "/")
	section := slugPath(parts[0])
	if section == "shards" && len(parts) >= 4 {
		return path.Join(section, slugPath(parts[1]), slugPath(parts[2]), slug), nil
	}
	return path.Join(section, slug), nil
}

func readFrontmatter(filePath string) (map[string]*yaml.Node, error) {
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	raw := string(rawBytes)
	if !strings.HasPrefix(raw, "---\n") {
		return map[string]*yaml.Node{}, nil
	}
	idx := strings.Index(raw[len("---\n"):], "\n---\n")
	if idx == -1 {
		return map[string]*yaml.Node{}, nil
	}
	fmRaw := raw[len("---\n") : len("---\n")+idx]

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmRaw), &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return map[string]*yaml.Node{}, nil
	}
	values := map[string]*yaml.Node{}
	node := doc.Content[0]
	for i := 0; i < len(node.Content); i += 2 {
		values[node.Content[i].Value] = node.Content[i+1]
	}
	return values, nil
}

func scalar(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

var imageSequencePattern = regexp.MustCompile(`^img[_-](\d+)(?:[_-]v(\d+))?$`)

func canonicalImageName(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if match := imageSequencePattern.FindStringSubmatch(lower); match != nil {
		n, _ := strconv.Atoi(match[1])
		name := fmt.Sprintf("image-%02d", n)
		if match[2] != "" {
			name += "-v" + match[2]
		}
		return name
	}
	return slugPath(lower)
}

func checkCanonicalMedia() error {
	legacy, err := findLegacyURLs()
	if err != nil {
		return err
	}
	var problems []string
	if len(legacy) > 0 {
		for _, occ := range legacy {
			problems = append(problems, fmt.Sprintf("%s: legacy R2 URL %s", occ.filePath, occ.oldURL))
		}
	}

	var violations []string
	for _, root := range []string{"content", "config.yml"} {
		found, err := nonCanonicalURLs(root)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
	}
	if len(violations) > 0 {
		problems = append(problems, violations...)
	}

	localImages, err := committedStaticImages()
	if err != nil {
		return err
	}
	for _, filePath := range localImages {
		problems = append(problems, filePath+": static post images belong in R2, not the repo")
	}

	if len(problems) > 0 {
		return fmt.Errorf("media URLs are not canonical:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func nonCanonicalURLs(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nonCanonicalURLsInFile(root)
	}

	var violations []string
	err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(filePath) != ".md" {
			return nil
		}
		found, err := nonCanonicalURLsInFile(filePath)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

func nonCanonicalURLsInFile(filePath string) ([]string, error) {
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, rawURL := range blobURLs(string(rawBytes)) {
		u, err := url.Parse(rawURL)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: invalid R2 URL %s", filePath, rawURL))
			continue
		}
		if strings.Contains(u.EscapedPath(), "_") {
			violations = append(violations, fmt.Sprintf("%s: underscore in R2 path %s", filePath, rawURL))
		}
	}
	if strings.Contains(string(rawBytes), "github.com/rednafi/com.rednafi/releases/download/post-cards") {
		violations = append(violations, filePath+": generated cards must use R2, not GitHub Releases")
	}
	return violations, nil
}

func committedStaticImages() ([]string, error) {
	var paths []string
	for _, root := range []string{"static/images", "assets/images"} {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(filePath)) {
			case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
				paths = append(paths, filepath.ToSlash(filePath))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

var blobURLPattern = regexp.MustCompile(`https://blob\.rednafi\.com/[^\s\]\)"<>]+`)

func blobURLs(raw string) []string {
	return blobURLPattern.FindAllString(raw, -1)
}

func optimizeLosslessly(filePath, ext string) error {
	switch ext {
	case ".png":
		return run("oxipng", "-o", "max", "--strip", "safe", filePath)
	case ".jpg", ".jpeg":
		tmp := filePath + ".jpegtran"
		if err := run("jpegtran", "-copy", "none", "-optimize", "-progressive", "-outfile", tmp, filePath); err != nil {
			return err
		}
		return os.Rename(tmp, filePath)
	default:
		return nil
	}
}

func getR2Object(wrangler, bucket, key, outPath string) error {
	return runShell(wrangler + " r2 object get " + shellQuote(bucket+"/"+key) + " --file " + shellQuote(outPath) + " --remote")
}

func putR2Object(wrangler, bucket, key, filePath, ct string) error {
	return runShell(wrangler +
		" r2 object put " + shellQuote(bucket+"/"+key) +
		" --file " + shellQuote(filePath) +
		" --content-type " + shellQuote(ct) +
		" --cache-control " + shellQuote(immutableCache) +
		" --remote")
}

func deleteR2Object(wrangler, bucket, key string) error {
	return runShell(wrangler + " r2 object delete " + shellQuote(bucket+"/"+key) + " --remote --force")
}

func contentType(key string) string {
	ext := strings.ToLower(path.Ext(key))
	switch ext {
	case ".svg":
		return "image/svg+xml; charset=utf-8"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}

func publicObjectExists(publicBase, key string) bool {
	req, err := http.NewRequest(http.MethodHead, strings.TrimRight(publicBase, "/")+"/"+key, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func detectImageExt(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	switch http.DetectContentType(raw) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ""
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runShell(command string) error {
	cmd := exec.Command("bash", "-lc", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

var slugPartPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugPath(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		part = strings.ReplaceAll(part, "_", "-")
		part = slugPartPattern.ReplaceAllString(part, "-")
		part = strings.Trim(part, "-")
		if part == "" {
			part = "image"
		}
		parts[i] = part
	}
	return strings.Join(parts, "/")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "media:", err)
	os.Exit(1)
}
