---
atprotoPath: /go/request-coalescing/
date: 2026-06-27T00:00:00Z
description: A hot cache key expires and a hundred requests issue the same query at once, saturating the database. Go's singleflight package coalesces those duplicate calls into one. How to wire it up, how to measure whether it's firing, and why per-pod coalescing is usually enough.
mermaid: true
slug: request-coalescing
tags:
    - Go
    - Concurrency
    - Distributed Systems
title: Request coalescing with Go singleflight
discussions:
    - label: Reddit
      url: https://www.reddit.com/r/golang/comments/1uhqvyx/request_coalescing/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mpcm52jalc2z"
---

Say you put a cache in front of Postgres to speed up reads. A hot key expires:

- the next request misses the cache, so it queries Postgres to refill the key
- the key is popular, so while that first query runs, a hundred more pile in for it
- they all miss too, and each fires its own query
- Postgres ends up running a hundred identical queries at once, all for the same value

<!-- prettier-ignore-start -->

{{< mermaid >}}
sequenceDiagram
    participant U1 as User 1
    participant UN as User N
    participant App as Application
    participant DB as Database
    U1->>App: Get Product 123
    UN->>App: Get Product 123
    App->>DB: SELECT * FROM products WHERE id=123
    App->>DB: SELECT * FROM products WHERE id=123
    DB-->>App: Product data
    DB-->>App: Product data
    App-->>U1: Product data
    App-->>UN: Product data
    Note over App,DB: N identical queries!
{{</ mermaid >}}

<!-- prettier-ignore-end -->

That's known as a [thundering herd] or [cache stampede]. I've also heard people call it
dog-piling. Every request wants the same value, yet each one still fires its own query. It's
wasted work, and it pounds your database for nothing.

Worse, a stampede can feed itself. The query flood slows the database, slow queries time
out, clients retry, and the retries heap on even more load, so the overload outlives the
spike that set it off. Marc Brooker shows how this can lead to a metastable failure in
[Caches, Modes, and Unstable Systems].

Request coalescing aims to fix that. Another name for it is [request collapsing]. The first
caller runs the query; everyone else waits on it and gets the same result.

You'll want it anywhere a crowd of callers needs the same expensive value at once:

- an access token expires, and every in-flight request tries to refresh it at once
- a config value gets evicted, and every worker reloads it
- a thousand goroutines resolve the same hostname at the same time

## Suppressing duplicate calls

Coalescing keeps a table of the calls in flight, one entry per key. When a caller asks for a
key that's already running, it doesn't start a second call. It waits on the one in flight
and takes the result.

Go's [golang.org/x/sync/singleflight] package does this for you. Create a `Group`, then call
`Do` with a key and a function:

```go
import "golang.org/x/sync/singleflight"

// zero value is ready to use; share one across goroutines
var g singleflight.Group

v, err, shared := g.Do(key, func() (any, error) {
    return fetch(ctx, key) // expensive upstream call to dedup
})
```

`Do` runs the function at most once per key at a time. Call it with a key that's already in
flight and it won't run the function again. It blocks until that first call returns, then
hands you the same `v` and `err`.

`v` comes back as `any`, so you type-assert it to its real type. The third return value,
`shared`, tells you whether the result went to more than one caller. You'll use that for
metrics later.

## On a cache miss

`Do` only deduplicates calls that overlap in time, so it pairs with a cache.

The usual setup is [cache-aside]. You read the cache first, and on a miss you fetch the
value and store it. Wrap just the fetch in `Do`, and concurrent misses coalesce into one
call per key.

Here `s.fetch` is the upstream call: a database query, or an RPC to another service. The
cache itself has to be safe for concurrent use.

```go
func (s *Store) Get(ctx context.Context, key string) (string, error) {
    if v, ok := s.cache.Get(key); ok { // (1)
        return v, nil
    }
    v, err, _ := s.group.Do(key, func() (any, error) { // (2)
        val, err := s.fetch(ctx, key)
        if err != nil {
            return "", err
        }
        s.cache.Set(key, val) // (3)
        return val, nil
    })
    if err != nil {
        return "", err
    }
    return v.(string), nil // (4)
}
```

