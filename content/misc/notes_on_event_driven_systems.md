---
title: Notes on building event-driven systems
date: 2024-09-21
tags:
    - Networking
---

I spent the evening watching this incredibly grokkable talk on event-driven services by
James Eastham at NDC London 2024. Below is a cleaned-up version of my notes.

I highly recommend watching the full talk if you're interested before reading this
distillation.

{{< youtube qcJASFx-F5g >}}

## The curse of tightly coupled microservices

Microservices often start with HTTP-based request-response communication, which seems
straightforward but quickly becomes a pain as systems grow. Coupling—where one service
depends on another—creates a few issues. Take the _order processing service_ in a fictional
Plant-Based Pizza company. It has to talk to the _pickup service_, _delivery service_,
_kitchen_, and _loyalty point_ service. They're all tied together, so if one fails, the
whole system could go down.

The system relies on all services being up at the same time, which causes issues when any
service crashes. Even something like loyalty points can take the whole thing offline, making
you wonder if the order processing service really needs to care about that.

On top of that, there's semantic coupling—things like data formats. _"How do you handle null
values in strings? What casing is your JSON using—camelCase?"_ These details might seem
minor, but in tightly coupled systems, they pile up, making the integrations fragile and
complicated.

## What event-driven architecture solves

Event-driven architecture offers a way to decouple services. Instead of one service needing
to communicate directly with another, services react to events, giving more flexibility and
scalability. _"Event-driven architecture is about reversing dependencies—reversing the lines
of integration."_ Now, the order processing service doesn't need to know where the
downstream services are. It simply publishes an event, and the downstream services react to
it.

This shift is powerful because it frees services from having to know about each other. In
this model, the kitchen doesn't wait for the order processing service to send a direct HTTP
request. Instead, it listens for an event that triggers its response without direct
integration. _"You've removed that runtime coupling because producers and consumers no
longer need to know each other exist."_ By decoupling systems, event-driven architecture
improves fault tolerance, scalability, and flexibility.

## The nature of an event

At the core of event-driven systems is the concept of an event. An event is _"an immutable
fact. It's something that's happened in the past. It cannot be changed."_ When an event is
published, it's a record of something that has already occurred, like _"order confirmed"_ or
_"pizza boxed."_ Events are simple, factual, and unchangeable.

The analogy of a light switch brings this to life: _"You hit the light switch, and that
raises a light switched-on event. You can't un-switch on a light."_ To turn the light off,
you generate a new event—_“light switched-off"_—but you don't undo the original. This
principle of immutability ensures that events in the system are reliable and unambiguous,
forming the foundation for how systems react.

## Event-driven vs. event-based systems

It's easy to confuse event-driven systems with event-based systems, but the distinction is
crucial. Event-driven systems are driven by business-specific events—things that reflect
real-world actions and decisions, not just technical events like a button click. _"An
event-driven system uses events like these: 'order confirmed,' 'pizza boxed,' 'staff member
clocked in.'"_ These are business-level events that reflect the narrative of the company,
not just low-level system changes.

In contrast, event-based systems simply react to any change, such as a file being added to
S3 or a button being clicked in a UI. _"We've been building event-based systems for years...
that doesn't make it event-driven."_ The difference is significant because event-driven
systems align technical architecture with business needs, creating a more meaningful,
coherent system where the events reflect the organization's core processes.

## Fat vs. sparse events

One of the critical design decisions in event-driven systems is choosing between fat events
(which carry a lot of data) and sparse events (which carry minimal data). Fat events, also
known as Event-Carried State Transfer (ECST), include all the information a consumer might
need. For instance, _"the kitchen can consume this event—it's got the list of items on the
order, so now it knows what it needs to cook."_ This reduces the need for callbacks or
additional requests for data back to the original system that publishes the event, making
the system more robust in terms of runtime interaction.

However, fat events come with risks. _"The downside of that is that you get more coupling at
the schema level."_ Because fat events contain so much information, it becomes harder to
change the event format without impacting multiple consumers. As more services depend on
that data, the risk of breaking something grows.

In contrast, sparse events are lightweight but require callbacks to get additional
information. Initially, this might seem more efficient, but as more services join the
system, the number of callbacks increases exponentially. _"Now you've got this potentially
infinite number of downstream services that are all making calls back to get more
information."_ The result is a more tightly coupled system, albeit in a different form.

So which one of these is the right one to choose? The answer is: it depends. Sparse events
reduce the need for frequent changes, but fat events reduce the need for constant
back-and-forth communication. Often, a combination of both is necessary, depending on the
use case.

## Publish-subscribe pattern and the role of the broker

The core of event-driven architecture lies in the publish-subscribe pattern, facilitated by
an event broker. _"At its core, an event-driven architecture is made up of three parts: You
have a producer, a consumer, and some kind of event broker in the middle."_ The producer
generates the event, the broker routes it, and the consumer processes it. The beauty of this
system is that producers and consumers don't need to know about each other's existence.

_"The first thing you'll notice is that the producer and the consumer here have no idea each
other exists—the communication is managed by the broker."_ This decoupling makes the system
more flexible and scalable. A consumer can be added or removed without impacting the
producer. The broker ensures that events are delivered, allowing the system to continue
functioning smoothly even as it evolves.

However, one responsibility remains: _"The schema of your event—the format of that event—is
the biggest part of the coupling that you will see in event-driven architecture."_ While
runtime coupling is removed, semantic coupling still exists. Producers must ensure that the
event schema doesn't change in ways that break existing consumers.

