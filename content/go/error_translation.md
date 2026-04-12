---
title: Error translation in Go services
date: 2026-04-12
slug: error-translation
tags:
    - Go
    - Error Handling
description: >-
    Translating errors at layer boundaries so storage details don't leak
    into the handler or, worse, into client responses.
---

In a layered Go service, it's easy to accidentally leak storage errors like
`sql.ErrNoRows` all the way up to the handler, or worse, to the client. This
post shows how to catch those at the service boundary, translate them into
domain errors, and keep internal details from reaching places they shouldn't.

## When the handler knows your database

Say you have a user service backed by Postgres. The handler fetches a user by
ID and needs to distinguish "not found" from an actual failure:

```go
// handler.go

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    u, err := h.svc.GetUser(r.Context(), id)
    if errors.Is(err, sql.ErrNoRows) { // (1)
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(u)
}
```

- (1) This is the coupling. The handler imports `database/sql` and checks for
  `sql.ErrNoRows`, a storage-specific error. The handler now knows the service
  uses SQL.

For a small service with one database and one transport, that's a reasonable
tradeoff. You know it's SQL, and nothing else is going to change anytime soon.

Then the service grows. Someone puts Redis in front of Postgres as a
read-through cache, and now there are two different "not found" errors:

```go
// handler.go

if errors.Is(err, sql.ErrNoRows) || errors.Is(err, redis.Nil) {
    http.Error(w, "not found", http.StatusNotFound)
    return
}
```

The handler now imports two storage packages. It knows the service uses both
Postgres and Redis. Then you add soft deletes. A soft-deleted user exists in
both Postgres and Redis, so neither `sql.ErrNoRows` nor `redis.Nil` fires for
it. But the service considers the user gone. The handler has no way to return
404 for this case because neither storage error applies.

Then someone adds a gRPC handler for the same service:

```go
// handler.go

func (h *Handler) GetUser(
    ctx context.Context, req *pb.GetUserRequest,
) (*pb.GetUserResponse, error) {
    u, err := h.svc.GetUser(ctx, req.GetId())
    if errors.Is(err, sql.ErrNoRows) || errors.Is(err, redis.Nil) { // (1)
        return nil, status.Error(codes.NotFound, "not found")
    }
    if err != nil {
        return nil, status.Error(codes.Internal, "internal error")
    }
    return &pb.GetUserResponse{
        Id: u.ID, Name: u.Name, Email: u.Email,
    }, nil
}
```

- (1) The same storage error checks from the HTTP handler, duplicated here.
  The gRPC handler also imports `database/sql` and `redis` and maps the same
  storage errors to a different output format (`codes.NotFound` instead of
  `http.StatusNotFound`).

Now two handlers know about `sql.ErrNoRows` and `redis.Nil`. Adding a third
storage backend or removing Redis means updating both. Every change to storage
ripples into transport code that shouldn't care how data is stored.

The handler shouldn't need to know any of this. It should check for a single
"not found" error and return 404 regardless of whether the cause was a missing
SQL row, a Redis miss, or a soft delete. That means the service needs its own
error types.

## Defining domain errors

When `sql.ErrNoRows` passes through the service and reaches the handler, it
becomes part of the interface between those layers. Swap Postgres for DynamoDB
and the handler breaks, defeating the whole purpose of having a repository
layer in between. The service package can prevent this by defining errors that
describe what went wrong in business terms:

```go
// user/user.go

package user

import (
    "context"
    "errors"
    "time"
)

type User struct {
    ID        int64      `json:"id"`
    Name      string     `json:"name"`
    Email     string     `json:"email"`
    DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

var (
    ErrNotFound = errors.New("not found")
    ErrConflict = errors.New("conflict")
)

type Store interface {
    Get(ctx context.Context, id int64) (User, error)
    Create(ctx context.Context, u User) (int64, error)
}
```

`ErrNotFound` means the user doesn't exist. It doesn't say why. A missing SQL
row, an expired Redis key, and a soft-deleted record all produce the same
error. The handler doesn't need to distinguish between these cases because in
all three, the response is a 404.

`ErrConflict` means a uniqueness constraint would be violated. Whether that's
a SQL UNIQUE index or a DynamoDB conditional check is for the storage package
to worry about.

With these defined, the repository is where the mapping happens: catch
storage-specific errors and return domain errors instead.

## Catching storage errors in the repository

Here's the SQLite implementation of the repository interface. The two error
paths handle things differently on purpose:

```go
// sqlite/store.go

func (s *UserStore) Get(
    ctx context.Context, id int64,
) (user.User, error) {
    row := s.db.QueryRowContext(ctx,
        "SELECT id, name, email FROM users WHERE id = ?", id)

    var u user.User
    if err := row.Scan(&u.ID, &u.Name, &u.Email); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return user.User{}, fmt.Errorf(
                "user %d not in db: %w", id, user.ErrNotFound, // (1)
            )
        }
        return user.User{}, fmt.Errorf(
            "querying user %d: %v", id, err, // (2)
        )
    }
    return u, nil
}
```

