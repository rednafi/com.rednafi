---
title: Hoisting wire plumbing out of your Go handlers
date: 2026-05-02
slug: hoist-wire-plumb
tags:
    - Go
    - API
    - Design Patterns
description: >-
  Four of the five steps in every unary RPC handler are wire plumbing. Pin the
  service function signature and they fit in one generic adapter per transport.
---

Consider an HTTP handler:

```go {hl_lines=["7","12","17","20","27"]}
func handleGreet(w http.ResponseWriter, r *http.Request) {
    var body struct {
        UserID    int64 `json:"user_id"`
        Formality int   `json:"formality"`
    }
    // 1. decode
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    // 2. validate
    if body.UserID == 0 {
        http.Error(w, "user_id required", http.StatusBadRequest)
        return
    }
    // 3. cast
    in := greet.GreetIn{UserID: body.UserID, Formality: body.Formality}

    // 4. call service function
    out, err := svc.Greet(r.Context(), in)
    if err != nil {
        writeErr(w, err)
        return
    }

    // 5. encode
    json.NewEncoder(w).Encode(out)
}
```

And a gRPC handler for the same operation:

```go {hl_lines=["5","9","14","20"]}
func (s *Server) Greet(
    ctx context.Context, req *pb.GreetRequest,
) (*pb.GreetResponse, error) {
    // 1. validate
    if req.GetUserId() == 0 {
        return nil, status.Error(codes.InvalidArgument, "user_id required")
    }
    // 2. cast
    in := greet.GreetIn{
        UserID: req.GetUserId(), Formality: int(req.GetFormality()),
    }

    // 3. call service function
    out, err := s.svc.Greet(ctx, in)
    if err != nil {
        return nil, statusErr(err)
    }

    // 4. encode
    return &pb.GreetResponse{Message: out.Message}, nil
}
```

Both handlers run through the same five steps:

- decode the input off the wire
- validate it
- cast it into a domain type
- call the service function and collect the output
- encode the output and return it

Four of those steps are wire plumbing: decode, validate, cast, encode. Only the call to the
service function does anything domain-specific, and the body of that call is identical in
both handlers above.

The plumbing is per-transport, not per-endpoint. Every HTTP endpoint with a JSON body
decodes the same way, and every gRPC method unpacks its protobuf the same way. What changes
endpoint to endpoint is only the input and output types of the service function.

So instead of writing the same wire plumbing in every handler, hoist it into one adapter per
transport:

> [!Gist]
>
> - Every service function has the shape `func(ctx context.Context, in In) (Out, error)`.
>   `In` and `Out` are domain types. No transport type ever shows up in the signature.
> - For each transport, write a generic adapter `Wrap[In, Out]`. It takes three things: a
>   decode function that turns a wire request into `In`, the service function itself, and an
>   encode function that turns `Out` into a wire response.
> - Inside, `Wrap` decodes the request, runs `Validate()` on `In` if it has one, calls the
>   service function, and encodes the result.
> - `Wrap` returns the transport's handler shape. For HTTP that's `http.Handler`. For gRPC
>   it's the function shape `protoc-gen-go-grpc` generates for server methods, so the
>   wrapped function lives on the `Server` struct and the generated method forwards to it.
> - Adding an endpoint costs one decode, one encode, and one router line per transport. The
>   service function on the inside stays the same no matter which transport is calling it.

The same service-function shape feeds multiple transports:

```text
service function                adapter        transport handler
----------------------------------------------------------------

func(ctx, In) (Out, error) ->   http Wrap ->   http.Handler
func(ctx, In) (Out, error) ->   grpc Wrap ->   gRPC handler
```

A few benefits of doing it this way are:

> [!Important]
>
> - You write the four plumbing steps once per transport. A fix lands inside `Wrap` and
>   applies to every endpoint at once.
> - Every endpoint is the same shape: a service method, a decode, an encode, and a router
>   line. Humans and LLMs pick that up from one example, and off-shape code won't compile.
> - Tests split at two layers: the service function in unit tests with no transport, and
>   `Wrap` plus its codecs once at the transport level. No per-handler plumbing tests.
> - Middleware and interceptors compose unchanged. They sit outside `Wrap`, so auth,
>   observability, and rate limiting go where they always did.
> - Drift goes away. The same domain error returns the same status everywhere, and
>   `Validate` runs on every input that has one.

This pattern isn't new.

[go-kit] had it back in 2015 as `Endpoint`, a pre-generics adapter that every per-transport
wrapper wrapped:

```go
type Endpoint func(
    ctx context.Context, request interface{},
) (response interface{}, err error)
```

[Connect-Go] uses the same shape as `UnaryFunc` over interface types, generating the
wrappers from `.proto` files:

```go
type UnaryFunc func(context.Context, AnyRequest) (AnyResponse, error)
```

Mat Ryer's [How I write HTTP services after 13 years] arrives at the HTTP half from the
other direction, with generic encode/decode helpers and a service that returns
`(Out, error)`.

Below I build a small greeter service from scratch and wrap it once for HTTP and once for
gRPC. The full code is in the [wire-plumb] directory of the examples repo.

## Writing the service function

The service function holds the business logic. No transport types in its signature, so the
same function runs over both HTTP and gRPC.

The greeter takes a user store and a logger, loads the user, logs the call, and writes a
message based on a formality flag:

```go {hl_lines=["11"]}
// greet/service.go
type Service struct {
    users  UserStore
    logger *slog.Logger
}

func NewService(users UserStore, logger *slog.Logger) *Service {
    return &Service{users: users, logger: logger}
}

func (s *Service) Greet(ctx context.Context, in GreetIn) (GreetOut, error) {
    u, err := s.users.GetUser(ctx, in.UserID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return GreetOut{}, NotFound("user %d", in.UserID)
        }
        return GreetOut{}, fmt.Errorf("getting user: %w", err)
    }

    s.logger.Info("greeted", "user_id", u.ID, "formality", in.Formality)

    msg := "hey " + u.Name + "!"
    if in.Formality == 1 {
        msg = "Good day, " + u.Name + "."
    }
    return GreetOut{Message: msg}, nil
}
```

Drop the receiver from the highlighted line and what's left is
`func(ctx context.Context, in GreetIn) (GreetOut, error)`, the `func(ctx, In) (Out, error)`
shape. Every service function in the project will match that line.

You could give the shape a name:

```go
type SvcFunc[In, Out any] func(ctx context.Context, in In) (Out, error)
```

I don't bother. I prefer to repeat `func(ctx context.Context, in In) (Out, error)` literally
in `Wrap`'s definition, so the shape is obvious wherever `Wrap` shows up, and every other
file stays plain non-generic Go.

This file imports neither `net/http`, `google.golang.org/grpc`, JSON, nor protobuf. New
dependencies can land as fields on `Service`, but they stay there. The signature doesn't
change, so the wrappers don't need to know.

`GreetIn` and `GreetOut` are plain Go structs with no transport-specific tags. `In` can
optionally satisfy a `Validate` method, and the wrappers will run it between decode and
call:

```go
// greet/service.go
type GreetIn struct {
    UserID    int64
    Formality int
}

func (in GreetIn) Validate() error {
    if in.UserID == 0 {
        return Invalid("user_id is required")
    }
    if in.Formality < 0 || in.Formality > 1 {
        return Invalid("formality must be 0 or 1")
    }
    return nil
}

type GreetOut struct{ Message string }
```

(`NotFound` and `Invalid` return a domain `*greet.Error` carrying a `Code` enum that maps to
HTTP statuses and gRPC codes. I covered the mapping in [Error translation in Go services].)

With the service function and its types pinned, the only thing left is the adapter that runs
decode, validate, call, and encode around them once per transport.

## Wrapping it for HTTP

