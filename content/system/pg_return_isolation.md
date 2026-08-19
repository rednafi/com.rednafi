---
title: "A return to Postgres isolation"
slug: pg-return-isolation
date: 2026-08-19
description: >-
    Use PostgreSQL's transaction isolation levels as a reference model for reasoning about
    the guarantees and tradeoffs of other relational and non-relational databases.
tags:
    - Database
    - SQL
    - "Distributed Systems"
    - Postgres
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /system/pg-return-isolation/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mthp24j6pl2p"
---

![PostgreSQL transaction isolation levels][image_1]

Over the years, I've participated in a few system design interviews. Some went my way and
others didn't. One thing has stayed consistent: when I'm designing an [online system],
transaction isolation almost always comes up in the discussion.

When the requirements call for a [CP system], I usually go for a PostgreSQL-compatible
NewSQL database such as Aurora PostgreSQL, DSQL, or CockroachDB. [Andy Pavlo and Matthew
Aslett define NewSQL] as relational databases that aim for NoSQL-like scale while keeping
SQL and ACID transactions.

Part of the reason is that I like thinking in Postgres. There's also the usual interview
move where a system starts at a modest scale and then the interviewer cranks it up by 100x
for shits and giggles. With shared-nothing Postgres, I may suddenly need manual sharding or
a different database. Either choice can change the transaction boundary and force a
substantial redesign. NewSQL lets me keep the relational model and distributed transactions
across a wider range of scales.

That still leaves the isolation question. I use Postgres as the reference model. If MongoDB,
DynamoDB, Cassandra, or another database fits the problem better, I compare its guarantees
with Postgres's _[Read Committed]_, _[Repeatable Read]_, and _[Serializable]_ isolation
levels. I can then describe the closest Postgres equivalent and explain the differences.

I use the same approach outside interviews. When I run into an unfamiliar OLTP or SQL OLAP
database, I compare its snapshot semantics and conflict handling with Postgres before
trusting the isolation-level name. This is especially useful for analytical databases that
offer snapshot reads but don't support a multi-statement transaction. A statement-level
snapshot is closer to Postgres's _Read Committed_ behavior than to its _Repeatable Read_
isolation, even if both use MVCC underneath. Think ClickHouse.

Postgres gives me a concrete reference model that is pretty close to the SQL standard.

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

Cassandra is a useful edge case because there isn't an isolation level to line up with
Postgres. A normal CQL read or write stands on its own. You can't open a _Read Committed_ or
_Repeatable Read_ transaction and run several statements against one snapshot. [Batches]
group writes, but isolation stops at the partition boundary. [Lightweight transactions] use
Paxos for linearizable compare-and-set, which is the closest match to _Serializable_. It
only covers a conditional change inside one partition. Postgres can keep a wider invariant
inside a _Serializable_ transaction. With Cassandra, I have to keep that invariant in one
partition or coordinate it in the application.

I don't need to memorize every database as a separate isolation model. I start with
Postgres, find the closest level, and spend the rest of the discussion on the differences.
It's worked well for me so far.

---

Snapshot isolation is my favorite isolation level in any database that offers it. In
Postgres, that's _Repeatable Read_. It detects write-write conflicts. _Serializable_ does
that too, but SSI also tracks the rows and predicates each transaction reads so it can
detect read-write dependencies. This allows SSI to prevent write skew.

That bookkeeping grows with the read set, and broad reads can overlap more writes and cause
more serialization failures. When an invariant maps to existing rows, `SELECT FOR UPDATE`
can force competing transactions to conflict on those rows and prevent write skew. If the
invariant depends on missing rows or an arbitrary predicate, there may be nothing to lock,
so you'll need to use _Serializable_. But I tend to avoid that kind of design in practice.

Marc Brooker has a [fantastic post] on why snapshot isolation makes sense for
high-performance systems.

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
    https://brooker.co.za/blog/2024/01/23/big-deal.html

[image_1]:
    https://blob.rednafi.com/system/pg-return-isolation/postgresql-isolation-levels-dafec4753c2c.png

<!-- prettier-ignore-end -->
