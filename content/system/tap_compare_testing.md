---
title: Tap compare testing for service migration
date: 2025-12-13
slug: tap-compare-testing
tags:
    - Distributed Systems
    - Go
mermaid: true
---

Throughout the years, I've been part of a few medium- to large-scale system migrations. As
in, rewriting old logic in a new language or stack to gain better scalability, resilience,
and maintainability, or to respond more easily to changing business requirements. Whether
rewriting your system is the right move is its own debate.

A common question that shows up during a migration is, "How do we make sure the new system
behaves exactly like the old one, minus the icky parts?" Another one is, "How do we build
the new system while the old one keeps changing without disrupting the business?"

There's no universal playbook. It depends on how gnarly the old system is, how ambitious the
new system is, and how much risk the business can stomach. After a few of these migrations,
one approach keeps coming back.

The core idea is that you shadow a slice of production traffic to the new system. The old
system keeps serving real users. A copy of that same traffic is forwarded to the new system
along with the old system's response. The new system runs the same business logic and
compares its outputs with the old one. The entire point is to make the new system return the
exact same answer the old one would have, for the same inputs and the same state.

At the start, you don't rip out bad behavior or ship new features. Everything is about
output parity. Once the systems line up and the new one has processed enough real traffic to
earn some trust, you start sending actual user traffic to it. If something blows up, you
roll back. If it behaves as expected, you push more traffic. Eventually the old system gets
to ride off into the sunset.

This workflow is typically known as _[shadow testing]_ or _tap and compare testing_.

## The scenario

Say we have a Python service that exposes a handful of read and write endpoints the business
depends on. It's been around for a while. Different teams have patched it over the years,
and parts of the logic behave the way they do because of decisions nobody remembers. While
it still works, over time, it's getting harder to maintain. The business wants a tighter
SLO, so the team decides to rebuild the service in Go.

To keep the scope contained, I'm only talking about HTTP read and write endpoints on the
main request path. I'm ignoring everything else: message queues, background workers, async
job processing, analytics pipelines, and other side channels that also need to be migrated.
For migrating between gRPC services, the workflow is pretty much identical. You mirror
calls, let both services handle them, and compare responses. The transport details change,
not the pattern.

During shadow testing, the Python service stays on the main request path. All real user
traffic still goes to the Python service. A proxy or load balancer sitting in front of it
forwards requests as usual, gets an answer back, and returns that answer to the user.

That same proxy also emits tap events. Each tap event contains a copy of the request and the
canonical response the Python service sent to the user. Those tap events go to the Go
service on a shadow path. From the outside world, nothing has changed. Clients talk to
Python, and Python talks to the live production database.

The Go service never serves real users during this phase. It only sees tap events. For each
event, it reconstructs the request, runs its version of the logic against a separate
datastore, and compares its outputs with the Python response recorded in the event. The
Python response is always the source of truth. There's no second call back into Python to
guess what it might do.

The Go service has its own datastore, usually a snapshot or replica of production that's
been detached so it can be written freely. This is the sister datastore. The Go service only
talks to it for reads and writes. It never touches the real production DB. The sister
datastore is close enough to show real-world behavior but isolated enough that nothing
breaks.

With this setup in place, you spend time fixing differences. If the Python service returns a
specific payload shape or some quirky value, the Go service has to match it. If Python gets
a bug fix or a new feature, you update Go. You keep doing this until shadow traffic stops
producing mismatches. Then you start thinking about cutover.

## Start with read endpoints

Reads don't change anything in the database, so they are easier to start with.

On the main path, a user sends a request. The proxy forwards it to the Python service as
usual. The Python service reads from the real database, builds a response, and returns it to
the caller.

While that is happening, the proxy also constructs a tap event. At minimum, this event
contains:

- The original request: method, URL, headers, body.
- The canonical Python response: status code, headers, body.

The proxy sends this tap event to the Go service on an internal HTTP or RPC endpoint. The
important thing is that the tap event captures the exact input and output of the Python
service as seen by the real user.

