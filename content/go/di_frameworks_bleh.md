---
title: You probably don't need a DI framework
date: 2025-05-24
tags:
    - Go
---

When working with Go in an [industrial] context, I feel like dependency injection (DI) often
gets a bad rep because of _DI frameworks_. But DI as a technique is quite useful. It just
tends to get explained with too many OO jargons and triggers PTSD among those who came to Go
to escape GoF theology.

> _Dependency Injection is a 25-dollar term for a 5-cent concept._
>
> _— James Shore_

DI basically means _passing values into a constructor instead of creating them inside it_.
That's really it. Observe:

```go
type server struct {
    db DB
}

// NewServer constructs a server instance
func NewServer() *server {
    db := DB{}            // The dependency is created here
    return &server{db: db}
}
```

Here, `NewServer` creates its own `DB`. Instead, to inject the dependency, build `DB`
elsewhere and pass it in as a constructor parameter:

```go
func NewServer(db DB) *server {
    return &server{db: db}
}
```

Now the constructor no longer decides how a database is built; it simply _receives_ one.

In Go, DI is often done using interfaces. You collate the behavior you care about in an
interface, and then provide different concrete implementations for different contexts. In
production, you pass a real implementation of `DB`. In unit tests, you pass a fake
implementation that behaves the same way from the caller's perspective but avoids real
database calls.

Here's how that looks:

```go
// behaviour we care about
type DB interface {
    Get(id string) (string, error)
    Save(id, value string) error
}

type server struct{ db DB }

// NewServer accepts a concrete implementation of the DB interface in runtime
// and passes it to the server struct.
func NewServer(db DB) *server { return &server{db: db} }
```

A real implementation of `DB` might look like this:

```go
type RealDB struct{ url string }

func NewDB(url string) *RealDB { return &RealDB{url: url} }

func (r *RealDB) Get(id string) (string, error) {
    // pretend we hit Postgres
    return "real value", nil
}
func (r *RealDB) Save(id, value string) error { return nil }
```

And a fake implementation for unit tests might be:

```go
type FakeDB struct{ data map[string]string }

func NewFake() *FakeDB { return &FakeDB{data: map[string]string{}} }

func (f *FakeDB) Get(id string) (string, error) { return f.data[id], nil }
func (f *FakeDB) Save(id, value string) error   { f.data[id] = value; return nil }
```

Use the fake in unit tests like so:

```go
func TestServerGet(t *testing.T) {
    fake := NewFake()
    _    = fake.Save("42", "fake")

    srv := NewServer(fake)
    val, _ := srv.db.Get("42")

    if val != "fake" {
        t.Fatalf("want fake, got %s", val)
    }
}
```

The compiler guarantees both `RealDB` and `FakeDB` satisfy `DB`, and during tests, we can
swap out the implementations without much ceremony.

## Why frameworks turn mild annoyance into actual pain

Once `NewServer` grows half a dozen dependencies, wiring them by hand can feel noisy. That's
when a DI framework starts looking tempting.

With Uber's [dig], you register each constructor as a _provider_. `Provide` takes a
function, uses reflection to inspect its parameters and return type, and adds it as a node
in an internal dependency graph. Nothing is executed yet. Things only run when you call
`.Invoke()` on the container.

But that reflection-driven magic is also where the pain starts. As your graph grows, it gets
harder to tell which constructor feeds which one. Some constructor take one parameter, some
take three. There's no single place you can glance at to understand the wiring. It's all
figured out inside the container at runtime.

> _Let the container figure it out!_
>
> _— every DI framework ever_

```go
func BuildContainer() *dig.Container {
    c := dig.New()
    // Each Provide call teaches dig about one node in the graph.
    c.Provide(NewConfig)     // produces *Config
    c.Provide(NewDB)         // wants *Config, produces *DB
    c.Provide(NewRepo)       // wants *DB, produces *Repo
    c.Provide(NewFlagClient) // produces *FlagClient
    c.Provide(NewService)    // wants *Repo, *FlagClient, produces *Service
    c.Provide(NewServer)     // wants *Service, produces *server
    return c
}

func main() {
    // Invoke kicks off the whole graph. dig topologically sorts, calls each
    // constructor, and finally hands *server to your callback.
    if err := BuildContainer().Invoke(
    	func(s *server) { s.Run() }); err != nil {
    	panic(err)
    }
}
```

Now try commenting out `NewFlagClient`. The code still [compiles]. There's no error until
runtime, when dig fails to construct `NewService` due to a missing dependency. And the error
message you get?

```txt
dig invoke failed: could not build arguments for function
        main.main.func1 (prog.go:87)
    : failed to build *main.Server
    : could not build arguments for function main.NewServer (prog.go:65)
    : failed to build *main.Service: missing dependencies for function
        main.NewService (prog.go:55)
    : missing type: *main.FlagClient
```

That's five stack frames deep, far from where the problem started. Now you're digging
through dig's internals to reconstruct the graph in your head.

Google's [wire] takes a different approach: it shifts the graph-building to _code
generation_. You collect your constructors in a `wire.NewSet`, call `wire.Build`, and the
generator writes a `wire_gen.go` that wires everything up explicitly.