- (1) a cache hit returns straight away, without touching `Do`
- (2) a miss is the only place a herd can form, so it's the only thing `Do` wraps; the first
  caller per key runs `fetch` while other concurrent callers wait
- (3) the first caller stores the result in cache before it returns, so later reads hit the
  cache and skip `Do`
- (4) `Do` returns `any`, so you assert the value back to a `string`

> [!NOTE]
>
> Singleflight sits on the cache's miss path. When callers pile onto a miss, it runs one
> call for all of them, then purges the key the moment that call returns.

By the time the next wave of requests shows up, the cache is warm and they never reach the
group.

With `Do` wrapping the fetch, the first caller runs the query and the rest wait:

<!-- prettier-ignore-start -->

{{< mermaid >}}
sequenceDiagram
    participant U1 as User 1
    participant UN as User N
    participant App as Application
    participant DB as Database
    U1->>App: Get Product 123
    activate App
    Note over App: singleflight Do(key) runs the fetch once
    App->>DB: SELECT * FROM products WHERE id=123
    UN->>App: Get Product 123
    Note over App: User N joins the in-flight Do(key)
    DB-->>App: Product data
    Note over App,DB: only 1 query to the database
    App-->>U1: Product data
    App-->>UN: Product data
    deactivate App
    Note over U1,UN: every caller gets the same result
{{</ mermaid >}}

<!-- prettier-ignore-end -->

The only thing that changes is the call into the database: a hundred queries become one.

The [example repo] shows this. It fires 100 concurrent `Get` calls at a cold key and sees a
single `fetch`. After that the cache is warm, so the next 50 reads never reach the upstream.

## The cost of a shared call

Coalescing isn't free. You've routed many callers through one call, so a single failure or
one slow fetch hits all of them.

When the shared call fails, every caller waiting on it gets the same error, not just the one
that triggered it. They can all retry, and the next call starts fresh, because singleflight
drops the result as soon as it's delivered. But for that one window, a single failure hits
everyone.

They also share the wait. A caller is stuck for as long as the shared call takes, and
through `Do` there's no way to bail out early. This is [head-of-line blocking].

Cloudflare hit it. Inside a datacenter, their servers share a cache lock so only one of them
fetches from origin, and they built [concurrent streaming acceleration] so the waiters don't
block until that fetch finishes.

You can't make the shared call faster, but you can keep it from trapping everyone. Bound the
call with its own timeout, and let each caller leave on its own deadline, say a 200ms
budget, instead of waiting out the shared call. `DoChan` gives you both: it returns a
channel, so you select on it alongside the caller's context:

```go
ch := g.DoChan(key, func() (any, error) {
    detached := context.WithoutCancel(ctx) // (1)
    callCtx, cancel := context.WithTimeout(detached, fetchTimeout)
    defer cancel()
    return fetch(callCtx, key)
})
select {
case <-ctx.Done(): // (2)
    return "", ctx.Err()
case res := <-ch: // (3)
    if res.Err != nil {
        return "", res.Err
    }
    return res.Val.(string), nil
}
```

- (1) detach the shared call from any single caller, then give it its own timeout.
  `WithoutCancel` drops the caller's deadline along with its cancellation, so without
  `WithTimeout` the shared fetch would run with no bound
- (2) each caller can still leave on its own deadline; the shared call keeps running for the
  others
- (3) `DoChan` hands back a `singleflight.Result` with `Val`, `Err`, and `Shared`

> [!WARNING]
>
> Passing the first caller's context into the shared call is a common mistake. When that
> caller cancels or times out, it takes down every other caller waiting on the shared call.
> `context.WithoutCancel` (Go 1.21) detaches the shared work from any single caller's
> lifetime. But it drops the deadline too, so give the shared call its own timeout or it
> runs with no bound. Go's resolver detaches its shared lookup from any single caller's
> cancellation, for the same reason.