A typical read path diagram during tap compare looks like this:

<!-- prettier-ignore-start -->

{{< mermaid >}}
graph TD
    subgraph MAIN_PATH [MAIN PATH]
        User([User]) --> Proxy
        Proxy --> Python
        Python <-- reads production state --> ProdDB[(Prod DB)]
    end

    subgraph SHADOW_PATH [SHADOW PATH]
        Proxy -- "tap event: {request, python_resp}" --> Go
        Go <--> SisterDB[(Sister DB)]
        Go --> Log[Log mismatch?]
    end

    classDef userStyle fill:#6b7280,stroke:#4b5563,color:#fff
    classDef proxyStyle fill:#7c3aed,stroke:#5b21b6,color:#fff
    classDef pythonStyle fill:#2563eb,stroke:#1d4ed8,color:#fff
    classDef goStyle fill:#0d9488,stroke:#0f766e,color:#fff
    classDef dbStyle fill:#ca8a04,stroke:#a16207,color:#fff
    classDef logStyle fill:#dc2626,stroke:#b91c1c,color:#fff

    class User userStyle
    class Proxy proxyStyle
    class Python pythonStyle
    class Go goStyle
    class ProdDB,SisterDB dbStyle
    class Log logStyle
{{</ mermaid >}}

<!-- prettier-ignore-end -->

From the Go service's point of view, a tap event is just structured data. A simple shape
might look like this on the wire:

```json
{
  "request": {
    "method": "GET",
    "url": "/users/123?verbose=true",
    "headers": { "...": ["..."] },
    "body": "..."
  },
  "python_response": {
    "status": 200,
    "headers": { "...": ["..."] },
    "body": "{ \"id\": \"123\", \"name\": \"Alice\" }"
  }
}
```

The Go side reconstructs the request, runs its own logic against the sister datastore, and
compares its answer with `python_response`. No extra call back into Python. No race between
a second read and the response that already went to the user.

On the Go side, a handler for a read tap event might look like this:

```go
type TapRequest struct {
    Method  string              `json:"method"`
    URL     string              `json:"url"`
    Headers map[string][]string `json:"headers"`
    Body    []byte              `json:"body"`
}

type TapResponse struct {
    Status  int                 `json:"status"`
    Headers map[string][]string `json:"headers"`
    Body    []byte              `json:"body"`
}

type TapEvent struct {
    Request        TapRequest  `json:"request"`
    PythonResponse TapResponse `json:"python_response"`
}

func HandleGetUserTap(w http.ResponseWriter, r *http.Request) {
    // This endpoint is internal only.
    // It receives tap events from the proxy, not real user traffic.

    var tap TapEvent
    if err := json.NewDecoder(r.Body).Decode(&tap); err != nil {
        http.Error(w, "bad tap payload", http.StatusBadRequest)
        return
    }

    // Rebuild something close to the original HTTP request.
    reqURL, err := url.Parse(tap.Request.URL)
    if err != nil {
        http.Error(w, "bad url", http.StatusBadRequest)
        return
    }

    // Body is a one-shot stream, so buffer it for reuse.
    bodyBytes := append([]byte(nil), tap.Request.Body...)

    goReq := &http.Request{
        Method: tap.Request.Method,
        URL:    reqURL,
        Header: http.Header(tap.Request.Headers),
        Body:   io.NopCloser(bytes.NewReader(bodyBytes)),
    }

    // Go service: run candidate logic against sister datastore.
    goResp, goErr := goUserService.GetUser(r.Context(), goReq)
    if goErr != nil {
        log.Printf("go candidate error: %v", goErr)
    }

    // Normalize and compare off the main response path.
    // The real user already got python_response.
    go func() {
        normalizedPython := normalizeHTTP(tap.PythonResponse)
        normalizedGo := normalizeHTTP(goResp)

        if !deepEqual(normalizedPython, normalizedGo) {
            log.Printf(
                "read mismatch: url=%s python=%v go=%v",
                tap.Request.URL,
                normalizedPython,
                normalizedGo,
            )
        }
    }()

    // Optional debugging response for whoever is calling the tap endpoint.
    w.WriteHeader(http.StatusNoContent)
}
```

