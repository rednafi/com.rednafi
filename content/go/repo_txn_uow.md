---
title: Repositories, transactions, and unit of work in Go
date: 2026-03-21
slug: repo-txn-uow
tags:
    - Go
    - Database
    - Design Patterns
description: >-
  Decoupling business logic from storage in Go, adding transaction support without leaking
  SQL details, and coordinating atomic writes across multiple repositories using a unit of
  work.
---

This post started as a pair of quick answers to questions on [r/golang]. The first was about
whether [a repository layer on top of sqlc is worth it]. The second was about how to
[handle transactions when the interface hides storage details]. Both turned into
short shards on this site. This post ties them together and covers what to do when
transactions need to span multiple repositories.

It walks through three stages, each building on the last:

1. Put a repository interface between your service logic and your storage layer
2. Add transaction support to a single repository without leaking SQL into the service
3. Coordinate transactions across multiple repositories using a unit of work

All code examples use SQLite. Working examples for the [single-store version] and the
[cross-store version] are on GitHub.

## What's a repository?

Martin Fowler defined the repository pattern in
[Patterns of Enterprise Application Architecture]:

> A Repository mediates between the domain and data mapping layers using a collection-like
> interface for accessing domain objects.

In Go, a repository is just an interface. The service depends on the interface, a concrete
package implements it, and they live in separate packages. The service defines what it needs,
the storage satisfies it. The [dependency inversion principle] in action.

To see why this matters, consider what happens when you skip it.

## What happens without one

Say you're building a bookstore service with [sqlc]. The generated code gives you a `Queries`
struct with methods like `GetBook` and `CreateBook`. The tempting thing is to inject that
directly into your service:

```go
type Service struct {
    q *db.Queries
}

func (s *Service) RegisterBook(
    ctx context.Context, title string) (db.Book, error) {
    return s.q.CreateBook(ctx, title)
}
```

This compiles and runs, but the service is now welded to sqlc's generated types. Every service
method imports the `db` package. If you want to test `RegisterBook` without a database, you
need to mock the entire `Queries` struct or spin up a test database. If you later switch from
sqlc to raw SQL, or from Postgres to DynamoDB, you're rewriting the service layer too.

The service should describe _what_ it needs from storage without knowing _how_ storage does
it. "Get me a book by ID" and "create this book" are the what. SQL queries, connection pools,
and table schemas are the how. A small interface fixes that.

## Adding a repository interface

The interface lives in the `book` package alongside the domain types. This is the
business logic package. It has no imports from `database/sql` or any storage library:

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

Two methods. `Get` retrieves a book by ID, `Create` persists a new one and returns the
generated ID. The interface says nothing about SQL, tables, or connection pools. Any storage
backend that can get and create books can satisfy it.

The service depends only on `Store`:

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

func (s *Service) GetBook(
    ctx context.Context, id int64) (Book, error) {
    return s.store.Get(ctx, id)
}
```

`RegisterBook` builds a `Book`, asks the store to persist it, and gets an ID back. It doesn't
import anything from `database/sql`. The `book` package has zero storage dependencies.

Now we need something that actually talks to a database.

## SQLite implementation

A separate `sqlite` package satisfies the `Store` interface. I'm writing the queries by
hand here to avoid the sqlc ceremony, but the structure would be the same. sqlc would just
generate the query methods for you.

Before writing the store methods, there's one thing to set up. sqlc generates a `DBTX`
interface that both `*sql.DB` and `*sql.Tx` satisfy. `*sql.DB` is a connection pool,
`*sql.Tx` is a transaction:

```go
// sqlite/store.go

type DBTX interface {
    ExecContext(
        ctx context.Context,
        query string, args ...any) (sql.Result, error)
    QueryRowContext(
        ctx context.Context,
        query string, args ...any) *sql.Row
}
```

Why does this matter? Because `*sql.DB` has these two methods, and so does `*sql.Tx`. Any
code written against `DBTX` works with either one. We don't need this for the basic
repository, but it becomes important when we add transactions later.

The store struct holds `DBTX` instead of `*sql.DB`. If the store held `*sql.DB` directly, we
couldn't later construct a store backed by a transaction. Holding `DBTX` keeps that door
open:

```go
// sqlite/store.go

type BookStore struct{ db DBTX }

func NewBookStore(db DBTX) *BookStore { return &BookStore{db: db} }
```

The query methods call `s.db.ExecContext` and `s.db.QueryRowContext`, which right now go
through a `*sql.DB` connection pool:

```go
// sqlite/store.go

