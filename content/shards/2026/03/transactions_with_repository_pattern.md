---
title: How do you handle transactions with the repository pattern?
date: 2026-03-20
slug: transactions-with-repository-pattern
tags:
    - Go
    - Database
description: >-
  Adding transaction support to a repository interface without leaking storage details.
---

[Previously], I showed how to put a small interface between your service logic and your
storage layer so the service doesn't know whether it's talking to sqlc, raw SQL, or anything
else. The interface looked like this:

```go
// book/book.go

type Store interface {
    Get(ctx context.Context, id int64) (Book, error)
    Create(ctx context.Context, b Book) (int64, error)
}
```

A service depends on `Store`, a concrete `postgres` package satisfies it, and in tests you
swap in an in-memory fake. The service never imports `database/sql`.

In the [same Reddit thread], user xinoiP [asked]:

> How would you handle transactions with this approach? Since they are very specific to SQL.
> I tend to use context and store an optional transaction in there that can be used on the
> implementation of that interface. So, sqlc checks the context, if there is a transaction,
> uses it etc. I just wonder how you would handle it.

If each method on the interface runs independently, there's no way to make two writes
atomic. Say `RegisterBook` needs to insert a book _and_ an audit log entry, and both must
commit or roll back together.

---

The key is something sqlc already gives you. It generates a `DBTX` interface that both
`*sql.DB` and `*sql.Tx` satisfy:

```go
// Simplified from what sqlc generates.

type DBTX interface {
    ExecContext(
        ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

If your store struct accepts `DBTX` instead of `*sql.DB`, you can construct a store backed
by either a connection pool or a live transaction. Same struct, same methods, different
underlying executor. That means the interface can offer a `Tx` method that hands the caller
a transactional version of itself:

```go
// book/book.go

type Store interface {
    Get(ctx context.Context, id int64) (Book, error)
    Create(ctx context.Context, b Book) (int64, error)
    CreateAuditLog(ctx context.Context, e AuditEntry) error

    // Tx runs fn inside a transaction. The Store passed
    // to fn executes against that transaction.
    Tx(ctx context.Context, fn func(Store) error) error
}
```

The Postgres implementation of `Tx` starts a `sql.Tx`, wraps it in a fresh `BookStore`, and
passes that into the callback. If the callback returns an error, it rolls back. Otherwise it
commits:

```go
// postgres/store.go

// Previously this was `type BookStore struct{ db *sql.DB }`.
type BookStore struct{ db DBTX }

func NewBookStore(db DBTX) *BookStore { return &BookStore{db: db} }

func (s *BookStore) Tx(
    ctx context.Context, fn func(book.Store) error) error {

    sqlDB, ok := s.db.(*sql.DB)
    if !ok {
        return errors.New("cannot start tx: already inside a transaction")
    }

    tx, err := sqlDB.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op after Commit

    // Build a new BookStore backed by the tx.
    if err := fn(NewBookStore(tx)); err != nil {
        return err
    }
    return tx.Commit()
}
```

`NewBookStore(tx)` works because the struct field is `DBTX`, and `*sql.Tx` satisfies that
interface. No new type, no wrapper. The `Get`, `Create`, and `CreateAuditLog` methods on
this transactional store run their queries against the `tx` automatically.

The service uses `Tx` when it needs atomicity. Everything inside the callback goes through
the transactional store:

```go
// book/service.go

func (s *Service) RegisterBook(
    ctx context.Context, title string) (Book, error) {

    var book Book

    err := s.store.Tx(ctx, func(tx Store) error {
        id, err := tx.Create(ctx, Book{Title: title})
        if err != nil {
            return err
        }
        book = Book{ID: id, Title: title}
        return tx.CreateAuditLog(ctx,
            AuditEntry{BookID: id, Action: "created"})
    })

    return book, err
}
```

Both writes commit or roll back together. `RegisterBook` never sees `sql.Tx`, `*sql.DB`, or
anything from `database/sql`. If the audit log insert fails, the book insert is rolled back
too.

For tests, `Tx` just calls the function directly against the in-memory store:

```go
// book/service_test.go

func (m *memStore) Tx(
    ctx context.Context, fn func(Store) error) error {
    return fn(m)
}
```

No real transaction needed. The test exercises the same service code as production. If you
need to verify actual commit/rollback behavior, swap the in-memory store for something like
SQLite.

---

Back to xinoiP's approach of storing a `*sql.Tx` in the context: it works, but it leaks
storage into the service layer through the back door. The service has to set up the
transaction in context before calling the store, which means it knows a SQL transaction
exists. That's the coupling the interface was supposed to prevent.

With the callback approach, the service says "run these operations atomically" and the store
decides how. Swap Postgres for DynamoDB tomorrow and the service code doesn't change - you
just implement `Tx` differently in the new storage package.

The full working example with an HTTP server and SQLite is [on GitHub].

See also:

- [Do you need a repository layer on top of sqlc?]
- [Repositories, transactions, and unit of work in Go]

<!-- references -->
<!-- prettier-ignore-start -->

[Previously]:
    /shards/2026/03/repository-layer-over-sqlc/

[same Reddit thread]:
    https://www.reddit.com/r/golang/comments/1rv65k9/

[asked]:
    https://www.reddit.com/r/golang/comments/1rv65k9/comment/obdrohe/

[on GitHub]:
    https://github.com/rednafi/examples/tree/main/repository-transactions

[Do you need a repository layer on top of sqlc?]:
    /shards/2026/03/repository-layer-over-sqlc/

[Repositories, transactions, and unit of work in Go]:
    /go/repo-txn-uow/

<!-- prettier-ignore-end -->