A few things matter here:

- Truth lives with the Python response that already went to the user.
- The Go service sees exactly the same request the Python service saw.
- Comparison happens off the user path. Users never wait on the Go service.
- The Go service only touches the sister datastore, never the real one.

When the diff rate on reads stabilizes across real traffic, you have decent evidence that
the Go implementation behaves like the Python one for real-world inputs and state.

## Write endpoints are trickier

Reads are easy because they don't change state. Writes are harder to migrate.

On the main path, only the Python service is allowed to mutate production state.

A typical write looks like this on the main path:

1. User sends a write request.
2. Proxy forwards it to the Python service.
3. Python runs the real write logic, talks to the live database, sends emails, charges
   cards, and returns a response.
4. Proxy returns that response to the user.

That path is the only one touching production. The Go service must not:

- write anything to the real production database
- trigger real external side effects
- call any real Python write endpoint in a way that causes a second write

For writes, the tap event pushed by the proxy looks very similar to reads:

```json
{
  "request": {
    "method": "POST",
    "url": "/users",
    "headers": { "...": ["..."] },
    "body": "{ \"email\": \"alice@example.com\", \"name\": \"Alice\" }"
  },
  "python_response": {
    "status": 201,
    "headers": { "...": ["..."] },
    "body": "{ \"id\": \"123\", \"email\": \"alice@example.com\" }"
  }
}
```

The write path diagram during tap compare becomes:

<!-- prettier-ignore-start -->

{{< mermaid >}}
graph TD
    subgraph MAIN_PATH [MAIN PATH]
        User([User]) --> Proxy
        Proxy --> Python
        Python <-- writes prod state, triggers side effects --> ProdDB[(Prod DB)]
    end

    subgraph SHADOW_PATH [SHADOW PATH]
        Proxy -- "tap event: {request, python_resp}" --> Go
        Go <--> SisterDB[(Sister DB)]
        Go --> Log[Log mismatch?]
    end

    classDef userStyle fill:#6b7280,stroke:#4b5563,color:#fff
    classDef proxyStyle fill:#7c3aed,stroke:#5b21b6,color:#fff
    classDef pythonStyle fill:#2563eb,stroke:#1d4ed8,color:#fff
    classDef goStyle fill:#0d9488,stroke:#0f766e,color:#fff
    classDef dbStyle fill:#ca8a04,stroke:#a16207,color:#fff
    classDef logStyle fill:#dc2626,stroke:#b91c1c,color:#fff

    class User userStyle
    class Proxy proxyStyle
    class Python pythonStyle
    class Go goStyle
    class ProdDB,SisterDB dbStyle
    class Log logStyle
{{</ mermaid >}}

<!-- prettier-ignore-end -->

On the Go side, the write tap handler follows the same pattern as reads but has more corner
cases to think through.

A shadow write handler might look like this:

```go
type UserInput struct {
    Email string `json:"email"`
    Name  string `json:"name"`
    // ... other fields
}

type User struct {
    ID        string    `json:"id"`
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    // ... other fields
}

func HandleCreateUserTap(w http.ResponseWriter, r *http.Request) {
    // Internal only. Receives tap events for CreateUser.

    var tap TapEvent
    if err := json.NewDecoder(r.Body).Decode(&tap); err != nil {
        http.Error(w, "bad tap payload", http.StatusBadRequest)
        return
    }

    // Decode the original request body once.
    var input UserInput
    if err := json.Unmarshal(tap.Request.Body, &input); err != nil {
        log.Printf("bad original json: %v", err)
        return
    }

    // The Python response is canonical: this is what the user saw.
    pyUser, err := decodePythonUser(tap.PythonResponse)
    if err != nil {
        log.Printf("bad python response: %v", err)
        return
    }

    // Run the Go write path against the sister datastore.
    // This must never talk to the live production DB.
    goUser, goErr := goUserService.CreateUserInSisterStore(
        r.Context(), input,
    )
    if goErr != nil {
        log.Printf("go candidate write error: %v", goErr)
    }

    // Compare results asynchronously.
    go func() {
        normalizedPython := normalizeUser(pyUser)
        normalizedGo := normalizeUser(goUser)

        if !compareUsers(normalizedPython, normalizedGo) {
            log.Printf(
                "write mismatch: email=%s python=%v go=%v",
                normalizedPython.Email,
                normalizedPython,
                normalizedGo,
            )
        }
    }()

    w.WriteHeader(http.StatusNoContent)
}
```

