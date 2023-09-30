---
title: Dummy load balancer in a single Go script
date: 2023-08-30
tags:
    - Go
    - TIL
---

I was curious to see if I could prototype a simple load balancer in a single Go script. Go's
standard library and goroutines make this trivial. Here's what the script needs to do:

* Spin up two backend servers that'll handle the incoming requests.
* Run a reverse proxy load balancer in the foreground.
* The load balancer will accept client connections and round-robin them to one of the
backend servers; balancing the inbound load.
* Once a backend responds, the load balancer will relay the response back to the client.
* For simplicity, we'll only handle client's GET requests.

Obviously, this won't have SSL termination, advanced balancing algorithms, or session
persistence like you'd get with Nginx[^1] or Caddy[^2]. The point is to understand the basic
workflow and show how Go makes it easy to write this sort of stuff.

## Architecture

Here's an ASCII art that demonstrates the grossly simplified end-to-end workflow:

```txt
                +----------------------------------------+
                |          Load balancer (8080)          |
                |  +----------------------------------+  |
                |  |       Request from client        |  |
                |  +-----------------|----------------+  |
                |                    | Forward request   |
                |                    | to backend        |
                |                    v                   |
                |  +----------------------------------+  |
                |  |        Load balancing            |  |
                |  |  +----------+    +----------+    |  |
                |  |  | Backend  |    | Backend  |    |  |
                |  |  | 8081     |    | 8082     |    |  |
                |  |  +----------+    +----------+    |  |
                |  +-----------------|----------------+  |
                |                    | Distribute load   |
                |                    v                   |
                |  +----------------------------------+  |
                |  |        Backend Servers           |  |
                |  |  +----------+    +----------+    |  |
                |  |  | Response |    | Response |    |  |
                |  |  | body     |    | body     |    |  |
                |  |  +----------+    +----------+    |  |
                |  +----------------------------------+  |
                |                    | Send response     |
                |                    v                   |
                |  +----------------------------------+  |
                |  |    Client receives response      |  |
                |  +----------------------------------+  |
                +----------------------------------------+
```


The diagram shows a load balancer receiving client requests on port 8080. It distributes the
requests between the backends, sending each request either to a backend running on port 8081
or 8082. The selected backend processes the incoming request and returns a response through
the balancer. The balancer then routes the backend's response back to the client.

## Tools we'll need

Here are the stdlib tools we'll be using. Everything will live in the `main.go` script:

```go
// main.go
package main

import ("fmt"; "io"; "net/http"; "sync")
```

## A few global variables

```go
// main.go
// ... truncated previous sections

var (
    backends = []string{
        "http://localhost:8081/b8081",
        "http://localhost:8082/b8082",
    }
    currentBackend int
    backendMutex  sync.Mutex
)
```

The `backends` slice declares a list of backend server URLs that will be load-balanced
between.

The `currentBackend` integer variable keeps track of the index of the backend server that
handled the most recent request. This will be used later to perform the round-robin load
balancing between the backends.

The `backendMutex` lock provides mutually exclusive access to the shared variables. We'll
see how it's used when we write the load balancing algorithm[^3].

## Writing the backend server

The backend is a simple server that'll just write a message to the connected client,
denoting which server is handling the request.

```go
// main.go
// ... truncated previous sections

// Start a backend server on the specified port
func startBackend(port int, wg *sync.WaitGroup) {
    // Signals the lb when a backend is done processing a request
    defer wg.Done()

    http.HandleFunc(fmt.Sprintf("/b%d", port),
        func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from backend server on :%d\n", port)
    })

    addr := fmt.Sprintf(":%d", port)
    fmt.Printf("Backend is listening on :%d \n", port)

    err := http.ListenAndServe(addr, nil)
    if err != nil {
        fmt.Printf("Error for server on :%d; %s\n", port, err)
    }
}
```

The `startBackend` function starts a backend HTTP server listening on a given port. It takes
the port number and a `sync.WaitGroup`. When `startBackend` returns, it calls `Done()` on
the wait group to signal the load balancer that the backend has finished processing a
request. The function then registers a handler that responds with the port number. It starts
listening and serving on the provided port, printing any errors. We'll run this as
goroutines to spin up two backends on ports 8081 and 8082.

## Selecting backend servers in a round-robin fashion

When a request from a client hits the load balancer, it'll need a way to figure out which
backend server to relay the request to. Here's how it does that:

