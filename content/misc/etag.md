---
title: ETag and HTTP caching
date: 2024-04-06
tags:
    - API
    - Go
---

One neat use case for the HTTP `ETag` header is client-side HTTP caching. Along with the
`ETag` header, the caching workflow requires you to fiddle with other conditional HTTP
headers like `If-Match` or `If-None-Match`. However, their interaction can feel a bit
confusing at times.

Every time I need to tackle this, I end up spending some time browsing through the relevant
MDN docs[^1][^2][^3] just to jog my memory. At this point, I've done it enough times to
justify spending the time to write this.

## Caching the response of a `GET` endpoint

The basic workflow goes as follows:

-   The client makes a `GET` request to the server.
-   The server responds with a `200 OK` status, including the content requested and an
    `ETag` header.
-   The client caches the response locally, along with the `ETag` value.
-   For subsequent requests to the same resource, the client includes the `If-None-Match`
    header with the `ETag` value it has cached.
-   The server checks if the `ETag` value sent by the client matches the current version of
    the resource.
    -   If they match, the server responds with a `304 Not Modified` status, indicating that
        the client's cached version is still valid and the client serves the resource from
        the cache.
    -   If they don't match, the server responds with a `200 OK status`, including the new
        content and a new `ETag` header, prompting the client to update its cache.

```txt
Client                                 Server
  |                                       |
  |----- GET Request -------------------->|
  |                                       |
  |<---- Response 200 OK + ETag ----------|
  |     (Cache response locally)          |
  |                                       |
  |----- GET Request + If-None-Match ---->|  (If-None-Match == previous Etag)
  |                                       |
  |       Does ETag match?                |
  |<---- Yes: 304 Not Modified -----------|  (No body sent; Use local cache)
  |       No: 200 OK + New ETag ----------|  (Update cached response)
  |                                       |
```

We can test this workflow with GitHub's REST API suite via the GitHub CLI[^4]. If you've
installed the CLI and authenticated yourself, you can make a request like this:

```sh
gh api -i  /users/rednafi
```

This asks for the data associated with the user `rednafi`. The response looks as follows:

```txt
HTTP/2.0 200 OK
Etag: W/"b8fdfabd59aed6e0e602dd140c0a0ff48a665cac791dede458c5109bf4bf9463"

{
  "login":"rednafi",
  "id":30027932,
  ...
}
```

We've truncated the response body and omitted the headers that aren't relevant to our
discussion. You can see that the HTTP status code is `200 OK` and the server has included an
`ETag` header.

The `W/` prefix indicates that a weak validator[^5] is used to validate the content of the
cache. Let's see what happens if we make the same request again while passing the value of
the `ETag` in the `If-None-Match` header.

```sh
gh api -i -H \
    'If-None-Match: W/"b8fdfabd59aed6e0e602dd140c0a0ff48a665cac791dede458c5109bf4bf9463"' \
    /users/rednafi
```

This returns:

```txt
HTTP/2.0 304 Not Modified
Etag: "b8fdfabd59aed6e0e602dd140c0a0ff48a665cac791dede458c5109bf4bf9463"

gh: HTTP 304
```

A few key points to keep in mind:

-   Always wrap your `ETag` values in double quotes when sending them with the
    `If-None-Match` header, just as the spec says[^6].

-   Using the `If-None-Match` header to pass the `ETag` value means that the client request
    is considered successful when the `ETag` value from the client doesn't match that of the
    server. When the values match, the server will return `304 Not Modified` with no body.

-   If we're writing a compliant server, when it comes to `If-None-Match`, the spec tells
    us[^7] to use a weak comparison for ETags. This means that the client will still be able
    to validate the cache with weak ETags, even if there have been slight changes to the
    representation of the data.

-   If the client is a brower, it'll automatically manage the cache and send conditional
    requests without any extra work.

## Writing a server that enables client-side caching

Here's a simple server in Go that enables the above workflow for a JSON `GET` request:

```go
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
```

-   The server calculates a weak `ETag` for its content by creating a SHA-256 hash and
    prefixing it with `W/`, indicating it's suitable for weak comparison. You can make the
    `calculateETag` function even more robust to formatting changes so that the hash doesn't
    change if the format of the JSON changes but the content remains the same. I left that
    out for brevity.

-   When serving content, it includes this weak `ETag` in the response headers, letting
    clients cache the content with the `ETag`.

-   For follow-up requests, it checks if the client sent an `ETag` in the `If-None-Match`
    header and compares it to the current content's `ETag` after removing any formatting.

-   If the `ETag`s match, indicating the content hasn't changed significantly, the server
    replies with a `304 Not Modified` status. Otherwise, it sends the content again with a
    `200 OK` status and updates the `ETag`.

-   This method helps in efficient caching by informing clients when they can reuse cached
    content, reducing unnecessary data transfers and server load.

You can run the server by running `go run main.go` and from a different console start making
request to it like this:

```sh
curl -i  http://localhost:8080/foo
```

This will return the `ETag` header along with the JSON response:

```txt
HTTP/1.1 200 OK
Content-Type: application/json
Etag: W/"1d3b4242cc9039faa663d7ca51a25798e91fbf7675c9007c2b0470b72c2ed2f3"
Date: Wed, 10 Apr 2024 15:54:33 GMT
Content-Length: 28

{"message": "Hello, world!"}
```

Now, you can make another request with the value of the `Etag` in the `If-None-Match`
header:

```sh
curl -i -H \
    'If-None-Match: "1d3b4242cc9039faa663d7ca51a25798e91fbf7675c9007c2b0470b72c2ed2f3"' \
    http://localhost:8080/foo
```

This will return a `304 Not Modified` response with no body:

```txt
HTTP/1.1 304 Not Modified
Date: Wed, 10 Apr 2024 15:57:25 GMT
```

In a real life scenario, you'll probably factor out the caching part in a middleware so that
all of your HTTP GET requests can be cached from the client-side without repetition.

While writing a cache-enabled server, make sure the system is set up so that the server
always sends back the same `ETag` for the same content, even when there are multiple servers
working behind a load balancer. If these servers give out different ETags for the same
content, it can mess up how clients cache that content.

Clients use ETags to decide if content has changed. If the `ETag` value hasn’t changed, they
know the content is the same and don't download it again, saving bandwidth and speeding up
access. But if ETags are inconsistent across servers, clients might download content they
already have, wasting bandwidth and slowing things down.

This inconsistency also means servers end up dealing with more requests for content that
clients could have just used from their cache if ETags were consistent.

[^1]: [Etag - MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/ETag)
[^2]: [If-Match - MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/If-Match)
[^3]:
    [If-None-Match - MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/If-None-Match)

[^4]: [GitHub CLI](https://cli.github.com/)
[^5]:
    [Weak validation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Conditional_requests#weak_validation)

[^6]:
    [Double quote in conditional header values](https://www.rfc-editor.org/rfc/rfc7232#section-3.2)

[^7]:
    [Use weak comparison for Etags while caching](https://www.rfc-editor.org/rfc/rfc7232#section-2.3.2)
