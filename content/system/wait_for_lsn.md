---
title: Reading your own writes with WAIT FOR LSN in Postgres 19
date: 2026-06-12
slug: wait-for-lsn
atprotoPath: /system/wait-for-lsn/
tags:
    - Database
    - SQL
    - Distributed Systems
    - Postgres
description: >-
  PostgreSQL 19's new WAIT FOR LSN command lets a replica block until it has replayed
  your write. The read-after-write problem it solves, the workarounds it replaces, and what
  the timeout, status, and mode options are actually for.
---

Postgres 19 finally gives us a clean way to do read-after-write across replicas. Without it,
here's the problem:

- you write a row to the primary
- then you immediately read it back and that query goes to a replica
- but the replica hasn't replayed the write yet, so you get stale data or nothing at all

The usual workarounds are:

- sleep after the write and hope the replica caught up (terrible, don't do it)
- pin the user to the primary for a few seconds
- or poll the replica until the write shows up

The new [WAIT FOR LSN] command replaces all of that. It lets the replica block until it has
replayed up to your write. MySQL has had this for ages with [SOURCE_POS_WAIT()]; Postgres
never did, so I'm glad to see it land in [Postgres 19 beta 1]. Of everything in this
release, it's the feature I'm most excited about.

## Reproducing the stale read

A stale read only shows up when there's real replication lag, so reproducing one needs real
replication. The setup here is two Postgres containers running locally from the
`postgres:19beta1` image: a primary, and a `pg_basebackup` replica streaming from it.
Postgres calls a streaming replica like this a standby, and that's the word you'll see in
its config settings and its error messages.

Replication is asynchronous. The primary commits the moment a write reaches its write-ahead
log (WAL). The replica replays that WAL slightly later. In production that gap is a few
milliseconds, and with enough traffic some read lands inside it. Locally, running one query
at a time, you would never catch it. To create that gap on purpose, the replica runs with
[recovery_min_apply_delay] set to `10s`, a standby setting that holds each commit for a
fixed delay before applying it. So it still receives every WAL record the instant the
primary sends it, but waits ten seconds before applying each one.

All of it is wired up in [a small script]; pipe it to your shell, and pass a longer delay
like `sh -s 30s` if you want a wider window:

```sh
curl -fsSL https://gist.githubusercontent.com/rednafi/6812c61fd022715e1d94989d49077324/raw/wait_for_lsn_lab.sh | sh
```

That leaves two containers running. Open a psql session to each in separate terminals;
that's where the `-- on the primary` and `-- on the replica` snippets below run:

```sh
docker exec -it pg-primary psql -U postgres   # terminal 1: primary
docker exec -it pg-replica psql -U postgres   # terminal 2: replica, 10s behind
```

Create a table on the primary and seed it with a row:

```sql
-- on the primary
CREATE TABLE users (id serial PRIMARY KEY, name text);
INSERT INTO users (name) VALUES ('alice');
```

The replica runs ten seconds behind, so right after that it hasn't applied the
`CREATE TABLE` yet. Read it immediately and the table isn't even there:

```sql
-- on the replica
SELECT * FROM users;
```

```txt
ERROR:  relation "users" does not exist
LINE 1: SELECT * FROM users;
                      ^
```

Ten seconds on, the commit lands and the read works:

```sql
-- on the replica
SELECT * FROM users;
```

```txt
 id | name
----+-------
  1 | alice
(1 row)
```

So the replica does keep up; it just runs a fixed ten seconds behind the primary. Any read
that lands inside that ten-second window comes back stale.

## The usual workarounds

The crudest is to sleep before the read. Drop a 100 ms pause between the write and the read
and hope the lag stays under it. But lag isn't a constant. The pause comes out too long for
the common case and too short for the bad one, and every request pays it even when the
replica had already caught up.

Sticky reads are a step up. After a user writes, route their reads to the primary for a few
seconds. Plenty of routing layers do exactly this. But now something has to track who wrote
recently, the primary serves reads the replicas were meant to absorb, and "a few seconds" is
still a guess.

There's also the heavyweight option: make every write wait. Set [synchronous_commit] to
`remote_apply`, and commits on the primary block until synchronous standbys have applied the
WAL.

Reads are always fresh, but the cost lands in the wrong place. Every commit pays for the
replication round trip, including the ones nobody is ever going to read back. And a single
struggling standby drags down every writer in the cluster.

If you want to do it properly, you poll the replica. Right after the write, you grab the
commit's LSN, its log sequence number, which is just a byte position in the WAL, with
[pg_current_wal_insert_lsn()]. Then you ask the replica, over and over, whether its replay
has reached that point yet:

```sql
-- on the replica
SELECT pg_last_wal_replay_lsn() >= '0/0307B038'::pg_lsn;
```

```txt
 ?column?
----------
 f
(1 row)
```

You loop until it flips to `t`, and only then do the read. It works, but now you own a
busy-wait: you have to pick a poll interval, set a deadline, and decide what happens when
that deadline passes. Every codebase that does this ends up with its own slightly different
version of the loop.

Polling is the right idea with bad ergonomics. The server knows the exact moment replay
passes an LSN. There was just no way to ask it to block until that happened.

## WAIT FOR LSN

`WAIT FOR LSN` blocks until the replica has replayed up to an LSN you give it. The server
wakes the session the moment replay passes the mark, so there's no loop to write and no
busy-wait to own. The [synopsis]:

```sql
WAIT FOR LSN 'lsn'
    [ WITH ( option [, ...] ) ]
```

It takes three options: `MODE`, `TIMEOUT`, and `NO_THROW`. Leave them off and you get the
defaults: the `standby_replay` mode, no timeout, and a thrown error if the wait can't be
met.

End to end, a correct read-after-write is four steps:

1. write on the primary
2. on that same connection, right after the commit, grab the LSN with
   `pg_current_wal_insert_lsn()`
3. carry that LSN to wherever the read happens: a session store, a cookie, whatever the
   client echoes back
4. on the replica, run `WAIT FOR LSN` with that LSN, then do the read

Run it on a real write. On the primary, insert a row and grab its LSN on the same
connection, which covers steps 1 and 2:

```sql
-- on the primary
INSERT INTO users (name) VALUES ('bob');
SELECT pg_current_wal_insert_lsn();
```

```txt
INSERT 0 1
 pg_current_wal_insert_lsn
---------------------------
 0/0307B038
(1 row)
```

Your own value will differ, so use whatever your primary prints. bob is committed now, but
the replica is still ten seconds behind, so a read over there can't see him yet. That's why,
before the read, you wait on the LSN you just captured (step 4):

```sql
-- on the replica
WAIT FOR LSN '0/0307B038';
```

It blocks for the rest of the ten-second delay, then returns the instant replay reaches the
LSN:

```txt
 status
---------
 success
(1 row)
```

And now the read is guaranteed to see bob:

```sql
-- on the replica
SELECT * FROM users;
```

```txt
 id | name
----+-------
  1 | alice
  2 | bob
(2 rows)
```

> [!WARNING]
>
> Grab the LSN after you commit, not while the transaction is still open. If you call
> `pg_current_wal_insert_lsn()` before the commit, the commit record doesn't exist yet, so
> your real commit lands at a higher LSN than the one you captured. Wait on that lower
> number and it can return before your row is actually visible.

## Timeouts and statuses

By default the wait has no deadline. As the docs put it:

> If no timeout is specified or it is set to zero, this command waits indefinitely.

A replica that's wedged or hours behind would leave your request hanging for exactly that
long, so in practice you bound it with `TIMEOUT`. To watch one fire, commit a third row on
the primary and grab its LSN:

```sql
-- on the primary
INSERT INTO users (name) VALUES ('carol');
SELECT pg_current_wal_insert_lsn();
```

```txt
INSERT 0 1
 pg_current_wal_insert_lsn
---------------------------
 0/0307B120
(1 row)
```

Then wait on it inside the ten-second window, but give it only two seconds:

```sql
-- on the replica
WAIT FOR LSN '0/0307B120' WITH (TIMEOUT '2s');
```

```txt
ERROR:  timed out while waiting for target LSN 0/0307B120 to be replayed; current standby_replay LSN 0/0307B0F8
```

By default, a wait that doesn't succeed raises an error, so your application has to catch
it. Add `NO_THROW` and it returns a status instead of raising:

```sql
-- on the replica
WAIT FOR LSN '0/0307B120' WITH (TIMEOUT '2s', NO_THROW);
```

```txt
 status
---------
 timeout
(1 row)
```

The status is one of `success`, `timeout`, or `not in recovery`. That last one means the
server you asked isn't a standby. You'd see it if you ran the command on the primary by
mistake, which without `NO_THROW` is an error:

```sql
-- on the primary
WAIT FOR LSN '0/0307B120';
```

```txt
ERROR:  recovery is not in progress
HINT:  Waiting for the standby_replay LSN can only be executed during recovery.
```

```sql
-- on the primary
WAIT FOR LSN '0/0307B120' WITH (NO_THROW);
```

```txt
     status
-----------------
 not in recovery
(1 row)
```

You'd also see it if the standby gets promoted while a wait is in flight. Promote it while a
session is waiting on a far-off LSN, and that session comes back with:

```txt
ERROR:  recovery is not in progress
DETAIL:  Recovery ended before target LSN 99/00000000 was replayed; last standby_replay LSN 0/0307B240.
```

Promotion starts a new timeline, so the docs say to re-evaluate whether the LSN you're
holding still means anything.

That leaves the read path with three branches:

- `success`: read from the replica
- `timeout`: fall back to the primary, or knowingly serve stale data
- `not in recovery`: the topology changed, so re-check which server is the primary before
  retrying

## The other modes

> [!TIP]
>
> Everything so far has used the default mode, `standby_replay`. For read-after-write it's
> the one you want, and probably the only one you'll ever touch. The other three are about
> durability, not read visibility.

`MODE` picks which milestone the wait blocks on. Besides the default it takes [three more
values]. A committed write reaches a standby in three stages, and there's a mode for each:

- `standby_write`: the WAL is written to the standby's OS, though it may still sit in OS
  buffers
- `standby_flush`: the WAL is flushed to the standby's disk
- `standby_replay`: the WAL is applied, so a `SELECT` on the standby can see it

`standby_replay` is the default and the only one of the three about visibility. The other
two stop earlier, once the WAL has reached the standby but before it's applied. Streaming
and replay are separate steps, and the apply delay only slows replay, so carol's WAL is
already written and flushed on the standby even though its replay is still ten seconds out.
The LSN that timed out under `standby_replay` a moment ago comes back instantly under both:

```sql
-- on the replica, still inside the apply delay
WAIT FOR LSN '0/0307B120' WITH (MODE 'standby_write');
```

```txt
 status
---------
 success
(1 row)
```

```sql
-- on the replica, still inside the apply delay
WAIT FOR LSN '0/0307B120' WITH (MODE 'standby_flush');
```

```txt
 status
---------
 success
(1 row)
```

That looks useless until you remember asynchronous replication can lose a write for good:
the primary acknowledges a commit, then crashes before any standby has the WAL, and the row
goes with it.

For a write that can't tolerate that, say it moves money, you commit it on the primary, grab
its LSN, and then wait for a standby to confirm it has the WAL before you report success.
`standby_flush` is the mode to reach for: once it returns, the WAL is on the standby's disk,
so the row survives a crash on the primary and the failover to that standby.

`standby_write` is cheaper and weaker, since the bytes may still be in OS buffers when it
returns. Either way the cost falls only on the writes that ask for it, instead of switching
the whole cluster to synchronous replication.

`primary_flush` is the odd one out. It runs on the primary rather than a standby, and waits
for the primary's own WAL to reach disk:

```sql
-- on the primary
WAIT FOR LSN '0/0307B120' WITH (MODE 'primary_flush');
```

```txt
 status
---------
 success
(1 row)
```

It came back at once here because that WAL was flushed long ago. Where it pays off is under
`synchronous_commit = off`, where a `COMMIT` returns before its WAL is fsynced, so a crash
can drop the last few rows you thought were committed. Fire a batch of those fast commits,
then one `primary_flush` wait on the latest LSN confirms everything up to it is on disk.
Once it returns, `pg_current_wal_flush_lsn()` has reached or passed your target.

## Restrictions

`WAIT FOR LSN` only runs as a top-level statement, and only when no snapshot is open. Wrap
it in a function or a `DO` block and Postgres refuses:

```sql
DO $$
BEGIN
    EXECUTE 'WAIT FOR LSN ''0/0307B120''';
END $$;
```

```txt
ERROR:  WAIT FOR can only be executed as a top-level statement
DETAIL:  WAIT FOR cannot be used within a function, procedure, or DO block.
CONTEXT:  SQL statement "WAIT FOR LSN '0/0307B120'"
PL/pgSQL function inline_code_block line 3 at EXECUTE
```

Once a transaction above `READ COMMITTED` has taken a snapshot, it holds that snapshot for
the rest of the transaction, so those are out too:

```sql
BEGIN ISOLATION LEVEL REPEATABLE READ;
SELECT 1;
WAIT FOR LSN '0/0307B120';
ROLLBACK;
```

```txt
BEGIN
 ?column?
----------
        1
(1 row)

ERROR:  WAIT FOR must be called without an active or registered snapshot
DETAIL:  WAIT FOR cannot be executed within a transaction with an isolation level higher than READ COMMITTED.
ROLLBACK
```

At `READ COMMITTED` you can still run the wait inside a transaction, even after other
queries, since the snapshot is dropped between statements. It's only blocked once a snapshot
is pinned: by `REPEATABLE READ` or higher, an open cursor, or a surrounding function.

> [!TIP]
>
> The simplest habit is to issue `WAIT FOR LSN` on its own, right before the read it's
> guarding.

Two more things the docs call out:

- a wait on a standby can be interrupted by recovery conflicts, so the command can fail for
  reasons unrelated to your LSN, and retrying is on you
- LSNs know nothing about timelines, so after a failover the same number can mean a
  different history, which is exactly what the `not in recovery` status is warning you about

## Why it took three tries

Postgres tried to ship this twice before 19 and pulled it back both times. A stored
procedure called `pg_wal_replay_wait()` was committed during the Postgres 17 cycle and
reverted before release. It was [committed again for 18, and reverted again]. No released
version ever shipped it.

The blocker is the snapshot rule from the restrictions above. A query on a standby holds a
snapshot, and while it's alive Postgres can't discard the row versions that snapshot might
still need. That collides with replication: replaying WAL sometimes removes old row
versions, say after a vacuum on the primary, so replay stalls behind any query holding an
older snapshot.

Now picture the session waiting for replay also holding a snapshot. It's waiting for replay
to advance while replay waits for its snapshot to go away. Deadlock. The [commit that landed
in 19] cites exactly this as why a function can't do the job: a function always runs inside
a query, and that query holds a snapshot. The stored-procedure version kept hitting the same
wall, so 19 made it a top-level command instead.

And it's still beta 1. GA should land around September or October. Given the history above I
wouldn't wire this into anything important yet, but for the first time it feels like it'll
actually stick.

<!-- References -->
<!-- prettier-ignore-start -->

[postgres 19 beta 1]:
    https://www.postgresql.org/about/news/postgresql-19-beta-1-released-3313/

[source_pos_wait()]:
    https://dev.mysql.com/doc/refman/8.4/en/replication-functions-synchronization.html

[wait for lsn]:
    https://www.postgresql.org/docs/19/sql-wait-for.html

[a small script]:
    https://gist.github.com/rednafi/6812c61fd022715e1d94989d49077324

[recovery_min_apply_delay]:
    https://www.postgresql.org/docs/current/runtime-config-replication.html#GUC-RECOVERY-MIN-APPLY-DELAY

[pg_current_wal_insert_lsn()]:
    https://www.postgresql.org/docs/current/functions-admin.html

[synchronous_commit]:
    https://www.postgresql.org/docs/current/runtime-config-wal.html#GUC-SYNCHRONOUS-COMMIT

[committed again for 18, and reverted again]:
    https://pgpedia.info/p/pg_wal_replay_wait.html

[commit that landed in 19]:
    https://postgr.es/c/447aae13b

[synopsis]:
    https://www.postgresql.org/docs/19/sql-wait-for.html

[three more values]:
    https://postgr.es/c/49a181b5d

<!-- prettier-ignore-end -->
