---
title: Is passing user ID through context an antipattern?
date: 2026-03-18
slug: user-id-through-context
tags:
    - Go
    - API
    - Distributed Systems
description: >-
  Why the middleware-to-handler boundary is a special case for context values.
---

[Paweł Grzybek] reached out after reading [What belongs in Go's context values?] with a
question about their auth [middleware] and the [handler] that consumes the user ID it sets:

> I validate the session in middleware, and the session record in the DB holds the user ID,
> which I put in the context for handlers to use later. According to your post, this is an
> antipattern because the handler can't work without that value. But if I don't use context
> here, I'd have to hit the sessions table again in the handler. Is this actually wrong,
> and if so, how do I avoid the double DB lookup?

---

The litmus test from the previous shard says:

> If your code cannot proceed without some value, that value should not go in a context.

The reader's handler cannot create a resource without the user ID. So on a strict reading,
this looks like it fails the test.

But the test is about function signatures you control. When you write a regular Go function,
you can put `userID uuid.UUID` in its parameter list, and any caller knows the function
requires it. The middleware-to-handler boundary in `net/http` is different. Your handler is
always `func(http.ResponseWriter, *http.Request)`. You can't add parameters to it. Context is
how `net/http` middleware passes data to handlers, and that's by design.

The middleware already has to look up the session token to verify the request is authenticated.
The user ID comes out of that same lookup. Passing it along through context avoids a second
round trip to the DB for something the middleware already resolved.

The previous shard listed "authentication tokens for middleware propagation" as
context-appropriate. A user ID extracted from a validated session token is the resolved form
of that authentication. It's request-scoped, it comes from the auth layer, and every request
that reaches the handler has one because the middleware enforced it. It fits.

Their [middleware] does this:

```go
// internal/middlewares/auth.go

ctx := context.WithValue(
    r.Context(), config.UserIDContextKey, token.UserID)
next.ServeHTTP(w, r.WithContext(ctx))
```

And the [handler] consumes it:

```go
// internal/handlers/resourcesCreate.go

userID, _ := utils.UserIDFromContext(r.Context())
```

The middleware looks up the session once, stashes the user ID in context, and the handler reads
it from there.

If context weren't an option, another way to avoid the repeated DB hit would be to cache the
session behind something like Redis. Multiple cache lookups are cheaper than multiple DB
calls. But for this case that's overkill, and you'd still pay the cost of a TCP round trip
per lookup if the cache lives out of process.

<!-- references -->
<!-- prettier-ignore-start -->

[Paweł Grzybek]:
    https://bsky.app/profile/pawelgrzybek.com

[What belongs in Go's context values?]:
    /shards/2026/03/what-belongs-in-go-context-values/

[middleware]:
    https://github.com/hreftools/api/blob/2705fea8b1a508e00d35d248f16c063de353ef2d/internal/middlewares/auth.go#L62

[handler]:
    https://github.com/hreftools/api/blob/2705fea8b1a508e00d35d248f16c063de353ef2d/internal/handlers/resourcesCreate.go#L53

<!-- prettier-ignore-end -->