`ctx.Done` gets one caller out, but it leaves the slow call in flight, so the next caller
just joins it and waits all over again. A `time.After` case caps the wait, and `Forget`
drops the key so that next caller starts fresh. `Forget` doesn't cancel the in-flight fetch,
which is still bounded by its own `fetchTimeout`:

```go
ch := g.DoChan(key, func() (any, error) {
    detached := context.WithoutCancel(ctx)
    callCtx, cancel := context.WithTimeout(detached, fetchTimeout)
    defer cancel()
    return fetch(callCtx, key)
})
select {
case res := <-ch:
    if res.Err != nil {
        return "", res.Err
    }
    return res.Val.(string), nil
case <-time.After(maxWait):
    g.Forget(key)
    return "", context.DeadlineExceeded
}
```

> [!WARNING]
>
> Every waiter gets the same value the shared call returned: one pointer or slice, not a
> per-caller copy. That's fine for an immutable cache fill, but a bug the moment a caller
> mutates it or needs a per-caller result. Even the standard library guards against it: its
> DNS resolver clones the address slice it returns to shared callers, so each caller can
> mutate its copy safely.

## Measuring what it coalesces

Always instrument it. It's the only way to know whether coalescing is doing anything at all.

`Do` returns `shared`, set to `true` when the result went to more than one caller. It's true
for the whole coalesced group, the caller that ran the fetch included. Count those returns
and split them by success:

```go
v, err, shared := s.group.Do(key, func() (any, error) {
    val, err := s.fetch(ctx, key)
    if err != nil {
        return "", err
    }
    s.cache.Set(key, val) // (1)
    return val, nil
})
if shared { // (2)
    if err != nil {
        sharedErr.Add(1) // (3)
    } else {
        sharedOK.Add(1) // (4)
    }
}
// handle err and return v, as before
```

- (1) cache the result here, the same way the cache-aside `Get` does above
- (2) `shared` is true when this result went to more than one caller, the one that ran the
  fetch included, so the call was part of a coalesced group
- (3) the shared call returned an error, so every caller in the group got that error
- (4) the shared call succeeded, so every caller in the group got that value

Counted this way, each increment is one recipient of a shared result, not one suppressed
duplicate. A coalesced group of `n` callers adds `n`, not `n - 1`. That's fine as a relative
signal.

Compare the total against your traffic to see whether coalescing is doing real work or
sitting idle. If it stays near zero, your callers rarely hit the same key at the same time.
Either traffic is low, or the keyspace is wide enough that they don't collide.

A faster upstream lowers the count too: the in-flight window shrinks, so fewer callers land
inside it. A drop can mean a quicker upstream, not a regression.

Watch the ratio of errors to successes. A rising share of shared errors means callers keep
joining a call that fails, then retrying, which serializes the herd instead of absorbing it.

[Fastly] tracks the same shape with two counters, `request_collapse_usable_count` and
`request_collapse_unusable_count`, and treats the split as the signal to watch. Their usable
and unusable are about whether a collapsed request produced a reusable cache object, a
cache-policy question rather than a Go error, so treat the mapping as an analogy, not a
copy.

## Should you do distributed request coalescing?

Singleflight is per-process. A `Group` and its cache see only the calls inside one pod. Run
twenty pods behind a load balancer, and an expiring key gives you one query per pod. About
twenty queries hit the database, not the whole herd.

<!-- prettier-ignore-start -->

{{< mermaid >}}
sequenceDiagram
    participant P1 as Application (pod 1)
    participant PN as Application (pod N)
    participant DB as Database
    Note over P1,DB: each pod coalesces only its own herd
    P1->>DB: SELECT * FROM products WHERE id=123
    PN->>DB: SELECT * FROM products WHERE id=123
    Note over P1,DB: N queries, one per pod
{{</ mermaid >}}

<!-- prettier-ignore-end -->

Per-pod coalescing is usually enough. It ties your database load to pod count instead of
traffic, and you scale pod count on purpose while traffic spikes on its own. Go's own
resolver settles for exactly this: it [coalesces DNS lookups] per process. As long as one
miss per pod fits the downstream's budget, stop here.