`Wrap` is the HTTP adapter. It's a generic function over `[In, Out]` that takes a
per-endpoint `decode(*http.Request) (In, error)`, the service function in the middle, and a
per-endpoint `encode(http.ResponseWriter, Out) error`. It returns an `http.Handler` ready to
mount on a router.

The numbered comments mark the four plumbing steps:

```go {hl_lines=["8","15","20","26"]}
// http/http.go
func Wrap[In, Out any](
    decode func(*http.Request) (In, error),
    fn     func(context.Context, In) (Out, error),
    encode func(http.ResponseWriter, Out) error,
) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        in, err := decode(r) // (1)
        if err != nil {
            writeErr(w, err)
            return
        }

        if v, ok := any(in).(validator); ok {
            if err := v.Validate(); err != nil { // (2)
                writeErr(w, err)
                return
            }
        }
        out, err := fn(r.Context(), in) // (3)
        if err != nil {
            writeErr(w, err)
            return
        }

        if err := encode(w, out); err != nil { // (4)
            log.Printf("encode response: %v", err)
        }
    })
}
```

Structs without `Validate` skip step 2. The encode branch only logs on error: the response
is already partially written by then. Every other branch goes through `writeErr`, which
turns a domain `*greet.Error` into an HTTP status and a JSON body.

That leaves `decodeGreet` and `encodeGreet` per endpoint. The `http.Error` calls, early
returns, and status codes from the original handler are gone. Decode parses the body into
`GreetIn`. Encode writes the message back as JSON, wrapped in an anonymous struct so the
wire field is `message` (lowercase) without putting a JSON tag on the domain type:

```go
// http/http.go
func decodeGreet(r *http.Request) (greet.GreetIn, error) {
    var body struct {
        UserID    int64 `json:"user_id"`
        Formality int   `json:"formality"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        return greet.GreetIn{}, greet.Invalid("malformed json")
    }
    return greet.GreetIn{UserID: body.UserID, Formality: body.Formality}, nil
}