You are comparing how each system transforms the same request into a domain object and
response. You are not trying to drive the Python service a second time. You are not trying
to rebuild the Python result from scratch against changed state.

But with this setup, the write path has several corner cases to think through.

### Uniqueness, validation, and state-dependent logic

Uniqueness checks, conditional updates, and other validations that depend on database state
are sensitive to timing. The Python write runs against the actual production state at the
moment the main request hits. The Go write runs against whatever state exists in the sister
datastore when the tap event arrives.

If the sister datastore is a snapshot that is not continuously replicated, it will drift
almost immediately. Even with streaming replication, there may be short lags. That means:

- A create request that was valid in prod might look invalid against a slightly stale
  snapshot if another request changed state in between.
- A conditional update like "only update if version is X" can take different branches if the
  sister store has not applied the latest change yet.
- A multi-entity invariant that Python enforced with a transaction might appear broken in
  the sister store if replication replayed statements in a different order relative to the
  tap event.

You should expect some write comparisons to be noisy because of state drift and treat those
separately. In practice you often:

- Keep replication as close to real time as you can, or regularly reseed the sister
  datastore.
- Attach a few state fingerprints to the tap event, like the version of the row before and
  after the write, so you can tell when the sister store is simply behind.
- Filter out mismatches that can be traced to obvious replication lag when you look at diff
  reports.

The important thing is: when you see a mismatch, you can decide whether it is a real logic
difference or just the sister store living in a slightly different universe for that
request.

### Idempotency, retries, and ordering

Real systems don't get one clean write per user action. You get retries, duplicates, and
concurrent updates.

On the main path, you might have:

- A user hitting "submit" twice.
- A client retrying on a network timeout.
- Two services racing to update the same record.

Your Python service probably already has a story for this, such as idempotency keys, version
checks, or last-write-wins semantics. The tap path needs to reflect what actually happened,
not an idealized story.

Because the tap event is constructed from the real request and real response at the proxy,
it naturally honors whatever the Python service did. If a retry was coalesced into a single
write under an idempotency key, you will see a single successful response in the tap stream.
If the second retry was rejected as a conflict, you will see that error. The Go service just
needs to implement the same semantics against the sister datastore.

What still bites you is ordering. Tap events may arrive at the Go service a little out of
order relative to how mutations hit production. If two writes race, Python might process
them in order A, B while the tap messages arrive as B, A. The sister datastore will then
experience a different sequence of state changes than production did, which can yield
legitimate differences in final state.

You can't fully eliminate this. What you can do is:

- Keep tap delivery low latency and best-effort ordered.
- Focus your comparisons more on single-request behavior (did CreateUser behave the same)
  than on multi-request history until you are comfortable with the noise.
- Use version numbers or timestamps in the domain model to detect when the sister store is
  applying changes in a different order, and treat those as "not comparable" rather than
  bugs.

### External side effects

Writes often have external side effects: emails, payment gateways, cache invalidations,
search indexing, analytics.

The tap path isolates database writes by using the sister datastore, but that is not enough
on its own. You have to run the Go service in a mode where those side-effectful calls are
either disabled or mocked.