Going fleet-wide, down to a single query, means coordinating across pods, and that's where
it gets expensive. There are two ways to do it, and both cost you.

The first is a shared lock: each pod grabs a Redis lease before it fetches, so the database
sees one query. But now Redis sits on every miss's critical path, one more hop that can
stall and one more thing that can fall over, and the surge that used to hit the database
hits the lock instead.

The second is to let infrastructure coalesce for you, no lock to run. [groupcache] routes
each key to an owner pod, so one process makes the call and serves the rest over RPC. A
shared cache tier does the same in front of the origin: [Varnish] coalesces by default,
queuing concurrent requests for one object and sending a single fetch upstream; Nginx does
it behind [proxy_cache_lock]; a [CloudFront Origin Shield] collapses requests across regions
so the origin sees as few as one. A tier only coalesces what it can cache, though, and it
carries the same shared-call costs: one slow fetch stalls the whole waiting list, and an
uncacheable response makes Varnish serialize the queue unless you tag it hit-for-miss.

Each option adds a moving part the miss path didn't have before: a lock, a membership
protocol, or a whole cache tier. That's why most services stop at per-pod.

## When to use it

Coalescing is worth it when three things are true:

- the key is hot enough that callers overlap in time
- the work behind it is expensive or slow enough to be worth sharing
- the key fully determines the result

A hot cache key fits. So do token refreshes, config reloads, and DNS. The same goes for
anything with a predictable hotspot: a scoreboard everyone polls, a feature-flag bundle read
on every request.

When they aren't, coalescing has little to do, and leaving it on is cheap. A miss that
overlaps nothing only pays for `Do`'s lock and map lookup, lost in the noise against a slow
upstream.

So coalescing every cache miss by default is reasonable: it does nothing until a key turns
hot, then absorbs the herd. The exception is genuinely cheap work, where that bookkeeping
can cost more than the call it guards, so measure there before you assume it's free.

Some calls must never be coalesced. Anything with side effects is out. Merging two
`create payment` calls is a correctness bug: the second caller wanted its own payment, not a
copy of the first one's.

The key also has to capture everything that affects the result. Coalesce by URL when the
response depends on the `Authorization` header, and you'll serve one user's data to another.

## Or reach for a library

[sturdyc] already handles all of this: in-flight request coalescing, stampede protection,
refresh-ahead, eviction, and more. If you're building something resilient, see if you can
get away with it before rolling your own.

<!-- references -->
<!-- prettier-ignore-start -->

[golang.org/x/sync/singleflight]:
    https://pkg.go.dev/golang.org/x/sync/singleflight

[cache-aside]:
    https://learn.microsoft.com/en-us/azure/architecture/patterns/cache-aside

[groupcache]:
    https://github.com/golang/groupcache

[sturdyc]:
    https://github.com/viccon/sturdyc

[example repo]:
    https://github.com/rednafi/examples/tree/main/request-coalescing

[request collapsing]:
    https://www.fastly.com/blog/request-collapsing-demystified

[Fastly]:
    https://www.fastly.com/documentation/reference/changes/2024/12/request-collapsing-metrics/

[coalesces DNS lookups]:
    https://go.dev/src/net/lookup.go

[CloudFront Origin Shield]:
    https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/origin-shield.html

[concurrent streaming acceleration]:
    https://blog.cloudflare.com/introducing-concurrent-streaming-acceleration/

[head-of-line blocking]:
    https://en.wikipedia.org/wiki/Head-of-line_blocking

[Varnish]:
    https://info.varnish-software.com/blog/two-minutes-tech-tuesdays-request-coalescing

[proxy_cache_lock]:
    https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_lock

[Caches, Modes, and Unstable Systems]:
    https://brooker.co.za/blog/2021/08/27/caches.html

[thundering herd]:
    https://en.wikipedia.org/wiki/Thundering_herd_problem

[cache stampede]:
    https://en.wikipedia.org/wiki/Cache_stampede

<!-- prettier-ignore-end -->
