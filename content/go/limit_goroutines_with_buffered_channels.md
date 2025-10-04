---
title: Limit goroutines with buffered channels
date: 2023-08-23
slug: limit-goroutines-with-buffered-channels
aliases:
    - /go/limit_goroutines_with_buffered_channels/
tags:
    - Go
    - TIL
---

I was cobbling together a long-running Go script to send webhook messages to a system when
some events occur. The initial script would continuously poll a Kafka topic for events and
spawn worker goroutines in a fire-and-forget manner to make HTTP requests to the
destination. This had two problems:

- It could create unlimited goroutines if many events arrived quickly (no backpressure)
- It might overload the destination system by making many concurrent requests (no
  concurrency control)

In Python, I'd use just `asyncio.Semaphore` to limit concurrency. I've previously [written
about this] here. Turns out, in Go, you could do the same with a buffered channel. Here's
how the naive version without any concurrency control looks:

```go
// go 1.24
package main

import (
    "fmt"
    "time"
)

// pollKafka pretends to fetch a message from Kafka
func pollKafka() string {
    time.Sleep(200 * time.Millisecond) // emulate poll delay
    return fmt.Sprintf("kafka-msg-%d", time.Now().UnixNano())
}

// worker simulates doing something with a message
func worker(id int, msg string) {
    fmt.Printf("worker %d: sending webhook for message: %s\n", id, msg)
    time.Sleep(200 * time.Millisecond)
}

func main() {
    for id := 0; ; id++ {
        // Poll a new message from Kafka before spawning the worker
        msg := pollKafka()

        // Spawn a worker goroutine for each message — fire and forget
        go worker(id, msg)
    }
}
```

Running it gives you this:

```txt
worker 0: sending webhook for message: kafka-msg-1759579628289116000
worker 1: sending webhook for message: kafka-msg-1759579629290305000
worker 2: sending webhook for message: kafka-msg-1759579630291584000
worker 3: sending webhook for message: kafka-msg-1759579631292667000
worker 4: sending webhook for message: kafka-msg-1759579632293768000
worker 5: sending webhook for message: kafka-msg-1759579633294909000
^Csignal: interrupt
```

The `main` function runs an infinite loop where it polls the upstream message queue
continuously to collect new message. Once a new message arrives, it spawns a new worker
goroutine in a fire-and-forget manner that actually sends the HTTP request to the desination
endpoint.

The problem here is that this setup creates an unbounded number of goroutines. If Kafka
produces messages faster than the workers can process them, the system will keep spawning
new goroutines, eventually consuming all available memory and CPU. Also, it can overwhelm
the destination system by sending too many requests at once if that doesn't have any
throttling mechanism in place. So we need a way to limit how many workers can run at once.

To fix this, we can use a buffered channel as a semaphore. The idea is to block before
launching a new worker if too many are already running. This applies backpressure naturally
and prevents unbounded spawning. Observe:

```go
// go 1.24
package main

import (
    "fmt"
    "time"
)

// pollKafka pretends to fetch a message from Kafka
func pollKafka() string {
    time.Sleep(500 * time.Millisecond) // emulate poll delay
    return fmt.Sprintf("kafka-msg-%d", time.Now().UnixNano())
}

// worker simulates doing something with a message
func worker(id int, msg string) {
    fmt.Printf("worker %d: sending webhook for message: %s\n", id, msg)
    time.Sleep(200 * time.Millisecond)
}

func main() {
    maxConcurrency := 2
    sem := make(chan struct{}, maxConcurrency) // semaphore
    batchInterval := 1 * time.Second

    for id := 0; ; id++ {
        msg := pollKafka() // get a message first

        sem <- struct{}{} // acquire BEFORE spawning; applies backpressure

        // Use a closure to wrap the original worker so that concurrency
        // primitives like semaphores don't pollute the core worker function.
        go func() {
            defer func() { <-sem }() // release when done
            worker(id, msg)
        }()

        // Apply backpressure
        if id%maxConcurrency == 0 && id != 0 {
            fmt.Printf("Limit reached, waiting %s...\n", batchInterval)
            time.Sleep(batchInterval)
        }
    }
}
```

Here, the buffered channel `sem` works as a semaphore that limits concurrency. Its capacity
defines how many goroutines can run at the same time. Before spawning a worker, we try to
send an empty struct into the channel. If the channel is full, that line blocks until a
running worker finishes and releases its spot by reading from the channel. This ensures that
only `maxConcurrency` workers run at once and prevents goroutine buildup.

The closure around the worker is intentional: it keeps concurrency management out of the
worker itself. The worker only focuses on processing messages, while the outer function
handles synchronization and throttling. This separation allows the caller to call the worker
synchronously if needed. It also makes testing the worker function much easier. In general,
it's a good practice to push concurrency to the outer edge of your system so that the caller
has the choice of leveraging concurrency or not.

The optional batch delay isn't required for correctness, but it helps spread out requests so
the downstream system isn't flooded. Running the script shows that even though the loop is
infinite, only two workers run at once, and there's a short pause between each batch.

```txt
worker 0: sending webhook for message: kafka-msg-1759580074075447000
worker 1: sending webhook for message: kafka-msg-1759580074576160000
Limit reached, waiting 1s...
worker 2: sending webhook for message: kafka-msg-1759580075077279000
worker 3: sending webhook for message: kafka-msg-1759580076579356000
Limit reached, waiting 1s...
worker 4: sending webhook for message: kafka-msg-1759580077079956000
worker 5: sending webhook for message: kafka-msg-1759580078581780000
Limit reached, waiting 1s...
worker 6: sending webhook for message: kafka-msg-1759580079082375000
^Csignal: interrupt
```

The workers in this version are still getting spawned in a fire-and-forget manner but
without leaking goroutines. When the concurrency limit is reached, the main loop blocks
instead of spawning more workers. This applies natural backpressure to the producer, keeping
the system stable even under heavy load.

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