The usual pattern is:

- Centralize side-effectful behavior behind interfaces or specific modules.
- In normal production mode, those modules call real providers.
- In tap compare mode, they are wired to no-op or record-only implementations.

You want the code paths that decide "should we send a welcome email" or "should we charge
this card" to run, because they influence the domain model and response shape. You don't
want the actual email to go out or the real payment provider to be hit twice.

On the Python side, you don't need dry runs or special write endpoints. The real main path
already did the work, and the tap event gives you the results. The only thing the Python
service might need for tap compare is a dedicated read endpoint that returns a normalized
view of state if you want to sample post-write state directly. That read endpoint must not
cause extra writes or side effects.

### What tap compare can and can't tell you on writes

Tap compare with proxy-captured responses and a sister datastore is strong but not magic.

It tells you:

- For a given real user request and the production state that existed at that moment, what
  the Python service chose to return.
- Whether your Go service, running against a similar but separate view of state, tends to
  produce the same shape and content of domain objects and responses.
- Whether your Go write path can execute at all against realistic traffic without panicking
  or tripping over obvious logic errors.

It doesn't guarantee:

- That the Go service produces exactly the same side effects in exactly the same order as
  the Python service. External systems and replication noise get in the way.
- That the Go service behaves identically under arbitrary concurrent write histories. You
  saw the histories that actually happened during the tap window, which might miss some edge
  case interleavings.
- That all mismatches are bugs. Some will be explained by replication lag, idempotency
  behavior, or intended fixes.

The right way to think about it is: tap compare lets you align the new system with the old
one for the traffic you actually have, under the state and timing conditions you actually
experienced. It shrinks the unknowns before you put the new system in front of real users.

## Risks and pitfalls

Most of the sharp edges already showed up in the write section: database drift and
replication lag, idempotency and ordering, and external side effects. Tap compare doesn't
make those go away. On top of that, a few cross-cutting issues are worth calling out
briefly.

**Logging and privacy:** It's tempting to dump the full request and response on every
mismatch. That is also a good way to leak user data into logs. Treat tap logs as sensitive.
Prefer logging IDs, fingerprints, and a few representative fields over full payloads, and
keep any raw dumps behind feature flags or in locked-down storage.

**Non-deterministic data:** You rarely get a byte-for-byte match between a Python app and a
Go app. Auto-incremented IDs diverge, timestamps differ by milliseconds, and serialization
details like `10.0` versus `10` don't matter for correctness. Both services may also call
`now()` or sample randomness during a request. Your comparison layer has to normalize or
ignore these fields, or treat time and randomness as inputs that are captured in the tap
event rather than hidden globals.

**Bug compatibility:** The old Python code will have bugs: swallowed errors, wrong status
codes, odd edge case behavior. The new Go code may fix those bugs, which shows up as a
mismatch. Sometimes you deliberately replicate the bug in the Go service to get to a high
match rate and keep the migration low-risk. Later, once the new system is on the main path
and you can communicate behavioral changes, you remove the compatibility shim and fix the
behavior for real.

**Cost and blast radius:** Shadowing high-volume production traffic can be expensive. The Go
service is doing real work against the sister datastore, often with extra logging and debug
features enabled. Plan for the extra load on databases, caches, and queues so the tap path
doesn't accidentally degrade the main path.

The shape of the migration then looks like this:

- Python stays on the main path, talking to the real production DB and external systems.
- The proxy emits tap events containing both the original request and the canonical Python
  response.
- The Go service consumes tap events, talks only to the sister datastore, and compares its
  outputs with Python.
- When mismatches shrink to an acceptable level and you understand the ones that remain, you
  start routing a small percentage of real traffic to the Go service as the source of truth.

## From tap handlers to production handlers

In the examples above, the handlers are intentionally suffixed with `Tap` and return
`204 No Content`. That behavior is deliberate: `HandleGetUserTap` and `HandleCreateUserTap`
exist only to ingest tap events, call into the Go business logic, and log mismatches. They
are never meant to sit on the main request path or respond to real users. Their job is to
exercise the candidate code against real traffic, not to become the production surface.