func encodeGreet(w http.ResponseWriter, out greet.GreetOut) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    return json.NewEncoder(w).Encode(struct {
        Message string `json:"message"`
    }{out.Message})
}
```

The `UserID == 0` check sits on `GreetIn.Validate` and runs inside `Wrap` between decode and
call, so it doesn't reappear here. Both functions only do wire-to-domain mapping.

`Register` mounts every endpoint. The highlighted line is the wiring: `Wrap` chews the three
pieces (decode, service function, encode) and hands back an `http.Handler` that the mux can
mount.

```go {hl_lines=["3"]}
// http/http.go
func Register(mux *http.ServeMux, svc *greet.Service) {
    mux.Handle("POST /greet", Wrap(decodeGreet, svc.Greet, encodeGreet))
}
```

`main` ties it together. The highlighted line is the only place `cmd/http` knows about the
HTTP package at all:

```go {hl_lines=["10"]}
// cmd/http/main.go
func main() {
    users := greet.NewMemoryStore(
        greet.User{ID: 1, Name: "red"},
        greet.User{ID: 2, Name: "blue"},
    )
    svc := greet.NewService(users, slog.Default())

    mux := http.NewServeMux()
    ehttp.Register(mux, svc)
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

`svc.Greet` is a method value: the `*Service` receiver gets bound into the function, so the
wrapper sees a plain `func(context.Context, GreetIn) (GreetOut, error)`. The store, the
logger, and anything else on `Service` ride along in the closure and never appear in the
wrapper's signature.

The gRPC version uses that same `svc.Greet`. Only the wrapping around it changes.

## Wrapping it for gRPC

The gRPC adapter has the same job: decode, validate, call, and encode around the service
function. Two things change.

gRPC's wire types are per-RPC. `*pb.GreetRequest` and `*pb.GreetResponse` belong to one
specific method, where `*http.Request` and `http.ResponseWriter` were shared across every
HTTP handler. So the gRPC `Wrap` carries two extra type parameters, `WireIn` and `WireOut`,
sitting on either side of the domain `In` and `Out`.

Errors come back as return values too, not bytes written to a stream, so every error branch
can `return` directly without a `writeErr` helper.

The function-typed arguments line up the same as before, with `WireIn` and `WireOut`
standing in for `*http.Request` and `http.ResponseWriter`: `decode(WireIn) (In, error)`, the
same service function, and `encode(Out) (WireOut, error)`. The wrapper returns
`func(context.Context, WireIn) (WireOut, error)`, which is the signature
`protoc-gen-go-grpc` generates for every server method:

```go {hl_lines=["10","16","20","25"]}
// grpc/grpc.go
func Wrap[WireIn, In, Out, WireOut any](
    decode func(WireIn) (In, error),
    fn     func(context.Context, In) (Out, error),
    encode func(Out) (WireOut, error),
) func(context.Context, WireIn) (WireOut, error) {
    return func(ctx context.Context, wireIn WireIn) (WireOut, error) {
        var zero WireOut

        in, err := decode(wireIn) // (1)
        if err != nil {
            return zero, statusErr(err)
        }

        if v, ok := any(in).(validator); ok {
            if err := v.Validate(); err != nil { // (2)
                return zero, statusErr(err)
            }
        }
        out, err := fn(ctx, in) // (3)
        if err != nil {
            return zero, statusErr(err)
        }

        return encode(out) // (4)
    }
}
```

Same four steps as the HTTP version, with the same `svc.Greet` doing the user lookup and
logging. `statusErr` turns a domain `*greet.Error` into a `*status.Status` carrying the
matching gRPC code.

The gRPC `decodeGreet` and `encodeGreet` are narrower than the HTTP versions because
protobuf has already parsed the bytes by the time `Wrap` sees the request. Decode copies
fields from the generated struct into the domain type. Encode copies them back:

```go
// grpc/grpc.go
func decodeGreet(req *pb.GreetRequest) (greet.GreetIn, error) {
    return greet.GreetIn{
        UserID:    req.GetUserId(),
        Formality: int(req.GetFormality()),
    }, nil
}

func encodeGreet(out greet.GreetOut) (*pb.GreetResponse, error) {
    return &pb.GreetResponse{Message: out.Message}, nil
}
```

Unlike the HTTP `mux.Handle` route, you can't hand the wrapped function straight to gRPC.
`protoc-gen-go-grpc` generates a server interface with concrete method signatures, so
`Server` holds the wrapped functions as fields and the methods forward to them:

```go {hl_lines=["5","10"]}
// grpc/grpc.go
type Server struct {
    pb.UnimplementedGreeterServer

    greet func(context.Context, *pb.GreetRequest) (*pb.GreetResponse, error)
}

func NewServer(svc *greet.Service) *Server {
    return &Server{
        greet: Wrap(decodeGreet, svc.Greet, encodeGreet),
    }
}

func (s *Server) Greet(
    ctx context.Context, req *pb.GreetRequest,
) (*pb.GreetResponse, error) {
    return s.greet(ctx, req)
}
```

The gRPC package's `Register` follows the same shape, and so does `main`. The highlighted
lines are the wiring points:

```go {hl_lines=["3"]}
// grpc/grpc.go
func Register(srv *grpc.Server, svc *greet.Service) {
    pb.RegisterGreeterServer(srv, NewServer(svc))
}
```

```go {hl_lines=["10"]}
// cmd/grpc/main.go
func main() {
    users := greet.NewMemoryStore(
        greet.User{ID: 1, Name: "red"},
        greet.User{ID: 2, Name: "blue"},
    )
    svc := greet.NewService(users, slog.Default())

    srv := grpc.NewServer()
    egrpc.Register(srv, svc)

    lis, _ := net.Listen("tcp", ":9090")
    log.Fatal(srv.Serve(lis))
}
```

Either way, `svc.Greet` runs unchanged. Only the `decode` and `encode` on either side of it
differ between transports.

## Adding a second endpoint

With one endpoint wired through both transports, a second endpoint should be cheap. Adding a
`Farewell` method costs three short pieces of code on the HTTP side and one extra line on
the router. The highlighted lines are the entire diff once `Wrap` exists:

```go {hl_lines=["20","21"]}
// greet/service.go
func (s *Service) Farewell(
    ctx context.Context, in FarewellIn,
) (FarewellOut, error) {
    // ...
}

// http/http.go
func decodeFarewell(r *http.Request) (greet.FarewellIn, error) {
    // ...
}

func encodeFarewell(w http.ResponseWriter, out greet.FarewellOut) error {
    // ...
}

func Register(mux *http.ServeMux, svc *greet.Service) {
    mux.Handle("POST /greet",
        Wrap(decodeGreet, svc.Greet, encodeGreet))
    mux.Handle("POST /farewell",
        Wrap(decodeFarewell, svc.Farewell, encodeFarewell))
}
```

The gRPC side follows the same shape: a `decodeFarewell`/`encodeFarewell` pair between
`*pb.FarewellRequest`/`*pb.FarewellResponse` and the domain types, plus one extra line in
`NewServer`. The plumbing inside `Wrap` doesn't change.

## Middleware and interceptors don't change

Middleware and interceptors don't see `Wrap`. They wrap the `http.Handler` or the gRPC
server method that `Wrap` returned, the same as any other handler. The HTTP signature stays
`func(http.Handler) http.Handler` and the gRPC signature stays
`grpc.UnaryServerInterceptor`.

A request logger as HTTP middleware. The highlighted line is where the middleware hands off
to the wrapped mux:

```go {hl_lines=["5"]}
// cmd/http/main.go
func RequestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s took=%s", r.Method, r.URL.Path, time.Since(start))
    })
}
```

Wired into `main` by wrapping the mux:

```go {hl_lines=["10"]}
// cmd/http/main.go
func main() {
    users := greet.NewMemoryStore(
        greet.User{ID: 1, Name: "red"},
        greet.User{ID: 2, Name: "blue"},
    )
    svc := greet.NewService(users, slog.Default())
    mux := http.NewServeMux()
    ehttp.Register(mux, svc)
    log.Fatal(http.ListenAndServe(":8080", RequestLogger(mux)))
}
```

The gRPC version of the same logger. The highlighted line is the analogous hand-off into the
wrapped server method:

```go {hl_lines=["7"]}
// cmd/grpc/main.go
func LoggingInterceptor(
    ctx context.Context, req any,
    info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
    start := time.Now()
    resp, err := handler(ctx, req)
    log.Printf("%s took=%s err=%v", info.FullMethod, time.Since(start), err)
    return resp, err
}
```

Wired into the server constructor:

```go {hl_lines=["8"]}
// cmd/grpc/main.go
func main() {
    users := greet.NewMemoryStore(
        greet.User{ID: 1, Name: "red"},
        greet.User{ID: 2, Name: "blue"},
    )
    svc := greet.NewService(users, slog.Default())
    srv := grpc.NewServer(grpc.UnaryInterceptor(LoggingInterceptor))
    egrpc.Register(srv, svc)

    lis, _ := net.Listen("tcp", ":9090")
    log.Fatal(srv.Serve(lis))
}
```

<!-- references -->
<!-- prettier-ignore-start -->

[How I write HTTP services after 13 years]:
    https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

[Error translation in Go services]:
    /go/error-translation

[go-kit]:
    https://github.com/go-kit/kit/blob/78fbbceece7bbcf073bee814a7772f4397ea756c/endpoint/endpoint.go#L9

[Connect-Go]:
    https://github.com/connectrpc/connect-go/blob/c4aac92b87026cd709cfbccdaabe8c45abef705c/interceptor.go#L36

[wire-plumb]:
    https://github.com/rednafi/examples/tree/main/wire-plumb

<!-- prettier-ignore-end -->