func (s *BookStore) Get(
    ctx context.Context, id int64) (book.Book, error) {
    row := s.db.QueryRowContext(ctx,
        "SELECT id, title FROM books WHERE id = ?", id)
    var b book.Book
    err := row.Scan(&b.ID, &b.Title)
    return b, err
}

func (s *BookStore) Create(
    ctx context.Context, b book.Book) (int64, error) {
    res, err := s.db.ExecContext(ctx,
        "INSERT INTO books (title) VALUES (?)", b.Title)
    if err != nil {
        return 0, err
    }
    return res.LastInsertId()
}
```

Later, when we add transactions, `s.db` will be a `*sql.Tx` instead of a `*sql.DB`, and
these same methods will execute against the transaction without any code changes. That's the
payoff of holding `DBTX`.

Wiring it up at startup is one line per dependency:

```go
// cmd/main.go

store := sqlite.NewBookStore(db)
svc := book.NewService(store)
```

The service receives a `Store`, which is the interface. The SQLite package receives a
`*sql.DB`, which satisfies `DBTX`. Neither package imports the other.

## Testing without a database

Since the service depends on an interface, we can test it without any database by writing an
in-memory fake:

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

The `var _ Store = (*memStore)(nil)` line is an [interface guard]. If `memStore` ever
stops satisfying `Store`, the build fails.

The test looks like production code, minus the database:

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

This runs in microseconds and exercises the same `RegisterBook` code that runs in production.
If the storage layer changes from SQLite to Postgres tomorrow, this test stays the same
because it only depends on the interface.

You should still write integration tests against a real database (we'll see those shortly),
but the bulk of your service logic can be tested with fakes.

So far we have a clean separation: the service talks to an interface, the SQLite package
implements it, and tests use an in-memory fake. But every method on the interface runs
independently. If `RegisterBook` needs to make two writes that must succeed or fail together,
we have a problem.

## Adding transactions to a single repository

Say the business requirements change. When a book is registered, we now also need to write an
audit log entry recording who created it and when. Both writes must be atomic: if the book
insert succeeds but the audit log fails, we don't want a book in the database with no audit
trail. That means we need a transaction.

This is the question that xinoiP [raised on Reddit]:

> How would you handle transactions with this approach? Since they are very specific to SQL.

To support the new requirement, the `Store` interface needs two additions. First, an
`AuditEntry` type and a `CreateAuditLog` method for the audit writes. Second, a `Tx` method
that lets the service group multiple operations into a single transaction:

```go
// book/book.go

type AuditEntry struct {
    BookID int64
    Action string
}

type Store interface {
    Get(ctx context.Context, id int64) (Book, error)
    Create(ctx context.Context, b Book) (int64, error)
    CreateAuditLog(ctx context.Context, e AuditEntry) error

    // Tx runs fn inside a transaction. The Store passed
    // to fn executes against that transaction.
    Tx(ctx context.Context, fn func(Store) error) error
}
```

`CreateAuditLog` is a regular data access method like `Get` and `Create`. The interesting
one is `Tx`. It takes a callback function that receives a `Store`. The `Store`
passed to the callback is backed by a database transaction, so every method called on it
executes within that transaction. Same idea as
[passing locked state into a closure]. The caller doesn't manage the lifecycle. No manual
begin/commit/rollback, just like no manual lock/unlock. It works with what the callback
gives it.

Here's how the SQLite implementation of `Tx` works:

```go
// sqlite/store.go