If you tried to "promote" those `*Tap` handlers directly at cutover, you'd be wiring
responses for the first time right when risk is highest. That's backwards. The safer shape
is to separate three layers:

- **Core business logic** that doesn't know about tap at all (for example, methods on
  `goUserService` that accept a context and a request or domain input and return a domain
  response).
- **Production HTTP handlers** that call that logic and write real responses for users.
- **Tap handlers** that call the same logic, compare against the Python response from the
  tap event, and then discard the result.

A production read handler might look like this:

```go
func HandleGetUser(w http.ResponseWriter, r *http.Request) {
    resp, err := goUserService.GetUser(r.Context(), r)
    if err != nil {
        // Map domain errors to HTTP.
        writeError(w, err)
        return
    }

    // This should already be returning the same shape
    // you validated against the Python response.
    writeHTTP(w, resp)
}
```

During tap compare, the `HandleGetUserTap` endpoint feeds the same inputs into
`goUserService.GetUser` and compares `resp` against the Python response from the tap event.
The real handler and the tap handler share the same core logic and response shape; the only
difference is who sees the result. That means by the time you cut over, the code that
actually writes HTTP responses has already been exercised and diffed against the Python
version. The `204` status in the tap handlers is intentional: it makes it obvious those
endpoints are not user-facing.

The cutover then looks less ceremonious:

- Before cutover, the Go service already has regular handlers like `HandleGetUser` and
  `HandleCreateUser` wired up, returning the same payload shape as Python. They may serve
  staging traffic, synthetic load, or a tiny canary slice behind a feature flag.
- The tap handlers keep calling the same core logic but only for comparison: they read tap
  events, call into the service, normalize responses, and log diffs.

When you're ready to put Go on the main path, the cleanup requires some grunt work:

- **Scrape the tap handlers.** Delete or disable the `*Tap` endpoints. They were
  intentionally named so you can find and remove them handler by handler.
- **Remove tap-only wiring.** Strip out comparison code, normalization glue, and any
  sister-datastore–only plumbing that never runs on the main path.
- **Point the core logic at the real datastore.** Swap `CreateUserInSisterStore` for the
  real write path, or make the target datastore configurable and turn off sister mode. The
  same service methods now talk to the production database instead of the sister store.
- **Flip the proxy.** Route user traffic to `HandleGetUser`, `HandleCreateUser`, and
  friends, not to Python. At that point, the Go responses are canonical.
- **Optionally keep a thin tap path.** If you want extra safety, continue to mirror a small
  fraction of traffic back to Python (or to an older Go version) and compare in the
  background, but treat that as a guardrail, not the main migration mechanism.

Tap compare is scaffolding. The `*Tap` handlers, the sister datastore, and the comparison
layer exist so you can learn how the new system behaves under real traffic without putting
it in charge.

## Parting words

Typically, you don't have to build all the plumbing by hand. Proxies like [Envoy], [NGINX],
and [HAProxy], or a service mesh like [Istio], can help you mirror traffic, capture tap
events, and feed them into a shadow service. I left out tool-specific workflows so that the
core concept doesn't get obscured.

Tap compare doesn't remove all the risk from a migration, but it moves a lot of it into a
place you can see: mismatched payloads, noisy writes, and gaps in business logic. Once those
are understood, switching over to the new service is less of a big bang and more of a boring
configuration change, followed by trimming a pile of `*Tap` code you no longer need.

<!--References -->

<!-- prettier-ignore-start -->

[Envoy]: https://www.envoyproxy.io/
[NGINX]: https://nginx.org/
[HAProxy]: https://www.haproxy.org/
[Istio]: https://istio.io/
[shadow testing]: https://microsoft.github.io/code-with-engineering-playbook/automated-testing/shadow-testing/

<!-- prettier-ignore-end -->
