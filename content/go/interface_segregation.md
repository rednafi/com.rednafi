---
title: Revisiting interface segregation in Go
date: 2025-11-01
slug: interface-segregation
tags:
    - Go
---

Object-oriented (OO) patterns get a lot of flak in the Go community, and often for good
reason.

Still, I've found that principles like [SOLID], despite their OO origin, can be useful
guides when thinking about design in Go.

Recently, while chatting with a few colleagues new to Go, I noticed that some of them had
spontaneously rediscovered the Interface Segregation Principle (the "I" in SOLID) without
even realizing it. The benefits were obvious, but without a shared vocabulary, it was harder
to talk about and generalize the idea.

So I wanted to revisit ISP in the context of Go and show how [small interfaces], [implicit
implementation], and [consumer-defined contracts] make interface segregation feel natural
and lead to code that's easier to test and maintain.

> _Clients should not be forced to depend on methods they do not use._
>
> _— Robert C. Martin (SOLID, interface segregation principle)_

Or, put simply: your code shouldn't accept anything it doesn't use.

Consider this example:

```go
type FileStorage struct{}

func (FileStorage) Save(data []byte) error {
    fmt.Println("Saving data to disk...")
    return nil
}

func (FileStorage) Load(id string) ([]byte, error) {
    fmt.Println("Loading data from disk...")
    return []byte("data"), nil
}
```

`FileStorage` has two methods: `Save` and `Load`. Now suppose you write a function that only
needs to save data:

```go
func Backup(fs FileStorage, data []byte) error {
    return fs.Save(data)
}
```

This works, but there are a few problems hiding here.

`Backup` takes a `FileStorage` directly, so it only works with that type. If you later want
to back up to memory, a network location, or an encrypted store, you'll need to rewrite the
function. Because it depends on a concrete type, your tests have to use `FileStorage` too,
which might involve disk I/O or other side effects you don't want in unit tests. And from
the function signature, it's not obvious what part of `FileStorage` the function actually
uses.

Instead of depending on a specific type, we can depend on an abstraction. In Go, you can
achieve that through an interface. So let's define one:

```go
type Storage interface {
    Save(data []byte) error
    Load(id string) ([]byte, error)
}
```

Now `Backup` can take a `Storage` instead:

```go
func Backup(store Storage, data []byte) error {
    return store.Save(data)
}
```

`Backup` now depends on behavior, not implementation. You can plug in anything that
satisfies `Storage`, something that writes to disk, memory, or even a remote service. And
`FileStorage` still works without any change.

You can also test it with a fake:

```go
type FakeStorage struct{}

func (FakeStorage) Save(data []byte) error         { return nil }
func (FakeStorage) Load(id string) ([]byte, error) { return nil, nil }

func TestBackup(t *testing.T) {
    fake := FakeStorage{}
    err := Backup(fake, []byte("test-data"))
    if err != nil {
        t.Fatal(err)
    }
}
```

That's a step forward. It fixes the coupling issue and makes the tests free of side effects.
However, there's still one issue: `Backup` only calls `Save`, yet the `Storage` interface
includes both `Save` and `Load`. If `Storage` later gains more methods, every fake must grow
too, even if those methods aren't used. That's exactly what the ISP warns against.

The above interface is too broad. So let's narrow it to match what the function actually
needs:

```go
type Saver interface {
    Save(data []byte) error
}
```

Then update the function:

```go
func Backup(s Saver, data []byte) error {
    return s.Save(data)
}
```

Now the intent is clear. `Backup` only depends on `Save`. A test double can just implement
that one method:

```go
type FakeSaver struct{}

func (FakeSaver) Save(data []byte) error { return nil }

func TestBackup(t *testing.T) {
    fake := FakeSaver{}
    err := Backup(fake, []byte("test-data"))
    if err != nil {
        t.Fatal(err)
    }
}
```

The original `FileStorage` still works fine:

```go
fs := FileStorage{}
_ = Backup(fs, []byte("backup-data"))
```

Go's implicit interface satisfaction makes this less ceremonious. Any type with a `Save`
method automatically satisfies `Saver`.

This pattern reflects a broader Go convention: define small interfaces on the consumer side,
close to the code that uses them. The consumer knows what subset of behavior it needs and
can define a minimal contract for it. If you define the interface on the producer side
instead, every consumer is forced to depend on that definition. A single change to the
producer's interface can ripple across your codebase unnecessarily.

