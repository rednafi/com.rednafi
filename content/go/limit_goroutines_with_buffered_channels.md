---
title: Limit goroutines with buffered channels
date: 2023-08-23
tags:
    - Go
    - Concurrency Patterns
    - TIL
---

I was cobbling together a long-running Go script to send webhook messages to a system when
some events occur. The initial script would continuously poll a Kafka topic for events and
spawn new goroutines to make HTTP requests to the destination. This had two problems:

- It could create unlimited goroutines if many events arrived quickly
- It might overload the destination system by making many concurrent requests

In Python, I'd use just `asyncio.Semaphore` to limit concurrency. I've previously [written
about this] here. Turns out, in Go, you could do the same with a buffered channel. Here's
how the naive version looks:

```go
package main

import ("fmt"; "sync")

func worker(id int, wg *sync.WaitGroup) {
    defer wg.Done()

    // ... Send http post request
    fmt.Println("Sending webhook request")
}

func main() {
    var wg sync.WaitGroup
    nWorkers := 10
    for i := 1; i <= nWorkers; i++ {
        wg.Add(1)
        go worker(i, &wg)
    }
    wg.Wait()
    fmt.Println("All workers have completed")
}
```

<codapi-snippet sandbox="go" editor="basic">
</codapi-snippet>

We're sending the webhook request in the `worker` function. It takes an integer ID for
bookkeeping and a pointer to a `WaitGroup` instance for synchronization. Once it finishes
making the request, it signals the `WaitGroup` with `wg.Done()`. In the `main` function, we
spawn 10 workers as goroutines and wait for all of them to finish work with `wg.Wait()`.
Without the wait-group synchronization, the `main` goroutine would bail before all the
background workers finish their work.

In the above scenario, all the requests were made in parallel. How can we limit the system
to only allow `n` number of concurrent requests at the same time? Sure, you can choose to
spin up `n` number of goroutines and no more. But how do you do it from inside an infinite
loop that's also polling a queue continuously?

In this case, I want to throttle the script so that it'll send 2 requests in parallel and
then wait until those are done. Then it'll wait for a bit before firing up the next batch of
2 goroutines and continuously repeat the same process. Buffered channels allow us to do
exactly that. Observe:

```go
package main

import ("fmt"; "sync"; "time")

func worker(id int, sem chan struct{}, wg *sync.WaitGroup) {
    defer wg.Done()

    // Acquire semaphore
    fmt.Printf("Worker %d: Waiting to acquire semaphore\n", id)
    sem <- struct{}{}

    // Do work
    fmt.Printf("Worker %d: Semaphore acquired, running\n", id)
    time.Sleep(10 * time.Millisecond)

    // Release semaphore
    <-sem
    fmt.Printf("Worker %d: Semaphore released\n", id)
}

func main() {
    nWorkers := 10      // Total number of goroutines
    maxConcurrency := 2 // Allowed to run at the same time
    batchInterval := 50 * time.Millisecond // Delay between each batch of 2 goros

    // Create a buffered channel with a capacity of maxConcurrency
    sem := make(chan struct{}, maxConcurrency)

    var wg sync.WaitGroup

    // We start 10 goroutines but only 2 of them will run in parallel
    for i := 1; i <= nWorkers; i++ {
        wg.Add(1)
        go worker(i, sem, &wg)

        // Introduce a delay after each batch of workers
        if i % maxConcurrency == 0 && i != nWorkers {
            fmt.Printf("Waiting for batch interval...\n")
            time.Sleep(batchInterval)
        }
    }
    wg.Wait()
    close(sem) // Remember to close the channel once done
    fmt.Println("All workers have completed")
}
```

<codapi-snippet sandbox="go" editor="basic">
</codapi-snippet>

The clever bit here is the buffered channel named `sem` which acts as a semaphore to limit
concurrency. We set its capacity to the max number of goroutines we want running at once, in
this case 2. Before making the request, each `worker` goroutine tries to _acquire_ the
semaphore by sending a value into the channel via `sem <- struct{}{}`. The value itself
doesn't matter. So we're just sending an empty struct to avoid redundant allocation.

Sending data to the channel will block if it's already full, essentially meaning all
_permits_ are taken. Once the send succeeds, the goroutine has acquired the semaphore and is
free to proceed with its work. When finished, it _releases_ the semaphore by reading from
the channel `<-sem`. This frees up a slot in the channel for another goroutine to acquire
it. By using this semaphore channel to limit access to critical sections, we can precisely
control the number of concurrent goroutines.

This channel-based semaphore gives us more flexibility than just using a `WaitGroup`.
Combining it with a buffered channel provides fine-grained control over simultaneous
goroutine execution. The buffer size of the channel determines the allowed parallelism, 2
here. We've also thrown in an extra bit of delay after each batch of operation finishes
with:

```go
// Introduce additional delay after each batch of workers
if i % maxConcurrency == 0 && i != nWorkers {
    fmt.Printf("Waiting for batch interval...\n")
    time.Sleep(batchInterval)
}
```

Running the script will show that although we've started 10 goroutines in the `main`
function, only 2 of them run at once. Also, there's a delay of 3 seconds between each batch.
We can tune it according to our need to be lenient on the consumer.

```txt
Waiting for batch interval...
Worker 2: Waiting to acquire semaphore
Worker 2: Semaphore acquired, running
Worker 1: Waiting to acquire semaphore
Worker 1: Semaphore acquired, running
Worker 1: Semaphore released
Worker 2: Semaphore released
Waiting for batch interval...
Worker 4: Waiting to acquire semaphore
Worker 4: Semaphore acquired, running
Worker 3: Waiting to acquire semaphore
Worker 3: Semaphore acquired, running
Worker 3: Semaphore released
Worker 4: Semaphore released
Waiting for batch interval...
...
```

Now, you might want to add extra abstractions over the core behavior to make it more
ergonomic. Here's a [pointer] on how to do so. [Effective Go also mentions] this pattern
briefly.

## Further readings

- [How to wait until buffered channel semaphore is empty]

<!-- Resources -->
<!-- prettier-ignore-start -->

<!-- Limit concurrency with semaphore - rednafi -->
[written about this]:
    /python/limit_concurrency_with_semaphore

<!-- go concurrency pattern: semaphore -->
[pointer]:
    https://levelup.gitconnected.com/go-concurrency-pattern-semaphore-9587d45f058d

<!-- effective go - channels -->
[effective go also mentions]:
    https://go.dev/doc/effective_go#channels


[how to wait until buffered channel semaphore is empty]:
    https://stackoverflow.com/questions/39776481/how-to-wait-until-buffered-channel-semaphore-is-empty

<!-- prettier-ignore-end -->

<link rel="stylesheet" href="/modules/codapi/snippet.css"/>
<script defer src="/modules/codapi/snippet.js"></script>
