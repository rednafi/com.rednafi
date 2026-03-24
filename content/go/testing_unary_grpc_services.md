---
title: Testing unary gRPC services in Go
date: 2026-03-23
slug: testing-unary-grpc-services
tags:
    - Go
    - gRPC
    - Testing
    - Distributed Systems
description: >-
    How to test unary gRPC services in Go - handler logic, interceptors, deadlines,
    metadata propagation, and rich error details - all in-memory with bufconn.
---

> We don't want to test gRPC or an HTTP server itself, we simply want to test our method's
> logic. The simple answer to this question is to de-couple gRPC's work from the actual
> work.
>
> -- John Doak, [Testing gRPC methods]

That advice is right most of the time. If your handler is a thin shell over business logic
that lives behind an interface, you can test the logic without gRPC at all. [Inject a fake],
call the method, check the result.

But sometimes you do need to test the gRPC layer. Maybe you want to verify that status codes
survive the round trip through serialization and HTTP/2 trailers. Maybe you have interceptors
that add logging or auth, deadlines that need to propagate as `grpc-timeout` headers,
metadata that carries trace IDs between services, or structured error details attached via
`status.WithDetails`. In those cases, you need the real gRPC stack running.

That's what [bufconn] does. It's an in-memory [net.Listener] from the gRPC-Go library that
lets you start a real gRPC server and connect a real client to it, all inside the test
process. The gRPC code paths are the same as production, but the underlying connection is an
in-memory pipe instead of a TCP socket.

This post walks through testing a unary gRPC service at two levels: calling the handler
directly without any transport, and using bufconn for in-memory integration tests that
exercise the full stack - including interceptors, deadlines, metadata, and rich error
details. Streaming RPCs have different patterns and are out of scope here.

I'll use a small BookStore service as the running example.

## The BookStore service

The gRPC service has two RPCs: create a book and get a book by ID.

```proto
// api/bookstore.proto
syntax = "proto3";
package bookpb;

option go_package = ".../testing-grpc-unary-service/api";

service Bookstore {
  rpc CreateBook(CreateBookRequest) returns (CreateBookResponse);
  rpc GetBook(GetBookRequest) returns (GetBookResponse);
}

message CreateBookRequest { string title = 1; string author = 2; }
message CreateBookResponse { int64 id = 1; }
message GetBookRequest { int64 id = 1; }
message GetBookResponse { int64 id = 1; string title = 2; string author = 3; }
```

The server struct takes a `Store` interface. It translates between protobuf types and domain
types, validates inputs, and maps errors to gRPC status codes:

```go
// server.go

type Book struct {
    ID     int64
    Title  string
    Author string
}

type Store interface {
    Create(ctx context.Context, title, author string) (int64, error)
    Get(ctx context.Context, id int64) (Book, error)
}

type Server struct {
    api.UnimplementedBookstoreServer
    store Store
}

func RegisterServer(srv *grpc.Server, store Store) {
    api.RegisterBookstoreServer(srv, &Server{store: store})
}
```

`CreateBook` uses `status.WithDetails` to attach structured field violations to validation
errors, so clients can programmatically inspect which fields failed:

```go
// server.go

func (s *Server) CreateBook(
    ctx context.Context, req *api.CreateBookRequest,
) (*api.CreateBookResponse, error) {
    var violations []*errdetails.BadRequest_FieldViolation
    if req.Title == "" {
        violations = append(violations,
            &errdetails.BadRequest_FieldViolation{
                Field:       "title",
                Description: "title is required",
            })
    }
    // ... same for author
    if len(violations) > 0 {
        st := status.New(codes.InvalidArgument,        // (1)
            "invalid book request")
        st, err := st.WithDetails(                     // (2)
            &errdetails.BadRequest{
                FieldViolations: violations,
            })
        if err != nil {
            return nil, status.Errorf(
                codes.Internal, "attaching details: %v", err)
        }
        return nil, st.Err()
    }

    id, err := s.store.Create(ctx, req.Title, req.Author)
    if err != nil {
        return nil, status.Errorf(
            codes.Internal, "creating book: %v", err) // (3)
    }
    return &api.CreateBookResponse{Id: id}, nil
}
```

- (1) creates a status with `InvalidArgument` - same code as a plain `status.Error`, so
  existing tests that check `codes.InvalidArgument` still pass
