package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net/http"
    "strings"
)

// calculateETag generates a weak ETag by SHA-256-hashing the content
// and prefixing it with W/ to indicate a weak comparison
func calculateETag(content string) string {
    hasher := sha256.New()
    hasher.Write([]byte(content))
    hash := hex.EncodeToString(hasher.Sum(nil))
    return fmt.Sprintf("W/\"%s\"", hash)
}

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Define the content within the handler
        content := `{"message": "Hello, world!"}`
        eTag := calculateETag(content)

        // Remove quotes and W/ prefix for If-None-Match header comparison
        ifNoneMatch := strings.TrimPrefix(
			strings.Trim(r.Header.Get("If-None-Match"), "\""), "W/")

        // Generate a hash of the content without the W/ prefix for comparison
        contentHash := strings.TrimPrefix(eTag, "W/")

        // Check if the ETag matches; if so, return 304 Not Modified
        if ifNoneMatch == strings.Trim(contentHash, "\"") {
            w.WriteHeader(http.StatusNotModified)
            return
        }

        // If ETag does not match, return the content and the ETag
        w.Header().Set("ETag", eTag) // Send weak ETag
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, content)
    })

    fmt.Println("Server is running on http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