```go
var serverSet = wire.NewSet(
    NewConfig,
    NewDB,
    NewRepo,
    NewFlagClient,   // comment this line out to see Wire complain at compile time
    NewService,
    NewServer,
)

func InitializeServer() (*server, error) {
    wire.Build(serverSet)
    return nil, nil // replaced by generated code
}
```

Comment out `NewFlagClient` and Wire fails earlier—during generation:

```txt
wire: ../../service/wire.go:13:2: cannot find dependency for *flags.Client
```

It's better than dig's runtime panic, but still comes with its own headaches:

- You need to remember to run `go generate ./...` whenever constructor signatures change.
- When something breaks, you're stuck reading through hundreds of lines of autogenerated
  glue to trace the issue.
- You have to teach every teammate Wire's DSL—`wire.NewSet`, `wire.Build`, build tags, and
  sentinel rules. And if you ever switch to something different like dig, you'll need to
  learn a completely different set of concepts: `Provide`, `Invoke`, scopes, named values,
  etc.

While DI frameworks tend to use vocabularies like _provider_ or _container_ to give you an
essense of familiarity, they still reinvent the API surface every time. Switching between
them means relearning a new mental model.

So the promise of "just register your providers and forget about wiring" ends up trading
clear, compile-time control for either reflection or hidden generator logic—and yet another
abstraction layer you have to debug.

## The boring alternative: keep wiring explicit

In Go, you can just wire your own dependencies manually. Like this:

```go
func main() {
    cfg := NewConfig()

    db    := NewDB(cfg.DSN)
    repo  := NewRepo(db)
    flags := NewFlagClient(cfg.FlagURL)

    svc := NewService(repo, flags, cfg.APIKey)
    srv := NewServer(svc, cfg.ListenAddr)

    srv.Run()
}
```

Longer? Yes. But:

- The call order is the dependency graph.
- Errors are handled right where they happen.
- If a constructor changes, the compiler points straight at every broken call:

    ```txt
    ./main.go:33:39: not enough arguments in call to NewService
        have (*Repo, *FlagClient)
        want (*Repo, *FlagClient, string)
    ```

No reflection, no generated code, no global state. Go type-checks the dependency graph early
and loudly, exactly how it should be.

If `main()` really grows unwieldy, split _your_ code:

```go
func buildInfra(cfg *Config) (*DB, *FlagClient, error) {
    // ...
}

func buildService(cfg *Config) (*Service, error) {
    db, flags, err := buildInfra(cfg)
    if err != nil { return nil, err }
    return NewService(NewRepo(db), flags, cfg.APIKey), nil
}

func main() {
    cfg := NewConfig()
    svc, err := buildService(cfg)
    if err != nil { log.Fatal(err) }
    NewServer(svc, cfg.ListenAddr).Run()
}
```

Each helper is a regular function that anyone can skim without reading a framework manual.
Also, you usually build all of your dependency in one place and it's really not that big of
a deal if your builder function takes in 20 parameters and builds all the dependencies. Just
put each function parameter on their own line and use gofumpt to format the code to make it
readable.

## Reflection works elsewhere, so why not here?

Other languages lean on containers because often times constructors cannot be overloaded and
compile times hurt. Go already gives you:

- First-class functions so constructors are plain values.
- Interfaces so implementations swap cleanly in tests.
- Fast compilation so feedback loops stay tight.

A DI framework often fixes problems Go already solved and trades away readability to do it.

> _The most magical thing about Go is how little magic it allows._
>
> _— Some Gopher on Reddit_

## You might still want a framework

It's tempting to make a blanket statement saying that you should **never** pick up a DI
framework, but context matters here.

I was watching Uber's [talk] on how they use Go and how their DI framework [Fx] (which uses
dig underneath) allows them to achieve consistency at scale. If you're Uber and have all the
observability tools in place to get around the downsides, then you'll know.

Also, if you're working in a codebase that's already leveraging a framework and it works
well, then it doesn't make sense to refactor it without any incentives.

Or, you're writing one of those languages where using a DI framework is the norm, and
you'll be called a weirdo if you try to reinvent the wheel there.

However, in my experience, even in organizations that maintain a substantial number of Go
repos, DI frameworks add more confusion than they're worth. If your experience is otherwise,
I'd love to be proven wrong.

---

The post got a fair bit of discussion going around the web. You might find it interesting.

- [r/golang]
- [r/experienceddevs]
- [r/programming]


[industrial]: https://peter.bourgon.org/go-for-industrial-programming/
[dig]: https://github.com/uber-go/dig
[compiles]: https://go.dev/play/p/Vhimup7ukLo
[wire]: https://github.com/google/wire
[talk]: https://www.youtube.com/watch?v=nLskCRJOdxM&t
[fx]: https://github.com/uber-go/fx

[r/golang]: https://www.reddit.com/r/golang/comments/1kv0y1u/you_probably_dont_need_a_di_framework/
[r/programming]: https://www.reddit.com/r/programming/comments/1kv0y2l/you_probably_dont_need_a_di_framework/
[r/experienceddevs]: https://www.reddit.com/r/ExperiencedDevs/comments/1kv0y3n/you_probably_dont_need_a_di_framework/