## Handling constraints and governance

In event-driven systems, the responsibility for handling constraints shifts from the
producer to the consumer. Producers generate events as quickly as they can, without worrying
about the load on consumers. _"As a producer, it's not your responsibility to care about how
your events are used... that's the subscriber's responsibility."_ Consumers must handle
their own ingestion rates and ensure they don't get overloaded.

Governance plays a critical role in managing these systems, particularly as they evolve.
When changes are made to event schemas, it's essential to communicate those changes to all
consumers. _"Governance is really important with event-driven architecture because you've
got these systems that just don't care about each other."_ One effective method for managing
this is through Request for Comments (RFCs), which allow for collaborative discussion before
any changes are implemented.

_"Rather than just publishing an event and hoping for the best, introducing governance
ensures that events remain consistent and understandable across teams."_ This helps prevent
breaking changes that could take down systems you didn't even know were relying on your
events.

## Metadata-data pattern for evolvability

To enhance the evolvability of an event-driven system, East recommends using the
metadata-data pattern. This pattern separates the event's core data from its metadata,
allowing for greater flexibility. _"Splitting your event down into a metadata section and
the data section helps you to stay evolvable."_ The data contains the specifics of the
event, while the metadata includes information like _"event type,"_ _"event ID,"_ and
_"version."_

This separation allows consumers to understand and process events more easily while
providing room for schema changes. For example, _"event versioning allows you to introduce
breaking changes in a controlled manner."_ By publishing multiple versions of an event, you
can ensure backward compatibility while encouraging consumers to upgrade to the latest
schema.

## Eventual consistency in event-driven systems

One of the trade-offs in event-driven architecture is that systems must embrace eventual
consistency. In a request-response system, actions happen immediately and are reflected in
real-time. But in an event-driven system, updates propagate over time. _"Eventually, over
time, these systems will converge on the same view of the world."_ This is a shift in
mindset for many developers used to strong consistency.

To illustrate this, consider a card payment: _"When you make a card transaction, all you're
doing is making a theoretical guarantee that, at some point in the future, that money is
going to move from your bank account to theirs."_ While the system is eventually consistent,
the end result will be correct, just not immediately. Event-driven architecture functions
similarly—updates happen asynchronously, and systems eventually reach a consistent state.

## Handling HTTP communication in an event-driven world

Not every system can fully adopt event-driven architecture, and many still rely on
HTTP-based communication. To integrate these systems into an event-driven world, you need a
middle layer. For example, if your loyalty point service is being replaced by a third-party
SaaS product that only supports an HTTP API, you'd still have a service managing that
integration. This service listens for events and translates them into HTTP requests for
systems that aren't event-driven.

To handle differences in response times and reliability between HTTP-based and event-driven
systems, introducing a queue or intermediary storage is crucial. _"Introducing this queue
means you can keep this amount of durability... you can process to the third-party API as
and when you need to."_ This queue adds resilience, allowing your system to continue
functioning smoothly, even when interacting with external services that don't follow
event-driven principles.

## Asynchronous commands

Commands in an event-driven system don't always need to be synchronous. Instead of waiting
for an immediate response, systems can issue commands asynchronously, allowing for greater
flexibility and non-blocking workflows. _"You want to send an email, but you might not
necessarily want it to be completely request-response."_

An asynchronous command might still send a request to a service, but the response isn't
required to continue processing. This allows systems like the email notification service to
handle requests at its own pace, rather than blocking the core order processing service.
_"Your email service can still expose an endpoint, but as opposed to that being an
HTTP-based endpoint, that could just be a message channel."_ This approach decouples the
services further and ensures more efficient use of resources.

## CQRS for separating reads and writes

Command Query Responsibility Segregation (CQRS) is a powerful pattern that pairs well with
event-driven architecture. CQRS separates the system into two parts: one for handling
commands (writes) and another for handling queries (reads). _"In CQRS, you split your system
into two completely independent services—one for processing commands, one for handling
queries."_ This allows each part of the system to be optimized for its specific workload.

For example, the command service focuses on writing data to the database and publishing
events, while the query service listens for those events and updates a read-optimized view
of the world. This separation enables more efficient scaling, as the query service can be
tuned for fast reads, potentially storing data in caches like Redis or even keeping it in
memory.

I'm a bit skeptical about CQRS since I've worked on a system with a terrible implementation
that went horribly wrong. But I intend to keep an open mind.

## Handling failure with the outbox pattern

In event-driven systems, failure is inevitable, so you need strategies to handle situations
where events fail to publish. The outbox pattern is one such approach. _"At the point you
write the data to the main database... you also write the data to a secondary table."_ This
outbox table ensures that if the event fails to publish initially, it can be retried later.

This creates consistency across the system by acting as a buffer between the database and
the event bus. Alternatively, systems can use change data capture to respond directly to
changes in the database. _"As a record is written to the database, you can stream that—you
can react to that—and you can publish events off the back of that."_ Both methods ensure
reliability, preventing events from being lost due to temporary failures.

The outbox pattern sounds great in theory, but in practice, if you have a large system with
many services publishing to the broker, managing an extra process for each service to read
from the outbox table and publish to the event bus becomes a hassle. Instead, on the
publisher side, retrying with a circuit breaker has worked better for me. Also, fun fact: I
was asked about the outbox pattern in 4 of the last 5 places I interviewed for a backend
role.
