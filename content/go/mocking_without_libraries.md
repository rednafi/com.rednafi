---
title: Your Go tests probably don't need a mocking library
date: 2026-01-23
slug: mocking-libraries-bleh
tags:
    - Go
    - Testing
description: >-
    Practical patterns for mocking in Go without external libraries. Learn to mock
    functions, methods, interfaces, HTTP calls, and time using only the standard library
---

I have nothing against mocking libraries like [gomock] or [mockery]. I use them all the
time, both at work and outside. But one thing I've noticed is that generating mocks often
leads to poorly designed tests and increases onboarding time for a codebase.

Also, since almost no one writes tests by hand anymore and instead generates them with LLMs,
the situation gets more dire. These ghosts often pull in all kinds of third-party libraries
to mock your code, simply because they were trained on a lot of hastily written examples on
the web.

So the idea of this post isn't to discourage using mocking libraries. Rather, it's to show
that even if your codebase already has a mocking library in the dependency chain, not all of
your tests need to depend on it. Below are a few cases where I tend not to use any mocking
library and instead leverage the constructs that Go gives us.

This does require some extra song and dance with the language, but in return, we gain more
control over our tests and reduce the chance of encountering [spooky action at a distance].

## Mocking a function

Say you have a function that creates a database handle:

```go
func OpenDB(user, pass, host, dbName string) (*sql.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, pass, host, dbName)
    return sql.Open("mysql", dsn)
}
```

The problem is that `sql.Open` hands the DSN directly to the driver. When you call
`OpenDB("admin", "secret", "db.internal", "orders")`, the function formats the DSN string
and hands it to the MySQL driver. You can't intercept that call, you can't control what it
returns, and you probably don't want unit tests leaning on a real driver (or a real MySQL
instance) just to verify DSN formatting.

The fix is to make the database opener injectable:

```go
type SQLOpenFunc func(driver, dsn string) (*sql.DB, error)  // (1)

func OpenDB(
    user, pass, host, dbName string, openFn SQLOpenFunc, // (2)
) (*sql.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, pass, host, dbName)
    return openFn("mysql", dsn)  // (3)
}
```

Here:

- (1) defines a function type that matches `sql.Open`'s signature
- (2) accepts an opener function as a parameter
- (3) delegates to that function instead of calling `sql.Open` directly

In production, pass the real `sql.Open`:

```go
func main() {
    db, err := OpenDB(
        "admin", "secret", "db.internal", "orders", sql.Open,  // (1)
    )
    // ...
}
```

Here:

- (1) the real `sql.Open` is passed as the last argument - no wrapper needed

In tests, pass a fake that captures what was passed or returns canned values:

```go
func TestOpenDB(t *testing.T) {
    var got string
    fakeOpen := func(driver, dsn string) (*sql.DB, error) {
        got = dsn  // (1) capture what was passed
        return nil, nil
    }

    OpenDB(
        "admin", "secret", "db.internal", "orders", fakeOpen,  // (2)
    )

    want := "admin:secret@tcp(db.internal)/orders"
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}
```

Here:

- (1) the fake captures the DSN for later assertion
- (2) the call site looks the same, just with a different opener

This pattern works for any function dependency - UUID generators, random number sources,
file openers. Functions are first-class values in Go, so you can pass them around like any
other value.

The downside is that parameter lists can grow quickly. If `OpenDB` also needed a logger, a
metrics client, and a config loader, the signature becomes unwieldy. When you find yourself
passing more than two or three function dependencies, consider grouping them into a struct
with an interface - see [Mocking a method on a type].

## Monkey patching

Sometimes you inherit code where refactoring the function signature isn't practical. Maybe
it's called from dozens of places, or it's part of a public API you can't change:

```go
func PublishOrderCreated(
    ctx context.Context, brokers []string, id string) error {
    w := &kafka.Writer{
        Addr: kafka.TCP(brokers...), Topic: "order-events",
    }
    defer w.Close()
    return w.WriteMessages(ctx, kafka.Message{Key: []byte(id)})
}
```

The Kafka writer is instantiated directly inside the function. There's no seam to inject a
fake without touching every call site. If this function is called from 50 places in your
codebase, changing its signature means updating all 50.

One workaround is a package-level variable that points to the constructor:

```go
type kafkaWriter interface {  // (1)
    WriteMessages(context.Context, ...kafka.Message) error
    Close() error
}

var newWriter = func(brokers []string) kafkaWriter {  // (2)
    return &kafka.Writer{
        Addr: kafka.TCP(brokers...), Topic: "order-events",
    }
}

func PublishOrderCreated(
    ctx context.Context, brokers []string, id string,
) error {
    w := newWriter(brokers)  // (3)
    defer w.Close()
    return w.WriteMessages(ctx, kafka.Message{Key: []byte(id)})
}
```

Here:

- (1) define an interface with only the methods we need from `kafka.Writer`
- (2) the package variable returns the interface type, not the concrete type
- (3) the function calls it instead of instantiating directly

Production code doesn't change - it calls `PublishOrderCreated` exactly as before, and the
default `newWriter` creates real Kafka writers.

Tests swap it out:

```go
type fakeWriter struct {
    key []byte
}

func (f *fakeWriter) WriteMessages(
    _ context.Context, msgs ...kafka.Message) error {
    if len(msgs) > 0 {
        f.key = msgs[0].Key  // (1)
    }
    return nil
}

func (f *fakeWriter) Close() error { return nil }

func TestPublishOrderCreated(t *testing.T) {
    orig := newWriter
    t.Cleanup(func() { newWriter = orig })  // (2) restore after test

    fake := &fakeWriter{}
    newWriter = func([]string) kafkaWriter {  // (3)
        return fake
    }

    PublishOrderCreated(
        t.Context(), []string{"kafka:9092"}, "ord-1",
    )

    if got := string(fake.key); got != "ord-1" {  // (4)
        t.Errorf("got %q, want %q", got, "ord-1")
    }
}
```

Here:

- (1) the fake captures the message key for later assertion
- (2) `t.Cleanup` ensures the original is restored even if the test fails
- (3) the replacement factory returns the fake - note it returns `kafkaWriter`, matching the
  variable's type
- (4) assert the captured key matches the expected value

This works, but be aware of the costs. Tests that mutate package state can't run in
parallel - they'd stomp on each other's fakes. If you're writing tests from an external
package (`package events_test`), the variable must be exported, which pollutes your public
API.

Prefer the [function parameter pattern] or the [interface pattern] over monkey patching.
Reserve this technique for legacy code where changing signatures would be too disruptive.

## Mocking a method on a type

This is a pattern you'll see all the time in services that integrate with third-party APIs.
Here's a payment service that charges customers through Stripe (this uses the newer
`stripe.Client` API, which is the recommended shape in recent stripe-go versions):

```go
func (s *Service) ChargeCustomer(
    ctx context.Context, custID string, cents int64) (string, error) {
    intent, err := s.client.V1PaymentIntents.Create(ctx,
        &stripe.PaymentIntentCreateParams{
            Amount:   stripe.Int64(cents),
            Currency: stripe.String("usd"),
            Customer: stripe.String(custID),
        },
    )
    if err != nil {
        return "", err
    }
    return intent.ID, nil
}
```

Testing this hits the real Stripe API. That's slow, requires live credentials, and in
production mode charges actual money. The problem is that `s.client` is a `*stripe.Client`
from the SDK - there's no way to swap it for a fake without introducing a seam.

The solution is to introduce an interface that describes what you need:

```go
type PaymentIntentCreator interface {  // (1)
    Create(
        context.Context, *stripe.PaymentIntentCreateParams,
    ) (*stripe.PaymentIntent, error)
}

type Service struct {
    intents PaymentIntentCreator  // (2)
}

func (s *Service) ChargeCustomer(
    ctx context.Context, custID string, cents int64) (string, error) {
    intent, err := s.intents.Create(ctx,  // (3)
        &stripe.PaymentIntentCreateParams{
            Amount:   stripe.Int64(cents),
            Currency: stripe.String("usd"),
            Customer: stripe.String(custID),
        })
    if err != nil {
        return "", err
    }
    return intent.ID, nil
}
```

Here:

- (1) the interface has one method matching what we need from the SDK
- (2) the service holds the dependency as a field
- (3) calls through the interface instead of the client directly

In production, inject the real Stripe service client:

```go
func main() {
    sc := stripe.NewClient("sk_test_...")
    svc := &Service{intents: sc.V1PaymentIntents}  // (1)
    // ...
}
```

Here:

- (1) `sc.V1PaymentIntents` satisfies `PaymentIntentCreator` (it has a `Create` method with
  the right signature)

In tests, you pass a fake that returns canned values:

```go
type fakeIntents struct {
    id string  // (1)
}

func (f *fakeIntents) Create(
    context.Context, *stripe.PaymentIntentCreateParams,
) (*stripe.PaymentIntent, error) {
    return &stripe.PaymentIntent{ID: f.id}, nil  // (2)
}

func TestChargeCustomer(t *testing.T) {
    fake := &fakeIntents{id: "pi_123"}  // (3)
    svc := &Service{intents: fake}
    id, _ := svc.ChargeCustomer(t.Context(), "cus_abc", 5000)
    // assert id == "pi_123"
}
```

Here:

- (1) the fake struct holds the canned return value
- (2) returns whatever you configured instead of calling Stripe
- (3) configure the fake with the expected payment intent ID

The service doesn't know or care whether it's talking to Stripe or a test fake. This is the
most common mocking pattern in Go - define an interface for your dependency, accept it in
your constructor, and swap implementations at runtime.

But what happens when the SDK surface area is huge and your code only needs one operation?
That's where the next pattern comes in.

## Consumer-side interface segregation

The previous pattern works well when you control the interface. But AWS SDK clients have
dozens of methods. The DynamoDB client has over 40 operations - `GetItem`, `PutItem`,
`Query`, `Scan`, `BatchGetItem`, and so on. If you write tests against a dependency that
exposes the whole surface area, your fakes become annoying fast.

The solution is to define a minimal interface on the consumer side:

```go
type itemGetter interface {  // (1)
    GetItem(context.Context, *dynamodb.GetItemInput,
        ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

func GetUserByID(
    ctx context.Context, client itemGetter, id string) (*User, error) {
    out, err := client.GetItem(ctx, &dynamodb.GetItemInput{  // (2)
        TableName: aws.String("users"),
        Key: map[string]types.AttributeValue{
            "pk": &types.AttributeValueMemberS{Value: id},
        },
    })
    // ...
}
```

Here:

- (1) the interface has exactly one method - just what this function needs
- (2) accept the minimal interface and call through it

In production, pass the real DynamoDB client - it satisfies `itemGetter` because it has a
`GetItem` method. Go interfaces are satisfied implicitly:

```go
func main() {
    client := dynamodb.NewFromConfig(cfg)
    user, err := GetUserByID(ctx, client, "user-123")  // (1)
    // ...
}
```

Here:

- (1) the real client satisfies `itemGetter` automatically - no adapter or wrapper needed
  thanks to implicit interface satisfaction

In tests, you only implement the one method you need:

```go
type fakeItemGetter struct {
    item map[string]types.AttributeValue  // (1)
}

func (f *fakeItemGetter) GetItem(context.Context, *dynamodb.GetItemInput,
    ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
    return &dynamodb.GetItemOutput{Item: f.item}, nil  // (2)
}

func TestGetUserByID(t *testing.T) {
    fake := &fakeItemGetter{
        item: map[string]types.AttributeValue{
            "email": &types.AttributeValueMemberS{Value: "a@b.com"},
        },
    }
    user, _ := GetUserByID(t.Context(), fake, "u-1")  // (3)
    // assert user.Email == "a@b.com"
}
```

Here:

- (1) the fake struct holds the canned response data
- (2) returns the configured item - no network call
- (3) pass the fake to the function under test

This is the [Interface Segregation Principle] in action - clients shouldn't be forced to
depend on methods they don't use.

But this approach has limits. If you have 20 functions each using different DynamoDB
operations, you'd end up with 20 tiny interfaces. And sometimes you're stuck with a
preexisting interface type that has more methods than you want. That's where struct
embedding helps.

## Struct embedding for partial implementation

Sometimes you can't define your own minimal interface. Maybe a library insists on a specific
interface type, and it's bigger than what your test cares about.

The AWS SDK v2's S3 upload manager is a good example. `manager.NewUploader` takes a client
interface that supports both single-part uploads and multipart uploads. If your test is
exercising the single-part path and you only want to intercept `PutObject`, implementing the
multipart methods just to satisfy the interface is pure busywork.

Go's struct embedding provides an escape hatch. Here's the production code:

```go
func UploadReport(
    ctx context.Context, client manager.UploadAPIClient,  // (1)
    bucket, key string, body io.Reader,
) error {
    up := manager.NewUploader(client)
    _, err := up.Upload(ctx, &s3.PutObjectInput{
        Bucket: aws.String(bucket),
        Key:    aws.String(key),
        Body:   body,
    })
    return err
}
```

Here:

- (1) accepts the SDK's `UploadAPIClient` interface - a large interface with many methods

In tests, embed the interface in your fake and override only what you need:

```go
type fakeS3 struct {
    manager.UploadAPIClient  // (1)
    gotKey  string
    gotBody []byte
}

func (f *fakeS3) PutObject(
    _ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
    if in.Key != nil {
        f.gotKey = *in.Key  // (2)
    }
    if in.Body != nil {
        f.gotBody, _ = io.ReadAll(in.Body)
    }
    return &s3.PutObjectOutput{}, nil
}

func TestUploadReport(t *testing.T) {
    fake := &fakeS3{}
    err := UploadReport(
        t.Context(),
        fake,  // (3)
        "my-bucket",
        "reports/q1.csv",
        bytes.NewReader([]byte("hi")),  // (4)
    )
    if err != nil {
        t.Fatal(err)
    }
    if fake.gotKey != "reports/q1.csv" {
        t.Errorf("got %q, want %q", fake.gotKey, "reports/q1.csv")
    }
}
```

Here:

- (1) embedding the interface satisfies the full interface at compile time
- (2) capture what you care about - only implement what this test needs
- (3) pass the fake to code that expects the full `UploadAPIClient` interface
- (4) use a small body so the upload manager takes the single `PutObject` path

The embedded interface value is `nil`, so any method you don't override will panic if
called. This is a feature, not a bug. If your code accidentally triggers multipart and calls
`CreateMultipartUpload`, the test crashes immediately, and you learn that your test setup
(or your assumptions) are wrong.

## Function type as interface

For interfaces with a single method, there's an even more compact approach. Say you have
middleware that validates authentication tokens:

```go
type ctxKey string

const userIDKey ctxKey = "userID"

type TokenValidator interface {  // (1)
    Validate(token string) (userID string, err error)
}

func RequireAuth(v TokenValidator, next http.Handler) http.Handler {  // (2)
    fn := func(w http.ResponseWriter, r *http.Request) {
        userID, err := v.Validate(r.Header.Get("Authorization"))
        if err != nil {
            http.Error(w, "unauthorized", 401)
            return
        }
        ctx := context.WithValue(r.Context(), userIDKey, userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
    return http.HandlerFunc(fn)
}
```

Here:

- (1) a single-method interface - the perfect candidate for a function type adapter
- (2) the middleware accepts the interface as a dependency

You could write a fake struct with a `Validate` method, but Go lets you define a function
type that satisfies the interface:

```go
type TokenValidatorFunc func(string) (string, error)  // (1)

func (f TokenValidatorFunc) Validate(token string) (string, error) {
    return f(token)  // (2)
}
```

Here:

- (1) define a function type with the right signature
- (2) add a method that just calls the function itself

This is the same pattern the standard library uses with `http.HandlerFunc`. Now tests can
pass inline functions:

```go
func TestRequireAuth(t *testing.T) {
    v := TokenValidatorFunc(func(token string) (string, error) {
        if token == "Bearer valid" {
            return "user-123", nil  // (1)
        }
        return "", errors.New("invalid")
    })
    next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
    handler := RequireAuth(v, next)  // (2)

    req := httptest.NewRequest("GET", "/protected", nil)
    req.Header.Set("Authorization", "Bearer valid")
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)
    // assert rec.Code == http.StatusOK
}
```

Here:

- (1) return a known user ID for a valid token
- (2) the middleware accepts it as a `TokenValidator` interface

No extra struct definitions cluttering up your test file.

## Mocking HTTP calls

When your code makes HTTP requests to external services, the `net/http/httptest` package
provides a test server that runs on localhost. Say you have a client that fetches exchange
rates:

```go
func (c *Client) GetRate(from, to string) (float64, error) {
    url := c.baseURL + "/latest?base=" + from + "&symbols=" + to
    resp, err := c.httpClient.Get(url)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    // decode JSON, return rate...
}
```

In production, `c.baseURL` points to the real API. Testing against it is problematic - it's
slow, requires credentials, returns different values each time, and might rate-limit your
CI.

The `httptest.Server` spins up a real HTTP server on localhost:

```go
func TestGetRate(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(  // (1)
        func(w http.ResponseWriter, r *http.Request) {
            fmt.Fprint(w, `{"base":"USD","rates":{"EUR":0.92}}`)
        },
    ))
    defer srv.Close()  // (2)

    client := NewClient(srv.URL, "key")  // (3)
    rate, _ := client.GetRate("USD", "EUR")

    // assert rate == 0.92
}
```

Here:

- (1) spin up a local HTTP server with a handler that returns canned JSON
- (2) shut down the server when the test finishes
- (3) point your client at `srv.URL` instead of the real API

Your code makes real HTTP calls over TCP, but they never leave the machine. You can return
different responses for different scenarios - rate limits, malformed JSON, network errors -
whatever you need to test.

## Mocking time

This is essentially the same technique as [Mocking a function] - we're just applying it to
`time.Now`. Code that depends on the current time is tricky to test:

```go
func IsExpired(expiresAt time.Time) bool {
    return time.Now().After(expiresAt)
}
```

Every call to `time.Now()` returns a different value. You can't write a reliable test
because the result depends on when the test runs.

Make the clock injectable:

```go
type Clock func() time.Time  // (1)

func IsExpired(expiresAt time.Time, clock Clock) bool {  // (2)
    return clock().After(expiresAt)
}
```

Here:

- (1) define a function type for getting the current time
- (2) accept it as a parameter

In production, pass `time.Now`:

```go
func main() {
    expired := IsExpired(token.ExpiresAt, time.Now)  // (1)
    // ...
}
```

Here:

- (1) pass the real `time.Now` function - it satisfies the `Clock` type

In tests, pass a function that returns a fixed time:

```go
func TestIsExpired(t *testing.T) {
    expiry := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

    before := func() time.Time { return expiry.Add(-time.Hour) }  // (1)
    after := func() time.Time { return expiry.Add(time.Hour) }   // (2)

    if IsExpired(expiry, before) {
        t.Error("should not be expired")
    }
    if !IsExpired(expiry, after) {
        t.Error("should be expired")
    }
}
```

Here:

- (1) a clock that returns one hour before expiry
- (2) a clock that returns one hour after expiry

For code that uses `time.Sleep`, timers, or tickers, Go 1.25's [testing/synctest] provides a
fake clock that advances automatically when goroutines in the bubble are [durably blocked]:

```go
func TestPeriodicFlush(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {  // (1)
        count := 0
        go func() {
            ticker := time.NewTicker(10 * time.Second)  // (2)
            defer ticker.Stop()
            for range ticker.C {
                count++
                if count >= 3 {
                    return
                }
            }
        }()
        time.Sleep(35 * time.Second)  // (3)
        synctest.Wait()               // (4)
        // assert count == 3
    })
}
```

Here:

- (1) `synctest.Test` runs the function in an isolated bubble with fake time starting at
  2000-01-01
- (2) the ticker uses fake time - no real 10-second waits
- (3) `time.Sleep` inside the bubble uses fake time; time advances when goroutines are
  durably blocked, so this returns instantly after the ticker fires 3 times
- (4) `synctest.Wait` is a synchronization point; it blocks until the other goroutines in
  the bubble are durably blocked or finished

Inside `synctest.Test`, the framework intercepts time operations. The test completes
instantly rather than waiting for real time to pass.

## Closing words

These are the most common ones where I typically avoid opting for mocking libraries. But
there are cases when I still like to generate mocks for an interface. One example that comes
to mind is testing gRPC servers. I’m sure I’m forgetting some other cases where I regularly
use mocking libraries.

The point is not to discourage the use of mocking libraries or to make a general statement
that "all mocking libraries are bad." It’s that these mocking libraries have costs
associated with them. Code generation is fun, but it's one extra step that you have to teach
someone who’s onboarding to your codebase.

Also, if you're using LLMs to generate tests, you may want to write some tests manually to
give the tool a sense of how you want your tests written, so it doesn't pull in the universe
just to mock something that can be mocked natively using Go constructs.

For more on why handwritten fakes often beat generated mocks, see [Test state, not
interactions].

<!-- references -->

<!-- prettier-ignore-start -->

[gomock]:
    https://github.com/uber-go/mock

[mockery]:
    https://github.com/vektra/mockery

[spooky action at a distance]:
    https://en.wikipedia.org/wiki/Action_at_a_distance_\(computer_programming\)

[mocking a method on a type]:
    #mocking-a-method-on-a-type

[mocking a function]:
    #mocking-a-function

[function parameter pattern]:
    #mocking-a-function

[interface pattern]:
    #mocking-a-method-on-a-type

[test state, not interactions]:
    /go/test-state-not-interactions/

[interface segregation principle]:
    /go/interface-segregation/

[testing/synctest]:
    https://pkg.go.dev/testing/synctest

[durably blocked]:
    https://pkg.go.dev/testing/synctest#hdr-Blocking

<!-- prettier-ignore-end -->