```go
// main.go
// ... truncated previous sections

// Get the next backend server to forward the request to
// in a round-robin fashion. This function is thread-safe
func getNextBackend() string {
    backendMutex.Lock()
    defer backendMutex.Unlock()

    backend := backends[currentBackend]
    currentBackend = (currentBackend + 1) % len(backends)
    return backend
}
```

The `getNextBackend()` function implements round-robin load balancing across the `backends`
slice in a thread-safe manner. It works like this:

* Acquire a lock on `backendMutex` to prevent concurrent access to the shared state.
* Read the index of the current backend server from `currentBackend`.
* Increment `currentBackend` to point to the next backend server. The modulo % operation
wraps around the index to the start when it reaches past the end.
* Release the lock on `backendMutex`.
* Return the URL of the backend at the index we read in step 2.

This allows each request handling goroutine to safely get the next backend server in a
round-robin fashion. The mutex prevents race conditions where two goroutines try to
read/write the shared `currentBackend` and `backends` state at the same time.

The mutex lock synchronizes access to the shared state across concurrent goroutines. This is
necessary because Go's HTTP server handles requests concurrently by default. Without the
mutex, the goroutines could overwrite each other's changes to `currentBackend`, leading to
incorrect load balancing behavior.

## Writing the load-balancing server

The load balancer itself is a server that sits between the backends and the clients. We
can write its handler function as such:

```go
// main.go
// ... truncated previous sections

// Handle incoming requests and forward them to the backend
func loadBalancerHandler(w http.ResponseWriter, r *http.Request) {
    // Pick a backend in round-robin fashion
    backend := getNextBackend()

    // Relay the client's request to the backend
    resp, err := http.Get(backend)
    if err != nil {
        http.Error(w, "Backend Error", http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()

    // Copy the backend response headers and propagate them to the client
    for key, values := range resp.Header {
        for _, value := range values {
            w.Header().Set(key, value)
        }
    }

    // Copy the backend response body and propagate it to the client
    io.Copy(w, resp.Body)
}
```

The `loadBalancerHandler()` function forwards incoming requests from the clients to the
backend servers. First, it calls `getNextBackend()` to retrieve the next backend server. It
then makes an HTTP GET request to that backend using `http.Get()`.

If there are any errors calling the backend, it just returns a 500 error to the client.
Otherwise, it copies the backend's headers and response body into the response writer to
propagate them back to the client.

This allows transparently load balancing each request across the backends in a round-robin
fashion. The client only sees a single load balancer endpoint. Behind the scenes, requests
are distributed to the dynamic backend servers based on round-robin ordering. Copying
headers and response bodies ensures clients get the proper responses from the chosen
backends.

## Wiring them up together

Finally, the `main` function here just starts the backend servers on port 8081-8082 and the
load balancing server on port 8080:

```go
// main.go
// ... truncated previous sections

func main() {
    var wg sync.WaitGroup

    ports := []int{8081, 8082}

    // Starts the backend servers in the background
    for _, port := range ports {
        wg.Add(1)
        go startBackend(port, &wg)
    }

    // Starts the load balancer server in the foreground
    http.HandleFunc("/", loadBalancerHandler)
    fmt.Println("Load balancer is listening on :8080")

    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        fmt.Printf("Error: %s\n", err)
    }
}
```

## Taking it for a spin

You can find the self-contained complete implementation in this gist[^4]. Run the server in
one terminal with:

```sh
go run main.go
```

It'll print the port numbers of the backend and the load-balancing servers:

```txt
Backend is listening on :8082
Backend is listening on :8081
Load balancer is listening on :8080
```

Then from another console, make a few GET requests with `curl`:

```sh
for i in {1..4}; do
  curl http://localhost:8080/
done
```

This prints:

```txt
Hello from backend server on :8081
Hello from backend server on :8082
Hello from backend server on :8081
Hello from backend server on :8082
```

Notice how the client requests are handled by different backends in an interleaving manner.

[^1]: [Nginx](https://www.nginx.com/)
[^2]: [Caddy](https://caddyserver.com/)
[^3]: [Selecting backend server in a round robin fashion](/go/dummy_load_balancer/#selecting-backend-servers-in-a-round-robin-fashion)
[^4]: [Complete implementation](https://gist.github.com/rednafi/4f871286f42177f21a74a0ce038ce725)
