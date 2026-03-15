---
title: Wrapping a gRPC client in Go
date: 2026-03-15
slug: wrap-grpc-client
tags:
    - Go
    - gRPC
    - API
description: >-
    How to wrap a generated gRPC client behind a clean Go API so users never
    have to touch protobuf types or connection management directly.
---

Yesterday I wrote a [shard on exploring the etcd codebase]. One of the things that stood out
was how the [clientv3 package] abstracts out the underlying gRPC machinery.

etcd is a distributed key-value store where the server and client communicate over gRPC. But
if you've only ever used `clientv3` and never peeked into the internals, you wouldn't know
that. You call `client.Put(ctx, "key", "value")` and get back a response. It feels like a
regular Go library. The fact that gRPC and protobuf are involved is an implementation detail
that the client wrapper keeps away from you.

I've been building a few gRPC services at work lately, and I keep running into the same
question: what API do the users of my client library see? The server ships as a binary. The
client ships as a Go package that other teams `go get`. If I hand them the raw generated
gRPC stubs, they have to import my protobuf types, manage gRPC connections, configure TLS,
and parse `codes.NotFound` from `google.golang.org/grpc/status`. That's a lot of protocol
plumbing for someone who just wants to consume my service.

This post walks through wrapping a generated gRPC client behind a higher level Go API,
following the same pattern etcd uses. I'll use a small in-memory KV store as the running
example.

## Layout

```txt
kv/
├── api/
│   ├── kv.proto           # service definition
│   ├── kv.pb.go           # generated message types
│   └── kv_grpc.pb.go      # generated client and server stubs
├── client/
│   └── client.go          # the wrapper (what users import)
├── server/
│   └── main.go            # the server binary
└── go.mod
```

`api/` holds the proto and generated code. `server/` is a binary you deploy. `client/` is
the library you ship. Other teams add it to their `go.mod` and never touch proto types
directly.

## The proto and generated code

The KV store has three RPCs: put, get, and delete.

```proto
// api/kv.proto
syntax = "proto3";
package kvpb;
option go_package = "example.com/kv/api";

service KV {
  rpc Put(PutRequest) returns (PutResponse);
  rpc Get(GetRequest) returns (GetResponse);
  rpc Delete(DeleteRequest) returns (DeleteResponse);
}

message PutRequest    { string key = 1; bytes value = 2; }
message PutResponse   {}
message GetRequest    { string key = 1; }
message GetResponse   { bytes value = 1; optional bool found = 2; }
message DeleteRequest { string key = 1; }
message DeleteResponse {}
```

`GetResponse` uses `optional bool found` because proto3 normally can't distinguish "field is
zero" from "field was never set." The `optional` keyword generates a pointer in Go, which
lets callers tell a missing key apart from an empty value.

Running `protoc` on this generates a client interface and a server stub. The client side
looks like this:

```go
// api/kv_grpc.pb.go (generated)
type KVClient interface {
    Put(ctx context.Context, in *PutRequest,
        opts ...grpc.CallOption) (*PutResponse, error)
    // Get, Delete have the same shape
}
```

Every method takes a `context.Context`, a protobuf request struct, and variadic
`grpc.CallOption`s, and returns a protobuf response plus an error. Anyone calling the service
has to import protobuf types, construct request structs like `&api.PutRequest{}`, and
understand gRPC call options, even for a simple "get this key" call.

The server implements the other side with an in-memory map. What we care about for the
wrapper is that it returns a gRPC `NOT_FOUND` status when a key doesn't exist. The wrapper
translates that into a Go sentinel error:

```go
// server/main.go
type server struct {
    kvpb.UnimplementedKVServer
    data map[string][]byte
}

func (s *server) Get(
    ctx context.Context, r *kvpb.GetRequest,
) (*kvpb.GetResponse, error) {
    v, ok := s.data[r.Key]
    if !ok {
        return nil, status.Errorf(
            codes.NotFound, "key %q", r.Key)
    }
    return &kvpb.GetResponse{
        Value: v, Found: proto.Bool(true),
    }, nil
}
// Put and Delete follow the same shape.
// Full code is on [GitHub].
```

The server embeds `UnimplementedKVServer`, the standard gRPC pattern. It provides no-op
implementations for all RPCs so the code compiles even before you've written the real logic. The `Get` method checks the map and returns `codes.NotFound` when the key isn't there.
This is the status code the wrapper will catch and turn into a Go error.

## Calling the server without a wrapper

Without a wrapper, callers use the generated `KVClient` directly. Pay attention to the
imports:

```go
// example/main.go (raw usage without wrapper)
import (
    "context"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "example.com/kv/api"
)

// ...
conn, err := grpc.NewClient("localhost:9090",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
// ...
kv := api.NewKVClient(conn)
_, err = kv.Put(ctx, &api.PutRequest{
    Key: "greeting", Value: []byte("hello"),
})
```

Three imports just to put a key. The caller manages the gRPC connection, constructs
`&api.PutRequest{}` structs for every call, and has to parse gRPC status codes to check if a
key exists. For internal code where everyone knows gRPC, this is fine. For a library you ship
to other teams, it's a lot of ceremony.

## Calling the server with the wrapper

Here's the same sequence (put a key, get it back, handle a missing key) using the wrapper
instead:

```go
// example/main.go (with the wrapper)
import "example.com/kv/client"

// ...

c, err := client.New("localhost:9090")
// ...
defer c.Close()

err = c.Put(ctx, "greeting", []byte("hello"))

val, err := c.Get(ctx, "greeting")

_, err = c.Get(ctx, "missing")
if errors.Is(err, client.ErrNotFound) { ... }
```

