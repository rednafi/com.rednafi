---
title: "Dynamo: Amazon's Highly Available Key-value Store"
date: 2026-04-11
slug: dynamo
tags:
    - Distributed Systems
    - Databases
description: >-
  Key takeaways from Amazon's 2007 Dynamo paper.
---

Finally got around to reading the original [Dynamo paper] from 2007. It's the one that
kicked off Cassandra, Riak, Voldemort, and a whole generation of eventually consistent
stores. Added it to my [papers] page.

Amazon had services like the shopping cart where consistency wasn't worth the availability
cost. If a node is unreachable in a consistent system, writes block or fail.
A stale cart is fine; an unavailable cart loses
money. So the idea with Dynamo is to never reject a write. If a network partition causes
two nodes to accept conflicting writes, both versions survive and the application sorts
it out on the next read. For the shopping cart that means taking the union of conflicting
versions - a deleted item might reappear, but nothing gets lost.

Data is partitioned on a [consistent hash] ring with [virtual nodes]. Each key replicates
to N nodes (typically 3), any of which can accept writes. There are no leaders or
followers - every node is identical. [Quorum] parameters R and W are tunable, and (N=3,
R=2, W=2) is the common setup where you get overlap between reads and writes.

If one of the N replicas is down, [sloppy quorum] lets the write land on the next healthy
node on the ring instead. That node holds the data and forwards it via [hinted handoff]
once the original recovers. [Merkle trees] handle
background [anti-entropy] to sync divergent replicas, and [gossip] handles failure
detection with no central membership service anywhere.

[Vector clocks] track causal ordering so the system knows when two writes genuinely
conflict rather than one superseding the other. When that happens, Dynamo keeps both
versions and hands them back on the next read - the application has to reconcile them.
They also had to truncate old clock entries to keep the metadata bounded, which can
lose ordering info and create even more conflicts for the app to sort out.

Table 1 from the paper summarizes the architecture:

| Problem                            | Technique                                                | Advantage                                                                                                        |
| ---------------------------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Partitioning                       | [Consistent hash]ing                                     | Incremental scalability                                                                                          |
| High availability for writes       | [Vector clocks] with reconciliation during reads         | Version size is decoupled from update rates                                                                      |
| Handling temporary failures        | [Sloppy quorum] and [hinted handoff]                     | Provides high availability and durability guarantee when some of the replicas are not available                  |
| Recovering from permanent failures | [Anti-entropy] using [Merkle trees]                      | Synchronizes divergent replicas in the background                                                                |
| Membership and failure detection   | [Gossip]-based membership protocol and failure detection | Preserves symmetry and avoids having a centralized registry for storing membership and node liveness information |

AWS's own DynamoDB service eventually dropped vector clocks and moved to leader-based
replication with Multi-Paxos per partition. The [2022 DynamoDB paper] covers the shift.
Turns out, pushing conflict resolution onto application developers didn't scale as a
product decision even if it scaled as an architecture.

<!-- references -->
<!-- prettier-ignore-start -->

[Dynamo paper]:
    https://www.allthingsdistributed.com/files/amazon-dynamo-sosp2007.pdf

[papers]:
    /papers/

[consistent hash]:
    https://arpitbhayani.me/blogs/consistent-hashing/

[virtual nodes]:
    https://arpitbhayani.me/blogs/consistent-hashing/

[quorum]:
    https://web.mit.edu/6.033/2005/wwwdocs/quorum_note.html

[sloppy quorum]:
    https://distributed-computing-musings.com/2022/05/sloppy-quorum-and-hinted-handoff-quorum-in-the-times-of-failure/

[hinted handoff]:
    https://systemdesign.one/hinted-handoff/

[vector clocks]:
    https://sookocheff.com/post/time/vector-clocks/

[Merkle trees]:
    https://brilliant.org/wiki/merkle-tree/

[anti-entropy]:
    https://www.influxdata.com/blog/eventual-consistency-anti-entropy/

[gossip]:
    https://highscalability.com/gossip-protocol-explained/

[2022 DynamoDB paper]:
    https://www.usenix.org/conference/atc22/presentation/elhemali

<!-- prettier-ignore-end -->
