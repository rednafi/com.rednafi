---
title: Deferred teardown closure in Go testing
date: 2025-03-28
tags:
    - Go
    - Testing
---

While watching Mitchell Hashimoto's excellent talk[^1] on Go testing, I came across this neat
technique for deferring teardown to the caller. Let's say you have a helper function in a
test that needs to perform some cleanup afterward.

You can't run the teardown inside the helper itself because the test still needs the setup.
For example, in the following case, the `helper` runs its teardown immediately:

```go
func TestFoo(t *testing.T) {
    helper(t)

    // Test logic here: resources may already be cleaned up!
}

func helper(t *testing.T) {
    t.Helper()

    // Setup code here.

    // Teardown code here.
    defer func() {
    	// Clean up something.
    }()
}
```

When `helper` is called, it defers its teardown—which executes at the end of the helper
function, not the test. But the test logic still depends on whatever the helper set up. So
this approach doesn't work.

The next working option is to move the teardown logic into the test itself:

```go
func TestFoo(t *testing.T) {
    helper(t)

    // Run the teardown of helper.
    defer func() {
    	// Clean up something.
    }()

    // Test logic here.
}

func helper(t *testing.T) {
    t.Helper()

    // Setup code here.

    // No teardown here; we move it to the caller.
}
```

This works fine if you have only one helper. But with multiple helpers, it quickly becomes
messy—you now have to manage multiple teardown calls manually, like this:

```go
func TestFoo(t *testing.T) {
    helper1(t)
    helper2(t)

    defer func() {
    	// Clean up helper2.
    }()

    defer func() {
    	// Clean up helper1.
    }()

    // Test logic here.
}
```

You also need to be careful with the order: `defer` statements are executed in LIFO
(last-in, first-out) order. So if teardown order matters, this can be a problem. Ideally,
your tests shouldn't depend on teardown order—but sometimes they do.

So rather than manually handling cleanup inside the test, have helpers return a teardown
function that the test can `defer` itself. Here's how:

```go
func TestFoo(t *testing.T) {
    teardown1 := helper1(t)
    defer teardown1()

    teardown2 := helper2(t)
    defer teardown2()

    // Test logic here.
}

func helper1(t *testing.T) func() {
    t.Helper()

    // Setup code here.
    // Maybe create a temp dir, start a mock server, etc.

    return func() {
    	// Teardown code here.
    }
}

func helper2(t *testing.T) func() {
    t.Helper()

    // Setup code here.

    return func() {
    	// Teardown code here.
    }
}
```

Each helper is self-contained: it sets something up and returns a function to clean up
whatever resource it has spun up. The test controls when teardown happens by calling the
cleanup function at the appropriate time. Another benefit is that the returned teardown
closure has access to the local variables of the helper. So `func()` can access the helper's
`*testing.T` without us having to pass it explicitly as a parameter.

Here's how I've been using this pattern.

## Creating a temporary file to test file I/O

The `setupTempFile` helper creates a temporary file, writes some content to it, and returns
the file name along with a teardown function that removes the file.

```go
func setupTempFile(t *testing.T, content string) (string, func()) {
    t.Helper()

    tmpFile, err := os.CreateTemp("", "temp-*.txt")
    if err != nil {
    	t.Fatalf("failed to create temp file: %v", err)
    }

    if _, err := tmpFile.WriteString(content); err != nil {
    	t.Fatalf("failed to write to temp file: %v", err)
    }
    tmpFile.Close()

    return tmpFile.Name(), func() {
    	if err := os.Remove(tmpFile.Name()); err != nil {
            t.Errorf("failed to remove temp file %s: %v", tmpFile.Name(), err)
    	} else {
            t.Logf("cleaned up temp file: %s", tmpFile.Name())
    	}
    }
}
```

In the main test:

```go
func TestReadFile(t *testing.T) {
    path, cleanup := setupTempFile(t, "hello world")
    defer cleanup()

    data, err := os.ReadFile(path)
    if err != nil {
    	t.Fatalf("failed to read file: %v", err)
    }

    t.Logf("file contents: %s", data)
}
```

Running the test displays:

```txt
=== RUN   TestReadFile
    prog_test.go:18: file contents: hello world
    prog_test.go:38: cleaned up temp file: /tmp/temp-30176446.txt
--- PASS: TestReadFile (0.00s)
PASS
```

## Starting and stopping a mock HTTP server

Sometimes you want to test code that makes HTTP calls. Here's a helper that starts an
in-memory mock server and returns its URL and a cleanup function that shuts it down:

```go
func setupMockServer(t *testing.T) (string, func()) {
    t.Helper()

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    	w.WriteHeader(http.StatusOK)
    	w.Write([]byte("mock response"))
    })

    server := httptest.NewServer(handler)

    return server.URL, func() {
    	server.Close()
    	t.Log("mock server shut down")
    }
}
```

And in the test:

```go
func TestHTTPRequest(t *testing.T) {
    url, cleanup := setupMockServer(t)
    defer cleanup()

    resp, err := http.Get(url)
    if err != nil {
    	t.Fatalf("failed to make HTTP request: %v", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    t.Logf("response body: %s", body)
}
```

Running the test prints:

```txt
=== RUN   TestHTTPRequest
    prog_test.go:34: response body: mock response
    prog_test.go:20: mock server shut down
--- PASS: TestHTTPRequest (0.00s)
PASS
```

## Setting up and tearing down a database table

In tests that hit a real (or test) database, you often need to create and drop tables.
Here's a helper that sets up a test table and returns a teardown function to drop it:

```go
func setupTestTable(t *testing.T, db *sql.DB) func() {
    t.Helper()

    query := `CREATE TABLE IF NOT EXISTS users (
    	id INTEGER PRIMARY KEY,
    	name TEXT
    )`
    _, err := db.Exec(query)
    if err != nil {
    	t.Fatalf("failed to create table: %v", err)
    }

    return func() {
        _, err := db.Exec(`DROP TABLE IF EXISTS users`)
        if err != nil {
            t.Errorf("failed to drop table: %v", err)
        } else {
            t.Log("dropped test table")
        }
    }
}
```

And the test:

```go
func TestInsertUser(t *testing.T) {
    db := getTestDB(t) // This opens a test DB connection; defined elsewhere.
    cleanup := setupTestTable(t, db)
    defer cleanup()

    _, err := db.Exec(`INSERT INTO users (name) VALUES (?)`, "Alice")
    if err != nil {
    	t.Fatalf("failed to insert user: %v", err)
    }
}
```

Fin!

[^1]: [GopherCon 2017: Advanced testing with Go - Mitchell Hashimoto](https://www.youtube.com/watch?v=8hQG7QlcLBk&t=3s)