The two paths use different format verbs and wrap different things:

- (1) `%w` wraps `user.ErrNotFound` - the domain sentinel, not the original
  `sql.ErrNoRows`. The repository catches `sql.ErrNoRows` in the `if`
  check above, but instead of wrapping it, builds a new error around
  `user.ErrNotFound`. So `errors.Is(err, user.ErrNotFound)` matches, but
  `errors.Is(err, sql.ErrNoRows)` does not because that error was consumed here,
  not wrapped. The message `"user 42 not in db: not found"` still tells
  you what happened during debugging.
- (2) `%v` wraps the raw `err` from `database/sql`. This is a storage error
  that callers shouldn't be able to inspect programmatically. `%v` preserves
  the error message for logging but severs the chain, so
  `errors.Is(err, sql.ErrWhatever)` won't match. If I used `%w` here, callers
  could `errors.Is` through to `database/sql` types and the coupling would
  come back. I wrote more about this choice in
  [Go errors: to wrap or not to wrap?].

The rule is: use `%w` for your own domain errors (callers should inspect
them), `%v` for storage errors (callers shouldn't).

For creates, constraint violations get the same treatment:

```go
// sqlite/store.go

func (s *UserStore) Create(
    ctx context.Context, u user.User,
) (int64, error) {
    res, err := s.db.ExecContext(ctx,
        "INSERT INTO users (name, email) VALUES (?, ?)",
        u.Name, u.Email,
    )
    if err != nil {
        if sqliteErr, ok := errors.AsType[sqlite3.Error](err); ok &&
            sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
            return 0, fmt.Errorf(
                "user %s already exists: %w", // (1)
                u.Email, user.ErrConflict,
            )
        }
        return 0, fmt.Errorf("inserting user: %v", err) // (2)
    }
    return res.LastInsertId()
}
```

- (1) Same pattern as `Get`. A database-specific constraint error becomes
  `user.ErrConflict` wrapped with `%w` and the conflicting email for debugging
  context. The handler sees "conflict" and returns 409. It doesn't know which
  database or which constraint was violated.
- (2) Unknown errors get `%v` wrapping, same as before. The message is
  preserved for logging but the chain is severed.

The service layer doesn't need to do any mapping of its own. It passes domain
errors from the store straight through. When it has business reasons to
produce the same error independently, it uses the same sentinel:

```go
// user/service.go

func (s *Service) GetUser(
    ctx context.Context, id int64,
) (User, error) {
    u, err := s.store.Get(ctx, id)
    if err != nil {
        return User{}, err // (1)
    }
    if u.DeletedAt != nil {
        return User{}, fmt.Errorf(
            "user %d soft-deleted: %w", id, ErrNotFound, // (2)
        )
    }
    return u, nil
}
```

- (1) If the store returned `ErrNotFound` (missing row), it passes through
  unchanged. The service doesn't translate anything here because the error is
  already in domain terms.
- (2) A soft-deleted user exists in the database but is logically gone. The
  service wraps `ErrNotFound` with `%w` and the user ID. `%w` is appropriate
  here because `ErrNotFound` is the service's own error, not a leaked storage
  detail. The handler can still match it with `errors.Is(err, ErrNotFound)`.

> [!IMPORTANT]
>
> You don't need to translate at every layer. The repository maps storage
> errors to domain errors. The handler maps domain errors to wire format. The
> service layer in between just passes domain errors through unchanged. Two
> translation points, not one per layer.

Once the repository handles the storage-to-domain mapping, the handler gets
much simpler.

## Mapping domain errors to status codes

Compare this to the handler from the beginning of the post. No `database/sql`
import, no `redis` import, no knowledge of which storage backends exist:

```go
// main.go

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

    u, err := h.svc.GetUser(r.Context(), id)
    if err != nil {
        writeError(w, err)
        return
    }
    json.NewEncoder(w).Encode(u)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    u, err := h.svc.CreateUser(r.Context(), req.Name, req.Email)
    if err != nil {
        writeError(w, err)
        return
    }
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(u)
}
```

All error-to-status mapping lives in one function. Domain errors go in, HTTP
status codes come out:

```go
// main.go

func writeError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, user.ErrNotFound):
        http.Error(w, "not found", http.StatusNotFound) // (1)
    case errors.Is(err, user.ErrConflict):
        http.Error(w, "conflict", http.StatusConflict) // (2)
    default:
        http.Error(w, "internal error", http.StatusInternalServerError) // (3)
    }
}
```

- (1) `ErrNotFound` becomes 404. The handler doesn't know if it was a SQL
  miss, a Redis miss, or a soft delete. It doesn't need to.
- (2) `ErrConflict` becomes 409. The handler doesn't know which constraint
  was violated.
- (3) Anything else becomes 500 with a generic message. No internal details
  leak to the client.

The gRPC handler uses the same service with a different mapping function:

```go
// main.go

func toStatus(err error) error {
    switch {
    case errors.Is(err, user.ErrNotFound):
        return status.Error(codes.NotFound, "not found") // 404 equivalent
    case errors.Is(err, user.ErrConflict):
        return status.Error(codes.AlreadyExists, "conflict") // 409 equivalent
    default:
        return status.Error(codes.Internal, "internal error")
    }
}
```

```go
// main.go

func (h *handler) GetUser(
    ctx context.Context, req *api.GetUserRequest,
) (*api.GetUserResponse, error) {
    u, err := h.svc.GetUser(ctx, req.GetId())
    if err != nil {
        return nil, toStatus(err)
    }
    return &api.GetUserResponse{
        Id: u.ID, Name: u.Name, Email: u.Email,
    }, nil
}

func (h *handler) CreateUser(
    ctx context.Context, req *api.CreateUserRequest,
) (*api.CreateUserResponse, error) {
    u, err := h.svc.CreateUser(
        ctx, req.GetName(), req.GetEmail(),
    )
    if err != nil {
        return nil, toStatus(err)
    }
    return &api.CreateUserResponse{Id: u.ID}, nil
}
```

`writeError` and `toStatus` have the same shape. One outputs HTTP status
codes, the other outputs gRPC status codes. The service behind both is
identical. If you add a new error like `ErrForbidden`, you define one sentinel
in the `user` package and add one case to each mapping function.

## What you lose and how to get it back

When the handler sees `ErrNotFound`, it doesn't know whether that was a SQL
miss, a Redis miss, or a soft delete. That's the whole point of the
translation, but during an incident you need that information.

This is why the repository and service wrap `ErrNotFound` with descriptive
context using `%w`, as shown above. The repository produces
`"user 42 not in db: not found"` and the service produces
`"user 42 soft-deleted: not found"`. Same domain error, different origin. The
handler treats both as 404, but the error strings are distinct.

To make this useful, the handler logs the full error before returning the
response:

```go
// main.go

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

    u, err := h.svc.GetUser(r.Context(), id)
    if err != nil {
        slog.ErrorContext(r.Context(), "get user failed",
            "user_id", id,
            "err", err,
        )
        writeError(w, err) // responds with "not found", not err.Error()
        return
    }
    json.NewEncoder(w).Encode(u)
}
```

The client sees a 404 with the body `not found`. The on-call engineer sees
this:

```
level=ERROR msg="get user failed" user_id=42
    err="user 42 not in db: not found"
```

The error string tells you which code path produced the error. If you have
tracing set up, the request-scoped context carries the trace ID too, so you
can follow the 404 all the way back to the storage call that failed.

## The standard library does the same thing

`io.EOF` is the most familiar example of this pattern. Reading a file
returns it when the read syscall gets 0 bytes. Reading a TCP connection
returns it when the peer hangs up. Reading a byte buffer returns it when
there's nothing left. Three different mechanisms, one error. Callers write
`if err == io.EOF` and never think about what's underneath.

`fs.ErrNotExist` works the same way across platforms. On Linux, a missing
file produces `syscall.ENOENT`. On Windows, it produces
`ERROR_FILE_NOT_FOUND`. The `os` package catches both and maps them to
`fs.ErrNotExist`. Callers write `errors.Is(err, fs.ErrNotExist)` and it
works everywhere.

Both follow the same structure as the repository in this post: the layer
that knows the implementation detail catches it and returns a domain error
instead.

etcd's [clientv3 package] does the same translation in the reverse
direction. The client receives gRPC status codes from the server and maps
them into plain Go errors so callers never import
`google.golang.org/grpc/status`. I covered this in
[Wrapping a gRPC client in Go].

---

Working examples for the [HTTP version] and the [gRPC version] are on GitHub,
in the [error-translation] directory.

<!-- references -->
<!-- prettier-ignore-start -->

[Go errors: to wrap or not to wrap?]:
    /go/to-wrap-or-not-to-wrap

[Wrapping a gRPC client in Go]:
    /go/wrap-grpc-client

[clientv3 package]:
    https://github.com/etcd-io/etcd/tree/main/client/v3

[error-translation]:
    https://github.com/rednafi/examples/tree/main/error-translation

[HTTP version]:
    https://github.com/rednafi/examples/tree/main/error-translation/http

[gRPC version]:
    https://github.com/rednafi/examples/tree/main/error-translation/grpc

<!-- prettier-ignore-end -->