- (2) `WithDetails` attaches a `BadRequest` proto to the status. The details serialize into
  trailing metadata during transport and deserialize on the client via `status.Details()` -
  we'll [test that round trip](#testing-rich-error-details) later
- (3) wraps store errors as `Internal`

`GetBook` is simpler - it maps a missing book to a `NotFound` status:

```go
// server.go

func (s *Server) GetBook(
    ctx context.Context, req *api.GetBookRequest,
) (*api.GetBookResponse, error) {
    book, err := s.store.Get(ctx, req.Id)
    if err != nil {
        return nil, status.Errorf(
            codes.NotFound, "book %d not found", req.Id)
    }
    return &api.GetBookResponse{
        Id:     book.ID,
        Title:  book.Title,
        Author: book.Author,
    }, nil
}
```

## Testing the handler directly

Most of your handler tests should look like this: create a `Server` with a fake store and
call the handler methods as regular Go functions, without starting a gRPC server or opening
a connection.

First, the fake store. It's an in-memory map that satisfies the `Store` interface:

```go
// server_test.go

var _ Store = (*memStore)(nil) // interface guard

type memStore struct {
    mu    sync.Mutex
    books map[int64]Book
    next  int64
}

func (m *memStore) Create(
    _ context.Context, title, author string,
) (int64, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.next++
    m.books[m.next] = Book{
        ID: m.next, Title: title, Author: author,
    }
    return m.next, nil
}

func (m *memStore) Get(
    _ context.Context, id int64,
) (Book, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    b, ok := m.books[id]
    if !ok {
        return Book{}, fmt.Errorf("book %d not found", id)
    }
    return b, nil
}
```

With that in place, the test creates a `Server` struct directly and calls `CreateBook` and
`GetBook` as plain method calls:

```go
// server_test.go

func TestDirect_CreateAndGetBook(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    srv := &Server{store: store}                  // (1)

    created, err := srv.CreateBook(t.Context(),  // (2)
        &api.CreateBookRequest{
            Title:  "DDIA",
            Author: "Martin Kleppmann",
        })
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }
    if created.Id == 0 {
        t.Fatal("expected non-zero ID")
    }

    got, err := srv.GetBook(t.Context(),
        &api.GetBookRequest{Id: created.Id})
    if err != nil {
        t.Fatalf("GetBook: %v", err)
    }
    if got.Title != "DDIA" {
        t.Errorf("title = %q, want DDIA", got.Title)
    }
}
```

Here:

- (1) creates a `Server` with the fake store, no gRPC server involved
- (2) calls the handler as a regular Go method

You can verify error codes the same way:

```go
// server_test.go

func TestDirect_GetBook_NotFound(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    srv := &Server{store: store}

    _, err := srv.GetBook(t.Context(),
        &api.GetBookRequest{Id: 999})
    if err == nil {
        t.Fatal("expected error")
    }
    s, ok := status.FromError(err)
    if !ok {
        t.Fatalf("expected gRPC status error, got %v", err)
    }
    if s.Code() != codes.NotFound {
        t.Errorf("code = %v, want NotFound", s.Code())
    }
}
```

This works because the handler returns `status.Error(codes.NotFound, ...)`, and
`status.FromError` can parse that even without gRPC transport involved.

These tests are fast and cover the handler's logic: validation, store delegation, error
mapping. For many services, this is enough.

## What direct calls miss

But the handler test has a blind spot. The `status.Error` returned by `GetBook` never
travels through the gRPC transport. In production, that error gets serialized into an HTTP/2
trailer, sent over the wire, and deserialized on the client side. The direct call skips all
of that.

The request and response never go through protobuf serialization, so issues like default
value handling or zero-value round-tripping won't surface. `status.FromError` works on the
original `status.Error` object without it ever being serialized into an HTTP/2 trailer and
reconstructed on the other side.

Server and client interceptors for auth, logging, or retries only fire when a real gRPC call
goes through `grpc.Server`, which direct calls bypass entirely. Deadlines set via
`context.WithTimeout` on the client never propagate as `grpc-timeout` headers, and metadata
attached via `metadata.AppendToOutgoingContext` never reaches the server.

Rich error details attached via `status.WithDetails` travel through trailing metadata during
transport - the direct test never exercises that path. And you're testing the handler in
isolation, not that the generated client and server actually agree on the wire format.

