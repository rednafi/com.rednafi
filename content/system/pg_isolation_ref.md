---
title: "Postgres as an isolation reference model"
slug: pg-isolation-ref
date: 2026-08-19
description: >-
    Use PostgreSQL's transaction isolation levels as a reference model for reasoning about
    the guarantees and tradeoffs of other relational and non-relational databases.
tags:
    - Database
    - SQL
    - "Distributed Systems"
    - Postgres
aliases:
    - /system/pg-return-isolation/
discussions: []
mermaid: false
type_label: ""
atprotoPath: /system/pg-isolation-ref/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mthuqpfscr26"
---

![PostgreSQL transaction isolation levels][image_1]

I've done my share of system design interviews over the years. Some went my way, and others
didn't. One thing stayed the same: transaction isolation almost always came up whenever the
problem involved an [online system].

When the requirements call for a [CP system], I usually reach for a PostgreSQL-compatible
NewSQL database such as Aurora PostgreSQL, DSQL, or CockroachDB. [Andy Pavlo and Matthew
Aslett define NewSQL] as relational databases that aim for NoSQL-like scale while keeping
SQL and ACID transactions.

Part of that is because I like thinking in Postgres. There's also the usual interview move:
the system starts at a modest scale, then the interviewer cranks it up by 100x for shits and
giggles. With regular Postgres, that may mean manual sharding or switching databases. Either
option can change the transaction boundary and force a substantial redesign. NewSQL lets me
keep the relational model and distributed transactions as the system grows.

That still leaves the isolation question. Postgres is my reference model. If MongoDB,
DynamoDB, Cassandra, or another database fits the problem better, I compare its guarantees
with Postgres's _[Read Committed]_, _[Repeatable Read]_, and _[Serializable]_ isolation
levels. From there, I can name the closest Postgres equivalent and explain where the
guarantees differ.

The same approach works outside interviews. When I run into an unfamiliar OLTP or SQL OLAP
database, I compare its snapshot semantics and conflict handling with Postgres before
trusting the isolation-level name. This is especially useful for analytical databases that
offer snapshot reads but no multi-statement transactions. Even when both use MVCC, a
statement-level snapshot behaves more like Postgres's _Read Committed_ than _Repeatable
Read_. ClickHouse is a good example.

Postgres gives me a concrete reference model that's close to the SQL standard.

- _Read Committed_ takes a new snapshot for each statement, so two reads in the same
  transaction can see different committed states.
- _Repeatable Read_ uses one snapshot for the transaction and prevents non-repeatable and
  phantom reads, but serialization anomalies such as write skew can happen.
- _Serializable_ adds dependency tracking through _Serializable Snapshot Isolation (SSI)_
  and aborts transactions that can't be ordered serially.

Here is how I stack a few common databases against that model.

| Database      | Transaction and isolation model                                                                                                                                                                                     | Closest Postgres reference             | Highlights                                                                                                                                                                                                                                                           |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PostgreSQL    | Interactive transactions across rows and tables. _Read Committed_ is the default, _Repeatable Read_ provides snapshot isolation, and _Serializable_ uses SSI.                                                       | Baseline                               | _Repeatable Read_ can allow write skew. _Serializable_ may return `40001`, requiring the application to retry the entire transaction.                                                                                                                                |
| [Aurora DSQL] | Interactive distributed transactions using [strong snapshot isolation] and optimistic concurrency control.                                                                                                          | _Repeatable Read_                      | DSQL detects write-write conflicts but doesn't validate ordinary read-write dependencies. Write skew can happen. Conflicts return `40001`, and `SELECT FOR UPDATE` can make the relevant read dependencies participate in conflict detection.                        |
| [MongoDB]     | Single-document operations are atomic. Multi-document transactions can span collections, databases, and shards. `snapshot` read concern with majority write concern provides a synchronized snapshot across shards. | _Repeatable Read_                      | MongoDB doesn't have a general _Serializable_ level. Read concern controls visibility, while write concern controls durability. Write skew can happen in snapshot transactions, and write conflicts require retries.                                                 |
| [DynamoDB]    | `TransactWriteItems` and `TransactGetItems` provide one-shot transactions over at most 100 known items. Transactional operations are _Serializable_ relative to other transactions and individual item operations.  | _Serializable_ over a bounded item set | The guarantee covers the items named in the request. `Query`, `Scan`, and `BatchGetItem` are _Read Committed_ as aggregate operations. There's no interactive read-compute-write transaction, and global tables don't preserve transaction atomicity across regions. |