One import. No gRPC or protobuf packages. `Put` takes a string and a byte slice. `Get`
returns `[]byte`. Missing keys come back as `client.ErrNotFound`, checked with `errors.Is`.
The rest of this post builds the wrapper that makes this work.

## Building the wrapper

The `client/` package is the only thing users import. It hides the generated `api.KVClient`
behind a struct and re-exposes the same operations using plain Go types. The whole wrapper
lives in a single file (`client/client.go`).

First, a sentinel error and a testable interface:

```go
// client/client.go

var ErrNotFound = errors.New("key not found")

type KV interface {
    Put(ctx context.Context, key string, value []byte) error
    Get(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}
```

`ErrNotFound` replaces the gRPC `NOT_FOUND` status code. Callers check it with `errors.Is`
and never import `google.golang.org/grpc/codes`.

`KV` is an interface with the same methods as `Client` but using only standard Go types, no
protobuf or gRPC. Other packages that depend on your client can accept a `KV` instead of a
`*Client`, which means their tests can swap in a simple in-memory fake without spinning up a
gRPC server or importing any gRPC packages.

Next, the struct and constructor:

```go
type Client struct {
    conn *grpc.ClientConn
    kv   api.KVClient
}

func New(addr string, opts ...grpc.DialOption) (*Client, error) {
    if len(opts) == 0 {
        opts = []grpc.DialOption{
            grpc.WithTransportCredentials(insecure.NewCredentials()),
        }
    }
    conn, err := grpc.NewClient(addr, opts...)
    if err != nil {
        return nil, fmt.Errorf("connecting to %s: %v", addr, err)
    }
    return &Client{conn: conn, kv: api.NewKVClient(conn)}, nil
}

func (c *Client) Close() error { return c.conn.Close() }
```

`Client` holds the gRPC connection and the generated `api.KVClient` as unexported fields.
The generated client is not embedded on purpose. If you embed `api.KVClient`, its methods
like `Put(ctx, *PutRequest, ...CallOption)` show up on `Client` directly, and callers can
bypass the wrapper to make raw gRPC calls. Keeping it as a private field means the only way
to talk to the server is through the wrapper methods.

`New` creates the gRPC connection and builds the generated client from it. The variadic
`grpc.DialOption` lets callers pass custom TLS, keepalive, or interceptor config. If they
pass nothing, the default is insecure credentials for local dev. The retries section below
shows what a production setup looks like.

Finally, the wrapper methods. `Get` shows the core pattern:

```go
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
    resp, err := c.kv.Get(ctx, &api.GetRequest{Key: key})
    if err != nil {
        if s, ok := status.FromError(err); ok &&
            s.Code() == codes.NotFound {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf(
            "getting key %s: %v", key, err)
    }
    return resp.Value, nil
}
// Put and Delete follow the same shape.
```

Each wrapper method does the same thing: take the caller's plain Go arguments, build the
protobuf request, call the generated client, and translate the response back into Go types.

The error handling matters. When the server returns `NOT_FOUND`, we convert it to our own
`ErrNotFound` sentinel. For all other errors, we wrap with `%v` instead of `%w`. `%w` would
let callers `errors.As` into gRPC status types, which re-couples them to gRPC internals. I
covered this tradeoff in the [error wrapping post].

## Plugging in retries and metrics

Since the wrapper owns the `grpc.NewClient` call, it can bake in retries and observability
without the caller knowing. gRPC interceptors work like HTTP middleware. They wrap every RPC
with extra logic (logging, retries, metrics) without changing the handler code. You register
them as dial options when creating the connection:

```go
// client/client.go (production version of New)
func New(addr string, opts ...grpc.DialOption) (*Client, error) {
    defaults := []grpc.DialOption{
        grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
        grpc.WithChainUnaryInterceptor(
            grpc_retry.UnaryClientInterceptor(
                grpc_retry.WithMax(3),
                grpc_retry.WithBackoff(
                    grpc_retry.BackoffExponential(100*time.Millisecond),
                ),
            ),
            grpcprom.UnaryClientInterceptor,
        ),
    }
    opts = append(defaults, opts...)
    // ... rest is the same
}
```

[grpc_retry] from [go-grpc-middleware] retries failed RPCs with exponential backoff.
[grpcprom] records latency histograms and error rates. Same `client.New`, same `c.Put`, but
now with retries and metrics baked in. Callers who need to override the defaults can pass
their own dial options. This is useful in tests where you might want insecure credentials or
no retries.

## Try it yourself

The full code is on [GitHub]. Install the server and run the example:

```sh
go install github.com/rednafi/examples/wrapping-grpc-client/server@latest
server &

go install github.com/rednafi/examples/wrapping-grpc-client/example@latest
example
```

Running the example will return:

```
put greeting=hello
get greeting=hello
get missing: not found (expected)
deleted greeting
get greeting after delete: not found (expected)
```

Or add the client library to your own project:

```sh
go get github.com/rednafi/examples/wrapping-grpc-client/client@latest
```


<!-- references -->
<!-- prettier-ignore-start -->

[clientv3 package]:
    https://github.com/etcd-io/etcd/tree/main/client/v3

[error wrapping post]:
    /go/to-wrap-or-not-to-wrap

[GitHub]:
    https://github.com/rednafi/examples/tree/main/wrapping-grpc-client

[go-grpc-middleware]:
    https://github.com/grpc-ecosystem/go-grpc-middleware

[grpcprom]:
    https://pkg.go.dev/github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus

[grpc_retry]:
    https://pkg.go.dev/github.com/grpc-ecosystem/go-grpc-middleware/retry

[shard on exploring the etcd codebase]:
    /shards/2026/03/etcd-codebase

<!-- prettier-ignore-end -->