From Go [code review comments]:

> _Go interfaces generally belong in the package that uses values of the interface type, not
> the package that implements those values. The implementing package should return concrete
> (usually pointer or struct) types: that way, new methods can be added to implementations
> without requiring extensive refactoring._

This isn't a strict rule. The standard library defines producer-side interfaces like
`io.Reader` and `io.Writer`, which is fine because they're stable and general-purpose. But
for application code, interfaces usually exist in only two places: production code and
tests. Keeping them near the consumer reduces coupling between multiple packages and keeps
the code easier to evolve.

You'll see this same idea pop up all the time. Take the AWS SDK, for example. It's tempting
to define a big S3 client interface and use it everywhere:

```go
type S3Client interface {
    PutObject(
        ctx context.Context,
        input *s3.PutObjectInput,
        opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)

    GetObject(
        ctx context.Context,
        input *s3.GetObjectInput,
        opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)

    ListObjectsV2(
        ctx context.Context,
        input *s3.ListObjectsV2Input,
        opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)

    // ...and many more
}
```

Depending on such a large interface couples your code to far more than it uses. Any change
or addition to this interface can ripple through your code and tests for no good reason.

For example, if your code uploads files, it only needs the `PutObject` method:

```go
func UploadReport(ctx context.Context, client S3Client, data []byte) error {
    _, err := client.PutObject(
        ctx,
        &s3.PutObjectInput{
            Bucket: aws.String("reports"),
            Key:    aws.String("daily.csv"),
            Body:   bytes.NewReader(data),
        },
    )
    return err
}
```

But accepting the full `S3Client` here ties `UploadReport` to an interface that's too broad.
A fake must implement all the methods just to satisfy it.

It's better to define a small, consumer-side interface that captures only the operations you
need. This is exactly what the [AWS SDK doc] recommends for testing.

> _To support mocking, use Go interfaces instead of concrete service client, paginators, and
> waiter types, such as s3.Client. This allows your application to use patterns like
> dependency injection to test your application logic._

Similar to what we've seen before, you can define a single method interface:

```go
type Uploader interface {
    PutObject(
        ctx context.Context,
        input *s3.PutObjectInput,
        opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}
```

And then use it in the function:

```go
func UploadReport(ctx context.Context, u Uploader, data []byte) error {
    _, err := u.PutObject(
        ctx,
        &s3.PutObjectInput{
            Bucket: aws.String("reports"),
            Key:    aws.String("daily.csv"),
            Body:   bytes.NewReader(data),
        },
    )
    return err
}
```

The intent is obvious: this function uploads data and depends only on `PutObject`. The fake
for tests is now tiny:

```go
type FakeUploader struct{}

func (FakeUploader) PutObject(
    _ context.Context,
    _ *s3.PutObjectInput,
    _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
    return &s3.PutObjectOutput{}, nil
}
```

If we distill the workflow as a general rule of thumb, it'd look like this:

> _Insert a seam between two tightly coupled components by placing a consumer-side interface
> that exposes only the methods the caller invokes._

Fin!

<!-- References -->

<!-- prettier-ignore-start -->

[solid]:
    https://en.wikipedia.org/wiki/SOLID

[small interfaces]:
    https://go.dev/doc/effective_go#interfaces_and_types:~:text=Interfaces%20with%20only%20one%20or%20two%20methods%20are%20common%20in%20Go%20code%2C%20and%20are%20usually%20given%20a%20name%20derived%20from%20the%20method%2C%20such%20as%20io.Writer%20for%20something%20that%20implements%20Write

[implicit implementation]:
    https://go.dev/tour/methods/10

[consumer-defined contracts]:
    https://go.dev/wiki/CodeReviewComments#interfaces:~:text=Go%20interfaces%20generally,requiring%20extensive%20refactoring

[code review comments]:
    https://go.dev/wiki/CodeReviewComments#interfaces:~:text=Go%20interfaces%20generally,the%20real%20implementation

[aws sdk doc]:
    https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/unit-testing.html#:~:text=To%20support%20mocking%2C%20use%20Go%20interfaces%20instead%20of%20concrete%20service%20client%2C%20paginators%2C%20and%20waiter%20types%2C%20such%20as%20s3.Client.%20This%20allows%20your%20application%20to%20use%20patterns%20like%20dependency%20injection%20to%20test%20your%20application%20logic.

<!-- prettier-ignore-end -->
