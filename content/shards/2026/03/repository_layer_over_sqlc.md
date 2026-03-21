---
title: Do you need a repository layer on top of sqlc?
date: 2026-03-16
slug: repository-layer-over-sqlc
tags:
    - Go
    - Database
description: >-
  Decoupling business logic from storage with a small interface in Go.
---

Today in [r/golang], user Leading-West-4881 asked:

> Is a repository layer over sqlc over-engineering or necessary for scale? I'm building a
> notification engine in Go using sqlc for the DB layer. Do you just inject `*db.Queries`
> into your services, or do you find the abstraction of a repository layer worth the extra
> code?

I [attempted to answer it] there and the gist is correct. But I wrote it in a hurry so the
example and the explanation could be better. Capturing it properly here.

---

Call it repository or whatever you want, the name doesn't matter. The point is that your
business logic should be oblivious to the persistence layer. Doesn't matter if it's [sqlc],
raw `database/sql`, or [gorm]. If your service functions call sqlc queries directly, your
core logic is coupled to your database code. That makes it harder to test in isolation and
harder to swap out later.

Put a small interface between your business code and your storage code. The business side
defines what it needs, the storage side satisfies it, and they live in separate packages.

Say you're building a service that manages books. Start with the domain type and the storage
interface:

```go
// book/book.go

type Book struct {
    ID    int64
    Title string
}

type Store interface {
    Get(ctx context.Context, id int64) (Book, error)
    Create(ctx context.Context, b Book) (int64, error)
}
```

The service depends only on that interface:

```go
// book/service.go

type Service struct {
    store Store
}

func NewService(s Store) *Service {
    return &Service{store: s}
}

func (s *Service) RegisterBook(
    ctx context.Context, title string) (Book, error) {

    b := Book{Title: title}
    id, err := s.store.Create(ctx, b)
    if err != nil {
        return Book{}, err
    }
    b.ID = id
    return b, nil
}

func (s *Service) GetBook(ctx context.Context, id int64) (Book, error) {
    return s.store.Get(ctx, id)
}
```

`RegisterBook` doesn't know about SQL, sqlc, or Postgres. It builds a `Book`, asks the store
to persist it, and gets an ID back.

The concrete implementation goes in a separate package. This is where sqlc-generated code
would live:

```go
// postgres/store.go

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Get(ctx context.Context, id int64) (book.Book, error) {
    // sqlc query or raw sql, doesn't matter
    // ...
}

func (s *Store) Create(
    ctx context.Context, b book.Book) (int64, error) {
    // INSERT INTO books (title) VALUES ($1) RETURNING id
    // ...
}
```

Wire it up at startup:

```go
// cmd/main.go

store := postgres.NewStore(db)
svc := book.NewService(store)
```

In tests, swap in a fake that satisfies the same interface:

```go
// book/service_test.go

var _ Store = (*memStore)(nil)

type memStore struct {
    mu   sync.Mutex
    books map[int64]Book
    next int64
}

func (m *memStore) Get(
    ctx context.Context, id int64) (Book, error) {

    m.mu.Lock()
    defer m.mu.Unlock()
    b, ok := m.books[id]
    if !ok {
        return Book{}, fmt.Errorf("book %d not found", id)
    }
    return b, nil
}

func (m *memStore) Create(
    ctx context.Context, b Book) (int64, error) {

    m.mu.Lock()
    defer m.mu.Unlock()
    m.next++
    b.ID = m.next
    m.books[b.ID] = b
    return b.ID, nil
}
```

Now the test reads exactly like production code, minus Postgres:

```go
// book/service_test.go

func TestRegisterBook(t *testing.T) {
    store := &memStore{books: make(map[int64]Book)}
    svc := NewService(store)

    b, err := svc.RegisterBook(t.Context(), "DDIA")
    if err != nil {
        t.Fatal(err)
    }
    if b.ID == 0 {
        t.Fatal("expected non-zero ID")
    }
    if b.Title != "DDIA" {
        t.Fatalf("got title %q, want DDIA", b.Title)
    }
}
```

Same service code, no database needed. The test exercises `RegisterBook` without touching
SQL. If the storage layer changes tomorrow, the service and its tests stay the same.

A working example with transactions, tests, and an HTTP server is [on GitHub].

See also:

- [How do you handle transactions with the repository pattern?]
- [Repository pattern & transactions in Go]

<!-- references -->
<!-- prettier-ignore-start -->

[r/golang]:
    https://www.reddit.com/r/golang/

[attempted to answer it]:
    https://www.reddit.com/r/golang/comments/1rv65k9/comment/oasp30r/

[sqlc]:
    https://github.com/sqlc-dev/sqlc

[gorm]:
    https://github.com/go-gorm/gorm

[on GitHub]:
    https://github.com/rednafi/examples/tree/main/repository-transactions

[How do you handle transactions with the repository pattern?]:
    /shards/2026/03/transactions-with-repository-pattern/

[Repository pattern & transactions in Go]:
    /go/repository-pattern-and-transactions/

<!-- prettier-ignore-end -->
