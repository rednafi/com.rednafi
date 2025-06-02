---
title: Stacked middleware vs embedded delegation in Go
date: 2025-03-06
tags:
  - Go
  - API
---

Middleware is usually the go-to pattern in Go HTTP servers for tweaking request behavior.
Typically, you wrap your base handler with layers of middleware—one might log every request,
while another intercepts specific routes like `/special` to serve a custom response.

However, I often find the indirections introduced by this pattern a bit hard to read and
debug. I recently came across the embedded delegation pattern while browsing the [Gin repo].
Here, I explore both patterns and explain why I usually start with delegation whenever I
need to modify HTTP requests in my Go services.

## Middleware stacking

Here's an example where the logging middleware records each request, and the special
middleware intercepts requests to `/special`:

```go
package main

import (
    "log"
    "net/http"
)

// loggingMiddleware logs incoming requests.
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("Middleware: received request for", r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

// specialMiddleware intercepts requests for "/special" and handles them.
func specialMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/special" {
            w.Write([]byte("Special middleware handling request"))
            return
        }
        next.ServeHTTP(w, r)
    })
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, world!"))
    })

    // The middleware chain applies special handling then logs every request.
    handler := loggingMiddleware(specialMiddleware(mux))
    http.ListenAndServe(":8080", handler)
}
```

In this setup, every incoming request is first handled by the special middleware, which
checks for the `/special` route, and then by the logging middleware that logs the request
details. We're effectively stacking the middleware functions.

If you hit the server with:

```sh
curl localhost:8080/
curl localhost:8080/special
```

the server logs will look like this:

```txt
2025/03/06 21:24:44 Middleware: received request for /
2025/03/06 21:24:47 Middleware: received request for /special
```

Stacking middleware functions like `middleware3(middleware2(middleware1(mux)))` can get
messy when you have many of them. That's why people usually write a wrapper function to
apply the middlewares to the mux:

```go
func applyMiddleware(
    handler http.Handler,
    middlewares ...func(http.Handler) http.Handler) http.Handler {

    // Apply middlewares in reverse order to preserve LIFO.
    for i := len(middlewares) - 1; i >= 0; i-- {
        handler = middlewares[i](handler)
    }
    return handler
}
```

`applyMiddleware` takes an `http.Handler` and a variadic list of middleware functions
(`...func(http.Handler) http.Handler`). It loops over the middleware in reverse order so
each one wraps the next properly. This avoids deep nesting like
`middleware3(middleware2(middleware1(mux)))` and keeps the middleware chain tidy.

You'd then use it like this:

```go
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, world!"))
    })

    // The middleware chain applies special handling then logs every request.
    // specialMiddleware is applied before loggingMiddleware, just like before.
    handler := applyMiddleware(mux, loggingMiddleware, specialMiddleware)
    http.ListenAndServe(":8080", handler)
}
```

This behaves just like the manual middleware stacking, but it's a bit cleaner.

While this is the canonical way to handle request-response modifications in Go, it can
sometimes be hard to reason about, especially when debugging or dealing with many middleware
layers.

There's another way to achieve the same result without dealing with a soup of nested
functions. The next section talks about that.

## Embedded delegation

Embedded delegation (or the delegation pattern) means you embed the standard HTTP
multiplexer inside your own struct and override its `ServeHTTP` method.

It's a bit like inheritance—overriding a method in a subclass to add extra functionality and
then delegating the call to the original method. Although Go doesn't have a class hierarchy,
you can still delegate responsibilities to the embedded type's method.

The following example implements the same behavior—logging every request and intercepting
the `/special` route—directly within a custom mux:

```go
package main

import (
    "log"
    "net/http"
)

// CustomMux embeds http.ServeMux to override ServeHTTP.
type CustomMux struct {
    *http.ServeMux
}

// ServeHTTP logs the request and intercepts "/special" before
// delegating to the embedded mux.
func (cm *CustomMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Log all requests.
    log.Println("CustomMux: received request for", r.URL.Path)

    // Handle "/special" differently.
    if r.URL.Path == "/special" {
        w.Write([]byte("Special handling in CustomMux"))
        return
    }
    cm.ServeMux.ServeHTTP(w, r)
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, world!"))
    })

    // Wrap the standard mux with our custom delegation.
    customMux := &CustomMux{ServeMux: mux}
    http.ListenAndServe(":8080", customMux)
}
```

In this example, the custom mux centralizes both logging and special-case route handling
within one `ServeHTTP` method. This approach cuts out the extra function calls in a
middleware chain and can simplify tracking the request flow. I find it a bit easier on the
eyes too.

If you have a bunch of extra functionality to add inside `cm.ServeHTTP`, you can wrap them
in utility functions like this:

```go
// logRequest logs incoming HTTP requests.
func logRequest(r *http.Request) {
    log.Println("CustomMux: received request for", r.URL.Path)
}

// handleSpecialRequest handles requests to "/special"
// and returns true if handled.
func handleSpecialRequest(w http.ResponseWriter, r *http.Request) bool {
    if r.URL.Path != "/special" {
        return false // Not handled, continue processing.
    }
    w.Write([]byte("Special handling in CustomMux"))
    return true // Handled; no further processing needed.
}
```

Then, simply call these functions inside your `cm.ServeHTTP` method:

```go
func (cm *CustomMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    logRequest(r)

    if handleSpecialRequest(w, r) {
        return
    }
    cm.ServeMux.ServeHTTP(w, r)
}
```

This keeps all the request modifications in a single `ServeHTTP` method.

## Mixing the two approaches

You can also mix both techniques. For example, you might use direct delegation for special
route handling and then wrap the resulting handler with middleware for logging. Here's how a
hybrid solution might look:

```go
package main

import (
    "log"
    "net/http"
)

// CustomMux embeds http.ServeMux and intercepts "/special".
type CustomMux struct {
    *http.ServeMux
}

// ServeHTTP intercepts "/special" and delegates other routes.
func (cm *CustomMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/special" {
        w.Write([]byte("Special handling in CustomMux"))
        return
    }
    cm.ServeMux.ServeHTTP(w, r)
}

// loggingMiddleware logs incoming requests.
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("Middleware: received request for", r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, world!"))
    })

    // Use direct delegation for special routing.
    customMux := &CustomMux{ServeMux: mux}
    // Wrap the custom mux with logging middleware.
    handler := loggingMiddleware(customMux)
    http.ListenAndServe(":8080", handler)
}
```

In this hybrid approach, the specialized behavior (intercepting the `/special` path) is
handled via direct delegation, while logging stays modular as middleware. This gives you the
best of both worlds.

I usually start with the embedded delegation and gradually introduce the middleware pattern
if I need it later. It's easier to adopt the middleware pattern if you start with delegation
than the other way around.

<!-- Resources -->
<!-- prettier-ignore-start -->

[gin repo]:
    https://github.com/gin-gonic/gin/blob/3b28645dc95d58e0df36b8aff7a6c64f7c0ca5e9/gin.go#L94

<!-- prettier-ignore-end -->
