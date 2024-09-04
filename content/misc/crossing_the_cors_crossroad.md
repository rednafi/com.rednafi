---
title: Crossing the CORS crossroad
date: 2024-03-12
tags:
    - Networking
    - Go
---

Every once in a while, I find myself skimming through the MDN docs to jog my memory on how
CORS[^1] works and which HTTP headers are associated with it. This is particularly true when
a frontend app can't talk to a backend service I manage due to a CORS error[^2].

MDN's CORS documentation is excellent but can be a bit verbose for someone just looking for
a way to quickly troubleshoot and fix the issue at hand.

Typically, the CORS issue I encounter boils down to:

-   A backend service that accepts requests only from a list of specified domains.
-   A new frontend service or some other client trying to access it from a different domain
    that's not on the server's allowlist. Consequently, the server rejects it with an HTTP
    4xx error.

Here's a list of some commonly found headers associated with CORS:

**Request headers**

-   `Origin`: indicates the origin of the request
-   `Access-Control-Request-Method`: used in preflight[^3] to specify the method of the
    actual request
-   `Access-Control-Request-Headers`: used in preflight to specify headers that will be used
    in the actual request

**Response headers**

-   `Access-Control-Allow-Origin`: specifies the origins that are allowed to access the
    resource
-   `Access-Control-Allow-Methods`: indicates the methods allowed when accessing the
    resource
-   `Access-Control-Allow-Headers`: specifies the headers that can be included in the actual
    request
-   `Access-Control-Allow-Credentials`: indicates whether or not the response can be exposed
    when the credentials flag is true
-   `Access-Control-Expose-Headers`: specifies the headers that can be exposed as part of
    the response
-   `Access-Control-Max-Age`: indicates how long the results of a preflight request can be
    cached

In most cases, focusing on the `Origin` and `Access-Control-Allow-Origin` headers is enough
to verify whether a service can be reached from a certain domain without running into a CORS
error.

To recreate the canonical CORS issue, here's a simple server written in Go that exposes a
single `POST` endpoint `/hello`. You can make a POST request to it with the
`{"name": "Something"}` payload, and it will echo back with a JSON message.

---

<details>

<summary>Click here ...</summary>

```go
// main.go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
)

// Person struct to parse the input JSON.
type Person struct {
    Name string `json:"name"`
}

// helloNameHandler responds with "Hello {name}".
func helloNameHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
        return
    }

    var p Person
    if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    response := fmt.Sprintf("Hello %s", p.Name)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"message": response})
}

// corsMiddleware adds CORS headers to the response.
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        allowedOrigins := map[string]bool{
            "http://allowed-origin-1.com": true,
            "http://allowed-origin-2.com": true,
        }

        origin := r.Header.Get("Origin")
        if _, ok := allowedOrigins[origin]; ok {
            w.Header().Set("Access-Control-Allow-Origin", origin)
        } else {
            // Optional: Handle not allowed origin, e.g., by returning an error.
            http.Error(w, "Origin not allowed", http.StatusForbidden)
            return
        }

        w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

        // Handle preflight request.
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func main() {
    mux := http.NewServeMux()
    mux.Handle("/hello", corsMiddleware(http.HandlerFunc(helloNameHandler)))

    fmt.Println("Server is running on http://localhost:7676")
    http.ListenAndServe(":7676", mux)
}
```

</details>

---

Here, the server only allows requests from two particular domains:
`http://allowed-origin-1.com` and `http://allowed-origin-2.com`. A client on another host
can make a preflight OPTIONS request to check if the server will permit the subsequent POST
request.

If the client is on a domain that's not on the allowlist, the server will reject the
request.

You can run the server with `go run main.go` and then, from another console, try making a
preflight request without specifying the `Origin` header:

```sh
curl -i -X OPTIONS http://localhost:7676/hello
```

The server will reject this:

```txt
HTTP/1.1 403 Forbidden
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff
Date: Tue, 12 Mar 2024 21:52:26 GMT
Content-Length: 19

Origin not allowed
```

You need to specify one of the domains expected by the server via the `Origin` header as
follows:

```sh
curl -i -X OPTIONS http://localhost:7676/hello \
        -H 'Origin: http://allowed-origin-1.com'
```

This time, the preflight request will succeed:

```txt
HTTP/1.1 200 OK
Access-Control-Allow-Headers: Content-Type
Access-Control-Allow-Methods: POST, OPTIONS
Access-Control-Allow-Origin: http://allowed-origin-1.com
Date: Tue, 12 Mar 2024 21:54:57 GMT
Content-Length: 0
```

Notice that the `Access-Control-Allow-Methods` header also specifies the methods allowed on
this endpoint.

If you make a preflight request with an origin not on the server's allowlist, you will
encounter a 4xx error again.

```sh
curl -i -X OPTIONS http://localhost:7676/hello -H 'Origin: http://notallowed.com'
```

The return message indicates that requests from `http://notallowed.com` are blocked by CORS
control:

```txt
HTTP/1.1 403 Forbidden
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff
Date: Tue, 12 Mar 2024 21:57:06 GMT
Content-Length: 19

Origin not allowed
```

Similarly, making a POST request without sending the expected origin in the header will
result in a 4xx error.

```sh
curl -i -X POST http://localhost:7676/hello --data '{"name": "Foo"}'
```

This returns:

```txt
HTTP/1.1 403 Forbidden
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff
Date: Tue, 12 Mar 2024 21:59:31 GMT
Content-Length: 19

Origin not allowed
```

Like the preflight request, you need to pass the expected origin in the header.

So, if your frontend cannot access the backend and the browser console indicates that CORS
control is blocking the request, you'll likely need to add the new domain to your server's
allowlist. Then make sure that the client is passing the desired origin in the header. In
the case of a browser, this should be automatically handled for you.

Use the preflight request commands to test that the server is only allowing access from the
whitelisted domain while blocking everything else.

[^1]: [CORS - mdn web docs](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)

[^2]: [CORS error](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS/Errors)

[^3]:
    [Preflight request](https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request)