Cassandra is a useful edge case because there isn't an isolation level that lines up with
Postgres. A normal CQL read or write stands on its own. You can't open a _Read Committed_ or
_Repeatable Read_ transaction and run several statements against one snapshot. [Batches]
group writes, but isolation stops at the partition boundary. [Lightweight transactions] use
Paxos for linearizable compare-and-set, which is the closest match to _Serializable_. That
guarantee covers only a conditional change inside one partition. Postgres can enforce a
wider invariant inside a _Serializable_ transaction. With Cassandra, I have to keep that
invariant in one partition or coordinate it in the application.

Rather than learn every database's isolation semantics from scratch, I map its behavior to
the closest Postgres level, then focus on gaps in guarantees and transaction scope. This has
worked well for me so far.

---

Snapshot isolation is my favorite isolation level in any database that offers it. It catches
write-write conflicts. _Serializable_ catches those too, along with read-write dependencies.
That stronger guarantee prevents write skew, which snapshot isolation allows.

But that stronger guarantee comes with more bookkeeping and can lower throughput. SSI tracks
the rows and predicates each transaction reads. Large read sets and broad predicates can
overlap more writes and cause more serialization failures. Avoiding those failures often
means narrowing the transaction scope or skipping reads that would make the code easier to
reason about.

With snapshot isolation, I can often prevent write skew directly. If an invariant maps to
existing rows, `SELECT FOR UPDATE` can lock them and force competing transactions to
conflict. If the invariant depends on missing rows or an arbitrary predicate, there may be
nothing to lock, so I use _Serializable_. I tend to avoid designs like that. In practice,
snapshot isolation with targeted locks gives me what I need without paying the full cost of
_Serializable_. Marc Brooker makes a similar case in this [fantastic post].

<!-- references -->
<!-- prettier-ignore-start -->

[online system]:
    https://www.ibm.com/think/topics/oltp

[cp system]:
    https://martin.kleppmann.com/2015/05/11/please-stop-calling-databases-cp-or-ap.html

[andy pavlo and matthew aslett define newsql]:
    https://www.cs.cmu.edu/~pavlo/papers/pavlo-newsql-sigmodrec2016.pdf

[read committed]:
    https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED

[repeatable read]:
    https://www.postgresql.org/docs/current/transaction-iso.html#XACT-REPEATABLE-READ

[serializable]:
    https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE

[aurora dsql]:
    https://docs.aws.amazon.com/aurora-dsql/latest/userguide/working-with-concurrency-control.html

[mongodb]:
    https://www.mongodb.com/docs/manual/core/transactions/

[dynamodb]:
    https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/transaction-apis.html

[strong snapshot isolation]:
    https://arxiv.org/html/2607.13276v2#S2.SS4

[batches]:
    https://cassandra.apache.org/doc/stable/cassandra/developing/cql/dml.html#batch_statement

[lightweight transactions]:
    https://cassandra.apache.org/doc/stable/cassandra/architecture/guarantees.html

[fantastic post]:
    https://www.linkedin.com/posts/marc-brooker-b431772b_that-would-be-fun-heres-the-shape-of-activity-7469787469589311488-OzI9

[image_1]:
    https://blob.rednafi.com/system/pg-return-isolation/postgresql-isolation-levels-dafec4753c2c.png

<!-- prettier-ignore-end -->