For a service where the handler is a thin adapter and the real logic lives behind the
`Store` interface, none of this matters and direct calls are fine. But if you have
interceptors, need deadline propagation, pass metadata between services, or return structured
error details, you need the real stack.

## Enter bufconn

For HTTP services, Go has [httptest]. `httptest.NewServer` spins up a real HTTP server on a
localhost port. `httptest.NewRecorder` skips the server entirely and calls
`handler.ServeHTTP` as a plain function. bufconn sits between these two: it runs a real gRPC
server and client through the full transport stack (HTTP/2 framing, protobuf serialization,
interceptors), but the underlying connection is an in-memory pipe rather than a TCP socket.

| Approach                  | Real server? | Real transport? | Real TCP? |
| ------------------------- | ------------ | --------------- | --------- |
| Direct handler call       | No           | No              | No        |
| `bufconn`                 | Yes          | Yes             | No        |
| `net.Listen("tcp", ":0")` | Yes          | Yes             | Yes       |

> [!NOTE]
>
> `httptest.NewServer` actually allocates a real TCP port on localhost.
> bufconn doesn't, which means no port conflicts when running tests in parallel in CI.
> If Go's httptest had an option to use `net.Pipe()` instead of TCP
> ([#14200]), bufconn would be the gRPC equivalent of that.

Starting a real gRPC server on `net.Listen("tcp", ":0")` works too. The OS assigns a free
port, so there are no conflicts, but each test pays for TCP setup and teardown. Under heavy
parallelism you can also hit ephemeral port exhaustion. bufconn avoids both while exercising
the same gRPC code paths.

## Setting up bufconn

The test helper starts a gRPC server on a bufconn listener and returns a connected client.
It accepts optional `grpc.ServerOption` values so callers can pass interceptors:

```go
// server_test.go

func startServer(
    t *testing.T, store Store, opts ...grpc.ServerOption, // (1)
) api.BookstoreClient {
    t.Helper()

    lis := bufconn.Listen(1 << 20)                        // (2)
    srv := grpc.NewServer(opts...)                         // (3)
    RegisterServer(srv, store)

    go srv.Serve(lis)                                      // (4)
    t.Cleanup(srv.GracefulStop)                            // (5)

    conn, err := grpc.NewClient("passthrough:///bufconn",  // (6)
        grpc.WithContextDialer(
            func(ctx context.Context, _ string) (net.Conn, error) {
                return lis.DialContext(ctx)                 // (7)
            },
        ),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        t.Fatalf("connecting to bufconn: %v", err)
    }
    t.Cleanup(func() { conn.Close() })

    return api.NewBookstoreClient(conn)
}
```

Walking through each piece:

- (1) `opts ...grpc.ServerOption` lets tests inject interceptors or other server
  configuration. Existing tests that call `startServer(t, store)` are unchanged.
- (2) `bufconn.Listen(1 << 20)` creates an in-memory listener with a 1 MB buffer. This
  replaces `net.Listen("tcp", ":0")`. No port is allocated.
- (3) `grpc.NewServer(opts...)` forwards any server options to the gRPC server.
- (4) `srv.Serve(lis)` starts the gRPC server in a goroutine, same as production but
  listening on the in-memory pipe instead of a socket.
- (5) `t.Cleanup(srv.GracefulStop)` shuts down the server when the test ends. Graceful stop
  waits for in-flight RPCs to finish before closing.
- (6) `"passthrough:///bufconn"` tells gRPC to skip DNS resolution. The actual address
  string doesn't matter because the custom dialer ignores it.
- (7) `WithContextDialer` replaces the default TCP dialer. The custom function ignores the
  address (the `_` parameter) and calls `lis.DialContext`, which returns the client end of
  the bufconn in-memory pipe. The server is already calling `srv.Serve(lis)` on the other
  end, so they're connected through shared memory rather than a network socket.

Every test calls `startServer` with a fresh store, gets back a connected client, and
exercises the full gRPC round trip.

## Server tests with bufconn

The same three scenarios from the direct tests, but now going through the real gRPC
transport.

Create a book, then get it back:

```go
// server_test.go

func TestCreateAndGetBook(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    client := startServer(t, store)                 // (1)

    created, err := client.CreateBook(t.Context(), // (2)
        &api.CreateBookRequest{
            Title:  "DDIA",
            Author: "Martin Kleppmann",
        })
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }
    if created.Id == 0 {
        t.Fatal("expected non-zero ID")
    }

    got, err := client.GetBook(t.Context(),
        &api.GetBookRequest{Id: created.Id})
    if err != nil {
        t.Fatalf("GetBook: %v", err)
    }
    if got.Title != "DDIA" {
        t.Errorf("title = %q, want DDIA", got.Title)
    }
    // ... same for author
}
```

- (1) starts a real gRPC server on a bufconn listener and returns a client connected to it
- (2) `client.CreateBook` is now a real gRPC call that goes through protobuf serialization
  and the HTTP/2 transport, unlike the direct test where it was a plain method call

The error code test looks identical to the direct version, but now the `NotFound` status
travels through the wire as an HTTP/2 trailer instead of staying in-process:

```go
// server_test.go

func TestGetBook_NotFound(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    client := startServer(t, store)

    _, err := client.GetBook(t.Context(),
        &api.GetBookRequest{Id: 999})
    if err == nil {
        t.Fatal("expected error")
    }
    s, ok := status.FromError(err)
    if !ok {
        t.Fatalf("expected gRPC status error, got %v", err)
    }
    if s.Code() != codes.NotFound {
        t.Errorf("code = %v, want NotFound", s.Code())
    }
}
```

The `InvalidArgument` test for empty titles follows the same pattern. If the proto
definitions or the gRPC transport had a bug in status code serialization, these tests would
catch it while the direct tests wouldn't.

## Testing interceptors

Interceptors are middleware for gRPC. A unary server interceptor wraps every RPC call -
common uses include logging, authentication, and request tagging. Here's one that generates a
request ID and sets it as response header metadata:

```go
// server.go

func RequestIDInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req any,
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (any, error) {
        id := fmt.Sprintf("%d", time.Now().UnixNano()) // (1)
        grpc.SetHeader(ctx, metadata.Pairs(             // (2)
            "x-request-id", id,
        ))
        return handler(ctx, req)                        // (3)
    }
}
```

- (1) generates a simple request ID from the current timestamp. In production you'd use a
  UUID library, but `UnixNano` avoids an external dependency for this example
- (2) `grpc.SetHeader` attaches the ID as response header metadata. The client can retrieve
  it with the `grpc.Header` call option
- (3) calls the next handler in the chain

The test passes the interceptor as a server option and verifies the response carries the
header:

```go
// server_test.go

func TestRequestIDInterceptor(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    client := startServer(t, store,
        grpc.UnaryInterceptor(RequestIDInterceptor()), // (1)
    )

    var header metadata.MD                             // (2)
    _, err := client.CreateBook(t.Context(),
        &api.CreateBookRequest{
            Title: "DDIA", Author: "Martin Kleppmann",
        },
        grpc.Header(&header),                          // (3)
    )
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }

    ids := header.Get("x-request-id")                  // (4)
    if len(ids) == 0 {
        t.Fatal("expected x-request-id in response headers")
    }
    if ids[0] == "" {
        t.Fatal("x-request-id is empty")
    }
}
```

- (1) passes the interceptor to `startServer` via the variadic `opts` parameter. This is why
  `startServer` accepts `...grpc.ServerOption` - interceptor tests can inject middleware
  without changing the helper's signature
- (2) declares a `metadata.MD` to capture response headers
- (3) `grpc.Header(&header)` is a call option that tells the gRPC client to populate
  `header` with the server's response headers after the call completes
- (4) verifies the interceptor set the `x-request-id` header

This test can't work with direct handler calls. Interceptors only fire when a request goes
through `grpc.Server`, which means you need the real transport stack - exactly what bufconn
provides.

## Testing deadlines

gRPC propagates deadlines automatically. When the client sets a timeout via
`context.WithTimeout`, gRPC encodes it as a `grpc-timeout` header in the request. The server
receives a context whose deadline matches the client's, and if that deadline fires before the
handler returns, the framework returns `codes.DeadlineExceeded` to the client.

To test this, we need a store that's slow enough to trigger the deadline. A `slowStore`
wraps `memStore` and adds a delay to `Get`:

```go
// server_test.go

type slowStore struct {
    *memStore
    delay time.Duration
}

func (s *slowStore) Get(
    ctx context.Context, id int64,
) (Book, error) {
    select {
    case <-time.After(s.delay):  // (1)
        return s.memStore.Get(ctx, id)
    case <-ctx.Done():           // (2)
        return Book{}, ctx.Err()
    }
}
```

- (1) waits for `delay` before delegating to the real store
- (2) returns immediately if the context is canceled or its deadline fires

The test wires up a `slowStore` with a 2-second delay, creates a book (fast, since `Create`
isn't overridden), then calls `GetBook` with a 100ms timeout:

```go
// server_test.go

func TestGetBook_DeadlineExceeded(t *testing.T) {
    base := &memStore{books: make(map[int64]Book)}
    store := &slowStore{memStore: base, delay: 2 * time.Second}
    client := startServer(t, store)

    // CreateBook is fast - slowStore only overrides Get
    created, err := client.CreateBook(t.Context(),
        &api.CreateBookRequest{
            Title: "DDIA", Author: "Martin Kleppmann",
        })
    // ...

    ctx, cancel := context.WithTimeout(
        t.Context(), 100*time.Millisecond)
    defer cancel()

    _, err = client.GetBook(ctx,
        &api.GetBookRequest{Id: created.Id})
    // ... check err != nil
    s, _ := status.FromError(err)
    if s.Code() != codes.DeadlineExceeded {
        t.Errorf("code = %v, want DeadlineExceeded",
            s.Code())
    }
}
```

The `DeadlineExceeded` the client sees comes from gRPC's transport layer, not from the
handler. If the deadline hadn't fired, `slowStore` would eventually return the book, and if
the store returned an error for some other reason, `GetBook` would wrap it as
`codes.NotFound`. The fact that the test gets `DeadlineExceeded` instead of `NotFound` proves
the deadline traveled through the wire: the client encoded it as a `grpc-timeout` header,
the server's context inherited it, and when it fired, the framework short-circuited the
response.

Direct handler calls can't test this. `context.WithTimeout` on a direct call would still
cancel the context and `slowStore` would return `ctx.Err()`, but `GetBook` would wrap that
as `codes.NotFound` since it treats all store errors the same way. You'd never see
`DeadlineExceeded` without the real transport.

## Testing metadata propagation

[Metadata][metadata] in gRPC carries application-defined key-value pairs alongside RPCs.
Services use it to propagate auth tokens, trace IDs, and request correlation IDs between
services. Testing that metadata survives the round trip requires the real transport.

> [!NOTE]
>
> gRPC metadata maps to HTTP/2 HEADERS frames, but not everything travels there. Status
> codes, error messages, and `WithDetails` payloads travel as HTTP/2 trailing HEADERS
> frames after the response body. The [metadata] package handles the application-level
> key-value pairs; the status and error detail machinery uses the trailer channel.

Here's an interceptor that reads `x-request-id` from incoming metadata and echoes it back as
a response header. This is a test helper, not production code - it isolates the metadata
round trip for verification:

```go
// server_test.go

func echoRequestIDInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req any,
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (any, error) {
        md, ok := metadata.FromIncomingContext(ctx) // (1)
        if ok {
            if ids := md.Get("x-request-id"); len(ids) > 0 {
                grpc.SetHeader(ctx, metadata.Pairs( // (2)
                    "x-request-id", ids[0],
                ))
            }
        }
        return handler(ctx, req)
    }
}
```

- (1) `metadata.FromIncomingContext` extracts the metadata that the client attached to the
  request. This is the server-side API for reading incoming metadata
- (2) echoes the `x-request-id` value back as a response header

The test attaches metadata to the outgoing request and verifies it comes back:

```go
// server_test.go

func TestMetadataPropagation(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    client := startServer(t, store,
        grpc.UnaryInterceptor(echoRequestIDInterceptor()),
    )

    ctx := metadata.AppendToOutgoingContext( // (1)
        t.Context(), "x-request-id", "abc-123",
    )

    var header metadata.MD
    _, err := client.CreateBook(ctx,
        &api.CreateBookRequest{
            Title: "DDIA", Author: "Martin Kleppmann",
        },
        grpc.Header(&header),                  // (2)
    )
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }

    ids := header.Get("x-request-id")          // (3)
    if len(ids) == 0 {
        t.Fatal("expected x-request-id in response headers")
    }
    if ids[0] != "abc-123" {
        t.Errorf("x-request-id = %q, want abc-123", ids[0])
    }
}
```

- (1) `metadata.AppendToOutgoingContext` attaches key-value pairs to the context. When the
  gRPC client makes the call, these become request metadata (the gRPC equivalent of HTTP
  request headers)
- (2) captures response headers into `header`
- (3) verifies the server echoed back the exact value

This pattern is how you'd test that auth tokens, trace IDs, or correlation IDs propagate
correctly through your service. The metadata travels through the gRPC transport as HTTP/2
headers - `AppendToOutgoingContext` on the client side, `FromIncomingContext` on the server
side, `SetHeader` back to the client. None of this machinery runs in a direct handler call.

## Testing rich error details

The `CreateBook` handler uses `status.WithDetails` to attach structured field violations to
validation errors. The details are serialized as protobuf messages in trailing metadata
during transport. To verify they survive the round trip, we need the real transport.

The test sends both fields empty to trigger two violations, then digs into the details:

```go
// server_test.go

func TestCreateBook_ValidationDetails(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    client := startServer(t, store)

    _, err := client.CreateBook(t.Context(),
        &api.CreateBookRequest{Title: "", Author: ""})
    // ... check err != nil

    s, _ := status.FromError(err)
    // ... check s.Code() == codes.InvalidArgument

    details := s.Details()                        // (1)
    if len(details) == 0 {
        t.Fatal("expected error details")
    }
    br, ok := details[0].(*errdetails.BadRequest) // (2)
    if !ok {
        t.Fatalf("expected BadRequest, got %T", details[0])
    }

    fields := make(map[string]string)
    for _, v := range br.FieldViolations {
        fields[v.Field] = v.Description
    }
    if fields["title"] != "title is required" {
        t.Errorf("title violation = %q, want %q",
            fields["title"], "title is required")
    }
    // ... same check for "author"
}
```

- (1) `s.Details()` deserializes the protobuf messages that `WithDetails` attached on the
  server side. They traveled through trailing metadata in the HTTP/2 response
- (2) type-asserts the first detail as `*errdetails.BadRequest` from the [errdetails] package

`WithDetails` always marshals each proto message into a `google.protobuf.Any` wrapper, and
`Details()` always unmarshals them back - that happens even in a direct test. What the direct
test skips is the transport-level round trip: the entire `Status` proto (including its details)
gets encoded into the `grpc-status-details-bin` trailing metadata, transmitted over HTTP/2, and
reconstructed on the client side via `status.FromError`. A direct test would pass even if that
wire-level serialization was broken.

## Choosing your testing level

Direct handler calls are the fastest option. You create a `Server` with a fake store and
call methods directly, with no gRPC server or transport involved. This covers handler logic
(validation, error mapping, store delegation). For many services it's all you need.

When you need to verify that status codes survive the round trip, that interceptors fire,
that deadlines propagate as `grpc-timeout` headers, that metadata round-trips correctly, or
that `WithDetails` error information deserializes on the client side, bufconn is the next
step. The request goes through protobuf serialization, HTTP/2 framing, and the interceptor
chain, all in-memory.

Starting a real TCP server with `net.Listen("tcp", ":0")` adds the OS networking layer on
top of that. You'd reach for this to validate TLS/mTLS configuration, test actual network
behavior, or run interop tests against clients in other languages. For most Go-to-Go service
testing, bufconn is enough and avoids the port allocation overhead.

The full working example is on [GitHub].

<!-- references -->
<!-- prettier-ignore-start -->

[Testing gRPC methods]:
    https://medium.com/@johnsiilver/testing-grpc-methods-6a8edad4159d

[Inject a fake]:
    https://rednafi.com/go/mocking-libraries-bleh/

[bufconn]:
    https://pkg.go.dev/google.golang.org/grpc/test/bufconn

[net.Listener]:
    https://pkg.go.dev/net#Listener

[httptest]:
    https://pkg.go.dev/net/http/httptest

[#14200]:
    https://github.com/golang/go/issues/14200

[metadata]:
    https://pkg.go.dev/google.golang.org/grpc/metadata

[errdetails]:
    https://pkg.go.dev/google.golang.org/genproto/googleapis/rpc/errdetails

[GitHub]:
    https://github.com/rednafi/examples/tree/main/testing-grpc-unary-service

<!-- prettier-ignore-end -->