func (s *BookStore) Tx(
    ctx context.Context,
    fn func(book.Store) error) error {

    sqlDB, ok := s.db.(*sql.DB)
    if !ok {
        return errors.New(
            "cannot start tx: already inside a transaction")
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

The type assertion `s.db.(*sql.DB)` checks that the underlying executor is a connection
pool and not an existing transaction. You can't nest `sql.Tx` inside `sql.Tx` in
`database/sql`. After starting the transaction with `BeginTx`, it builds a fresh `BookStore`
whose `db` field is the `*sql.Tx`. This is the payoff of the `DBTX` setup from earlier.
`*sql.Tx` satisfies `DBTX`, so the new store works with the exact same `Get`, `Create`,
and `CreateAuditLog` methods. The callback gets this transactional store, and every query
inside the callback goes through the transaction. If the callback returns an error, we roll
back. Otherwise we commit.

The caller never touches `sql.Tx`.

## Using Tx in RegisterBook

With `Tx` on the interface, `RegisterBook` can now create a book and an audit log entry
atomically. It calls `s.store.Tx`, and everything inside the callback goes through the
transactional store:

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

Both `tx.Create` and `tx.CreateAuditLog` execute against the same `sql.Tx`. If either fails,
the callback returns an error, and `Tx` rolls back both writes. If both succeed, `Tx` commits
them together. `RegisterBook` never sees `sql.Tx`, `*sql.DB`, or anything from `database/sql`.

## Testing single-store transactions

The in-memory store needs a `Tx` method now. Since there's no real database, it just calls
the function directly with itself:

```go
// book/service_test.go

func (m *memStore) Tx(
    ctx context.Context, fn func(Store) error) error {
    return fn(m)
}
```

This is enough to test service logic: whether `RegisterBook` calls both `Create` and
`CreateAuditLog`, and whether it handles errors correctly.

For integration tests that verify actual commit/rollback behavior at the database level, use
a real database:

```go
// sqlite/store_test.go

func TestTx_RollsBackOnError(t *testing.T) {
    db := setupTestDB(t)

    base := NewBookStore(db)
    failing := &failingStore{BookStore: base}

    svc := book.NewService(failing)

    _, err := svc.RegisterBook(t.Context(), "DDIA")
    if err == nil {
        t.Fatal("expected error")
    }

    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM books").Scan(&count)
    if err != nil {
        t.Fatal(err)
    }
    if count != 0 {
        t.Fatalf("expected 0 books after rollback, got %d", count)
    }
}
```

`failingStore` embeds the real SQLite `BookStore` but overrides `CreateAuditLog` to always return
an error. The sequence: `Tx` begins a transaction, `Create` inserts a book (inside the
transaction), `CreateAuditLog` fails, `Tx` rolls back, and the books table is empty.

Unit tests with fakes cover service logic quickly. Integration tests with a real database
cover transactional behavior. The interface makes both possible from the same service code.

## Why not use context to pass the transaction?

xinoiP's original suggestion was to put a `*sql.Tx` in the context and have the store check
for it:

```go
// Don't do this.
func (s *BookStore) Create(ctx context.Context, b Book) (int64, error) {
    var executor DBTX
    if tx, ok := TxFromContext(ctx); ok {
        executor = tx
    } else {
        executor = s.db
    }
    // ...
}
```

This works, but the service has to call something like `ctx = WithTx(ctx, tx)` before
calling the store, which means it knows a SQL transaction exists. That's the coupling the
interface was supposed to prevent.

There's another issue as well. Context values are untyped and invisible. If someone forgets
to set the transaction in context, or sets it on the wrong context, the store silently falls
back to the connection pool and the operations aren't atomic. With the callback approach, the
transactional store is passed as a function argument. It won't catch every mistake — you could
still accidentally call `s.store` instead of `tx` for one of several operations — but it's
harder to miss than an invisible context value.

With the callback, the service says "run these operations atomically" and the store decides
how. Swap Postgres for DynamoDB tomorrow and the service code doesn't change.

## Transactions across multiple repositories

The per-store `Tx` from the previous sections works when all writes go through the same
`Store`. Both `Create` and `CreateAuditLog` live on `Store`, so one store's `Tx`
method can wrap them in a single transaction.

But domains grow. Say the bookstore now tracks inventory and handles orders. Books get a
`Stock` field, there's a new `Order` type, and a new `Store` interface for order-related
queries. Each store still has its own `Tx`:

```go
// book/book.go

type Store interface {
    Get(ctx context.Context, id int64) (Book, error)
    Create(ctx context.Context, b Book) (int64, error)
    CreateAuditLog(ctx context.Context, e AuditEntry) error
    DecrementStock(ctx context.Context, id int64) error

    Tx(ctx context.Context, fn func(Store) error) error
}

// order/order.go

type Store interface {
    Create(ctx context.Context, o Order) (int64, error)
    Get(ctx context.Context, id int64) (Order, error)

    Tx(ctx context.Context, fn func(Store) error) error
}
```

`DecrementStock` reduces a book's inventory count by one. A checkout flow needs to call
`DecrementStock` on `book.Store` _and_ `Create` on `order.Store`, and both must commit or
roll back together. If the stock decrements but the order insert fails, you've lost inventory
with no corresponding order.

You might try nesting the callbacks:

```go
// This doesn't work.
err := s.books.Tx(ctx, func(txBooks book.Store) error {
    if err := txBooks.DecrementStock(ctx, bookID); err != nil {
        return err
    }
    return s.orders.Tx(ctx, func(txOrders order.Store) error {
        _, err := txOrders.Create(ctx, order.Order{BookID: bookID})
        return err
    })
})
```

This compiles, but `books.Tx` starts one `sql.Tx` for the book store and
`orders.Tx` starts a _second_, independent `sql.Tx` for the order store. If the order
insert fails, the order transaction rolls back, but the stock decrement has already committed
in the first transaction.

Each store only knows how to build a transactional copy of itself. You need something that
can build _all_ stores from a single `sql.Tx`.

## Unit of work

We need a coordinator that starts a single database transaction and constructs every store
from it. Martin Fowler called this pattern a Unit of Work in
[Patterns of Enterprise Application Architecture]:

> A Unit of Work keeps track of everything you do during a business transaction that can
> affect the database. When you're done, it figures out everything that needs to be done to
> alter the database as a result of your work.

Fowler's original formulation tracks dirty objects in memory and flushes them all in one
transaction. ORMs like Hibernate implement it that way. In Go, we don't need object tracking
since our stores already know how to write to the database. We just need to start one
`sql.Tx`, construct all stores from it, and pass them to a callback.

Since the unit of work now owns transaction management, we can strip `Tx` from both store
interfaces. The stores go back to being pure data access:

```go
// book/book.go

type Store interface {
    Get(ctx context.Context, id int64) (Book, error)
    Create(ctx context.Context, b Book) (int64, error)
    CreateAuditLog(ctx context.Context, e AuditEntry) error
    DecrementStock(ctx context.Context, id int64) error
}

// order/order.go

type Store interface {
    Create(ctx context.Context, o Order) (int64, error)
    Get(ctx context.Context, id int64) (Order, error)
}
```

A `Stores` struct groups all the repositories together, and a `UnitOfWork` interface provides
the single `RunInTx` method that replaces per-store `Tx`:

```go
// checkout/checkout.go

type Stores struct {
    Books  book.Store
    Orders order.Store
}

type UnitOfWork interface {
    // RunInTx runs fn inside a single transaction. Every store
    // in the Stores value executes against that transaction.
    RunInTx(ctx context.Context, fn func(Stores) error) error
}
```

`Stores` is a plain struct holding the same interfaces the service already depends on. As
the domain grows, you add more fields to it.

The SQLite implementation starts one transaction and constructs both stores from it:

```go
// sqlite/store.go

type UoW struct{ db *sql.DB }

func NewUoW(db *sql.DB) *UoW { return &UoW{db: db} }

func (u *UoW) RunInTx(
    ctx context.Context,
    fn func(checkout.Stores) error) error {

    tx, err := u.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op after Commit

    stores := checkout.Stores{
        Books:  NewBookStore(tx),
        Orders: NewOrderStore(tx),
    }

    if err := fn(stores); err != nil {
        return err
    }
    return tx.Commit()
}
```

Same `DBTX` trick as before. `NewBookStore(tx)` and `NewOrderStore(tx)` both accept `DBTX`,
and `*sql.Tx` satisfies `DBTX`. Both stores execute against the same transaction. When the
callback returns, either everything commits or everything rolls back.

## Using RunInTx in the service

Since the service now uses a `UnitOfWork` for transactions instead of per-store `Tx`, its
dependencies change. It takes a `Stores` for non-transactional reads and a `UnitOfWork` for
atomic writes:

```go
// checkout/checkout.go

type Service struct {
    stores Stores
    uow    UnitOfWork
}

func NewService(s Stores, uow UnitOfWork) *Service {
    return &Service{stores: s, uow: uow}
}
```

`PlaceOrder` reads the book outside the transaction (no need to hold a lock for a read), then
uses `RunInTx` for the two writes that must be atomic:

```go
// checkout/checkout.go

func (s *Service) PlaceOrder(
    ctx context.Context, bookID int64) (order.Order, error) {

    book, err := s.stores.Books.Get(ctx, bookID)
    if err != nil {
        return order.Order{}, err
    }

    var ord order.Order
    err = s.uow.RunInTx(ctx, func(tx Stores) error {
        if err := tx.Books.DecrementStock(ctx, book.ID); err != nil {
            return err
        }
        id, err := tx.Orders.Create(ctx, order.Order{BookID: book.ID})
        if err != nil {
            return err
        }
        ord = order.Order{ID: id, BookID: book.ID}
        return nil
    })

    return ord, err
}
```

Inside the callback, `tx.Books` and `tx.Orders` both execute against the same `sql.Tx`. If
`DecrementStock` succeeds but `Orders.Create` fails, the entire transaction rolls back and
the stock decrement is undone.

Single-store operations work the same way. `RegisterBook` goes through `RunInTx` and uses
only `tx.Books`, ignoring `tx.Orders`:

```go
// checkout/checkout.go

func (s *Service) RegisterBook(
    ctx context.Context, title string) (book.Book, error) {

    var b book.Book

    err := s.uow.RunInTx(ctx, func(tx Stores) error {
        id, err := tx.Books.Create(ctx, book.Book{Title: title})
        if err != nil {
            return err
        }
        b = book.Book{ID: id, Title: title}
        return tx.Books.CreateAuditLog(ctx,
            book.AuditEntry{BookID: id, Action: "created"})
    })

    return b, err
}
```

Once you have a unit of work, there's no need to keep per-store `Tx`. `RunInTx` handles both
single-store and cross-store transactions.

## Testing cross-store transactions

For unit tests, the in-memory unit of work passes the stores straight through:

```go
// checkout/checkout_test.go

type memUoW struct {
    stores Stores
}

func (m *memUoW) RunInTx(
    _ context.Context, fn func(Stores) error) error {
    return fn(m.stores)
}
```

For integration tests, verify that a failure in one store actually rolls back writes from the
other. In this test, the order insert fails, and we check that the stock decrement was undone:

```go
// sqlite/store_test.go

func TestRunInTx_RollsBackOnError(t *testing.T) {
    db := setupTestDB(t)
    bookID := seedBook(t, db, "DDIA", 5)

    stores := checkout.Stores{
        Books:  NewBookStore(db),
        Orders: NewOrderStore(db),
    }
    failUoW := &failingOrderUoW{db: db}
    svc := checkout.NewService(stores, failUoW)

    _, err := svc.PlaceOrder(t.Context(), bookID)
    if err == nil {
        t.Fatal("expected error")
    }

    // Stock should be unchanged because the tx rolled back.
    var stock int
    err = db.QueryRow(
        "SELECT stock FROM books WHERE id = ?",
        bookID).Scan(&stock)
    if err != nil {
        t.Fatal(err)
    }
    if stock != 5 {
        t.Fatalf("stock = %d, want 5", stock)
    }
}
```

`failingOrderUoW` is a `UnitOfWork` whose order `Store` always fails on `Create`. It starts
a real `sql.Tx`, builds both stores from it with the failing order store swapped in, and
rolls back when the callback returns an error:

```go
// sqlite/store_test.go

type failingOrderUoW struct{ db *sql.DB }

func (u *failingOrderUoW) RunInTx(
    ctx context.Context,
    fn func(checkout.Stores) error) error {

    tx, err := u.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op after Commit

    stores := checkout.Stores{
        Books:  NewBookStore(tx),
        Orders: &failingOrderStore{},
    }

    if err := fn(stores); err != nil {
        return err
    }
    return tx.Commit()
}

type failingOrderStore struct{}

func (f *failingOrderStore) Create(
    _ context.Context, _ order.Order) (int64, error) {
    return 0, sql.ErrConnDone
}

func (f *failingOrderStore) Get(
    _ context.Context, _ int64) (order.Order, error) {
    return order.Order{}, sql.ErrConnDone
}
```

`DecrementStock` ran inside the transaction and modified the stock, but because the order
insert failed, the entire transaction rolled back and the stock is back to 5.

## Is this too much abstraction for Go?

Yes. Do I always do it? Nope.

In larger codebases though, it's easy to end up with a mess if you mix storage concerns into
the service logic. I've seen it play out many times: you start with spaghetti in the name of
simplicity and things get out of hand as the codebase grows. With LLMs, generating code is
cheap. Guiding the clanker toward a good design doesn't cost much and pays dividends
throughout.

That said, I typically skip the ceremony when I'm knocking out something for my own use, or
working in a smaller codebase, or working in a codebase that doesn't do it already.

<!-- references -->
<!-- prettier-ignore-start -->

[r/golang]:
    https://www.reddit.com/r/golang/

[sqlc]:
    https://github.com/sqlc-dev/sqlc

[gorm]:
    https://github.com/go-gorm/gorm

[raised on Reddit]:
    https://www.reddit.com/r/golang/comments/1rv65k9/comment/obdrohe/

[single-store version]:
    https://github.com/rednafi/examples/tree/main/repository-transactions

[cross-store version]:
    https://github.com/rednafi/examples/tree/main/cross-repository-transactions

[a repository layer on top of sqlc is worth it]:
    /shards/2026/03/repository-layer-over-sqlc/

[handle transactions when the interface hides storage details]:
    /shards/2026/03/transactions-with-repository-pattern/

[Patterns of Enterprise Application Architecture]:
    https://martinfowler.com/eaaCatalog/

[dependency inversion principle]:
    https://en.wikipedia.org/wiki/Dependency_inversion_principle

[Go Proverbs]:
    https://go-proverbs.github.io/

[passing locked state into a closure]:
    /go/mutex-closure/

[interface guard]:
    /go/interface-guards/

<!-- prettier-ignore-end -->
