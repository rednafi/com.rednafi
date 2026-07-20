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
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultPublicBase = "https://blob.rednafi.com"
	defaultBucket     = "blog"
	defaultWrangler   = "npx -y wrangler@4.112.0"
	immutableCache    = "public, max-age=31536000, immutable"
)

func main() {
	check := flag.Bool("check", false, "fail if image URLs are not canonical")
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
	if *postPath != "" || *filePath != "" || *imageName != "" {
		if err := uploadPostImage(*publicBase, *bucket, *wrangler, *postPath, *filePath, *imageName); err != nil {
			fatal(err)
		}
		return
	}
	fatal(fmt.Errorf("pass --check or --post/--file/--name"))
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

	canonicalName := canonicalImageName(imageName)
	tmpFile := filepath.Join(tmpdir, canonicalName+ext)
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
	key := path.Join(dir, canonicalName+"-"+hash+ext)
	if err := putR2Object(wrangler, bucket, key, tmpFile, contentType(key)); err != nil {
		return err
	}

	fmt.Println(strings.TrimRight(publicBase, "/") + "/" + key)
	return nil
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
	rawSlug := strings.TrimSpace(scalar(fm["slug"]))
	if rawSlug == "" {
		rawSlug = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	}
	slug := slugPath(rawSlug)

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

func canonicalImageName(value string) string {
	return slugPath(value)
}

func checkCanonicalMedia() error {
	var problems []string

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
	default:
		return nil
	}
}

func putR2Object(wrangler, bucket, key, filePath, ct string) error {
	return runShell(wrangler +
		" r2 object put " + shellQuote(bucket+"/"+key) +
		" --file " + shellQuote(filePath) +
		" --content-type " + shellQuote(ct) +
		" --cache-control " + shellQuote(immutableCache) +
		" --remote")
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
