---
title: Let the domain guide your application structure
date: 2025-09-20
tags:
    - Go
---

I like to make the distinction between application structure and architecture. Structure is
how you organize the directories and packages in your app while architecture is how
different components talk to each other. The way your app talks to other services in a fleet
can also be called architecture.

While structure often influences architecture and vice versa, this distinction is important.
This post is strictly about application structure and not library structure. Library
structure is often driven by different design pressures than their app counterparts. There
are a ton of canonical examples of good library structure in the stdlib, but it's app
structure where things get a bit more muddy.

At work, I not only write Go in a distributed system environment but also review potential
candidates' assignments in the hiring pipeline. While there is no objectively right or wrong
way to structure an app, I do see a common pitfall in candidates' submissions that is
usually frowned upon in a Go application.

> _App structure should be driven by what it does and not what it's built with. Let the
> domain guide the structure, not technology or the current language specific zeitgeist._

Ben Johnson's [Standard Package Layout] is a good reference for this. He points out why
approaches like monolithic packages, Rails style layouts, or grouping by module don't fit
well in Go. Then he lays out a map where the root package holds domain types, dependencies
are grouped in separate packages, and the main package wires everything together.

While Ben's post is focused on what you should be doing, I want to keep this discussion a
bit more open-ended and just talk about one bad pattern that you probably should avoid. The
rest of the app structure is subjective and should be driven by requirements. Use your
judgement.

The mistake I often see is people making a bunch of generically named packages like
`models`, `controllers`, `handlers` and stuffing everything there. App structure like the
following is quite common:

```
mystore/
├── controllers/
│   ├── order_controller.go
│   └── user_controller.go.go
├── models/
│   ├── order.go
│   └── user.go.go
├── handlers/
│   ├── http_handler.go
│   └── webhook_handler.go
└── main.go
```

In Go there's no file level separation, only package level separation. That means everything
under `models` like order and product lives in the same namespace. The same is true for
`controllers` and `handlers`.

Once you put multiple business domains under a generic umbrella, you tie them together. This
might make sense in a language like Python where file names are prefixed in the fully
qualified import path. In Python you'd import them as follows:

```py
# Identifiers live in the order namespace
from mystore.models import order

# Identifiers live in the http_handler namespace
from mystore.handlers import http_handler
```

But in Go the import path becomes this:

```go
// Identifiers from order.go, user.go, product.go
// all live in the same namespace
import "mystore/models"

// Identifiers from http_handler.go & webhook_handler.go
// all live in the same namespace
import "mystore/handlers"
```

There is no file level delineation in Go. If you put different domains under the same
`models` directory, there is no indication at import time what domain a model belongs to.
The only clue is the identifier name. This is not ideal when you want clear separation
between domains.

> _In Go, packages define your [bounded context], not files within a package. Domains should
> be delineated by top level packages, not by file names._

For your top level business logic, you want package level separation between domains. Order
logic should live in `order`, user logic should live in `user`. These packages will be
imported in many places throughout the app, and keeping them separate keeps dependencies
clear.

It could look like this:

```
mystore/
├── order/          <-- business logic related to the order domain
│   ├── order.go
│   └── service.go
├── user/           <-- business logic related to the user domain
│   ├── user.go
│   └── service.go
└── cmd/            <-- wire everything here
    └── mystore/
        └── main.go
```

Each domain owns its own logic and optional adapters. If you need to find order related
code, you go to `order`. If you need user code, you go to `user`. Nothing is smooshed
together under a generic bucket.

The details around how you layer your app can differ based on requirements, but the
important point is that your top level directories should not just be generic buckets
containing all domains. That makes navigation harder. A better approach is letting the
domain guide the structure and only layering in technology when it matters.

You can place your transport concerns alongside the top level packages. A top level `http`
package can hold handlers that import service functions from the domain packages. You can
put all handlers under `http` or split them into `http/order` and `http/user`. Both are
valid choices. If you put all handlers under `http`, that's fine because they are usually
imported in one place where you wire routes. The same is true for database adapters. You can
put them all under `postgres` or split them into `postgres/order` and `postgres/user`. Both
patterns are acceptable. The key difference is that domains need package level separation,
while technology packages can be grouped because they are only wired at the edge.

```txt
mystore/
├── order/
│   ├── order.go
│   └── service.go
├── user/
│   ├── user.go
│   └── service.go
├── http/                 <-- lumping all the handlers here is fine
│   ├── order_handler.go
│   └── user_handler.go
├── postgres/             <-- this is fine, but you can create sub pkgs too
│   ├── order_repo.go
│   └── user_repo.go
└── cmd/
    └── server/
        └── main.go
```

But depending on the complexity of your app, this is also absolutely fine:

```txt
mystore/
├── order/
│   ├── order.go
│   └── service.go
├── user/
│   ├── user.go
│   └── service.go
├── http/                 <-- handlers are split by domain here
│   ├── order/
│   │   └── handler.go
│   └── user/
│       └── handler.go
├── postgres/             <-- repos are split by domain here
│   ├── order/
│   │   └── repo.go
│   └── user/
│       └── repo.go
└── cmd/
    └── server/
        └── main.go
```

The rule of thumb is that top level domains should never import anything from technology
folders like `http` or `postgres`. Instead, `http` and `postgres` should always import from
domain packages. You can add a linter to enforce this rule but since Go does not allow
import cycles, this is automatically enforced by the compiler.

```
   +-----------+     +-----------+
   |   order   |     |   user    |
   +-----------+     +-----------+
          ^                ^
          |                |
   +------------------------------+
   | http              postgres   |
   +------------------------------+
                  ^
                  |
             +---------+
             |   cmd   |
             +---------+
```

Domains sit at the top. Technology packages depend on them, never the other way around. The
`cmd` package wires everything together. This keeps the graph simple and keeps domains
independent.

---

Astute readers might notice that I have left out any discussion around the `internal`
directory. This is intentional. Depending on your requirements, you might opt in for an
`internal` directory or not. This isn't important for our discussion. The main point I
wanted to emphasize is that technology or architecture patterns should not guide your app
structure. It should be based on something more persistent and nothing is more persistent
than your application's domain.

<!-- References -->

<!-- prettier-ignore-start -->

[standard package layout]:
    https://www.gobeyond.dev/standard-package-layout/

[bounded context]:
    https://martinfowler.com/bliki/BoundedContext.html

<!-- prettier-ignore-end -->
