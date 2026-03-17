---
title: In praise of the etcd codebase
date: 2026-03-14
slug: etcd-codebase
tags:
    - Go
    - gRPC
    - Distributed Systems
description: >-
  Why the etcd codebase is my go-to reference for building gRPC services in Go.
---

I've been writing a lot of Go gRPC services these days at work - database proxies, metadata
services, storage orchestration control plane, etc. They require me to go a bit deeper into
protobuf and Go gRPC tooling than you'd typically need to. So I started poking around OSS
gRPC codebases to pick up conventions.

I was mainly looking for pointers on how to organize protobuf definitions, wire up server-side
metrics and interceptors, and build ergonomic client wrappers. The default answer here is
often "_go read the Docker or Kubernetes codebase._" But both of those are pretty huge and
take forever to get accustomed to.

Then I found [etcd]. It's used by Kubernetes' control plane for storing configs in a
consistent manner. It exposes a small set of well-defined gRPC endpoints to interact with the
storage layer. The core services are defined in a single [rpc.proto] file:

```proto
service KV {
  rpc Range(RangeRequest) returns (RangeResponse);
  rpc Put(PutRequest) returns (PutResponse);
  rpc DeleteRange(DeleteRangeRequest) returns (DeleteRangeResponse);
  rpc Txn(TxnRequest) returns (TxnResponse);
  rpc Compact(CompactionRequest) returns (CompactionResponse);
}

// ...
```

The full file also defines `Watch`, `Lease`, `Cluster`, `Maintenance`, and `Auth` services.
Grokking that file and the surrounding [api directory] is a good way to learn how to organize
your protobufs and [generated code]. Some other things I picked up:

- Proto definitions live under [api/], separated into subpackages like `etcdserverpb`,
  `mvccpb`, `authpb`. Generated Go code lives alongside the proto files.

- The RPC handler implementations live under [server/etcdserver/api/v3rpc]. [key.go]
  implements the KV service (Range, Put, DeleteRange, Txn, Compact), and the other services
  follow the same pattern in `watch.go`, `lease.go`, `member.go`, `maintenance.go`, `auth.go`.

- [grpc.go] shows how to assemble a gRPC server with chained unary and stream interceptors
  using [go-grpc-middleware].

- Server-side Prometheus metrics are wired in [grpc.go] via `grpc_prometheus.ServerMetrics`
  interceptors. It optionally enables latency histograms when the metric type is `extensive`.

- [metrics.go] defines custom Prometheus counters and histograms on top of the standard gRPC
  ones, things like `etcd_network_client_grpc_sent_bytes_total` and watch stream durations.

- [interceptor.go] handles logging. `newLogUnaryInterceptor` logs request/response sizes at
  warn level when latency exceeds a threshold.

- The client has no built-in metrics. The [clientv3 README] says you can wire up
  [go-grpc-prometheus] yourself, but the library doesn't do it for you.

- [retry_interceptor.go] implements client-side retry with backoff, safe retry classification
  for read-only vs mutation RPCs, and auth token refresh on failure.

- The [clientv3 package] wraps the generated gRPC client behind a nicer Go API. Good
  reference if you're building an ergonomic client on top of raw protobuf types.

- If you're a distributed systems nerd, etcd uses [Raft] for consensus. That part of the
  codebase is its own rabbit hole.

This has become my go-to whenever I'm wiring up another gRPC service at work. I've gotten
comfortable enough with it over the last few weeks that I can point people to specific files
when we need to make decisions.

<!-- references -->
<!-- prettier-ignore-start -->

[etcd]:
    https://github.com/etcd-io/etcd

[rpc.proto]:
    https://github.com/etcd-io/etcd/blob/main/api/etcdserverpb/rpc.proto

[api directory]:
    https://github.com/etcd-io/etcd/tree/main/api

[api/]:
    https://github.com/etcd-io/etcd/tree/main/api

[generated code]:
    https://github.com/etcd-io/etcd/blob/main/api/etcdserverpb/rpc.pb.go

[grpc.go]:
    https://github.com/etcd-io/etcd/blob/main/server/etcdserver/api/v3rpc/grpc.go

[go-grpc-middleware]:
    https://github.com/grpc-ecosystem/go-grpc-middleware

[metrics.go]:
    https://github.com/etcd-io/etcd/blob/main/server/etcdserver/api/v3rpc/metrics.go

[interceptor.go]:
    https://github.com/etcd-io/etcd/blob/main/server/etcdserver/api/v3rpc/interceptor.go

[retry_interceptor.go]:
    https://github.com/etcd-io/etcd/blob/main/client/v3/retry_interceptor.go

[clientv3 package]:
    https://github.com/etcd-io/etcd/tree/main/client/v3

[clientv3 README]:
    https://github.com/etcd-io/etcd/blob/main/client/v3/README.md

[go-grpc-prometheus]:
    https://github.com/grpc-ecosystem/go-grpc-prometheus

[Raft]:
    https://github.com/etcd-io/raft

[server/etcdserver/api/v3rpc]:
    https://github.com/etcd-io/etcd/tree/main/server/etcdserver/api/v3rpc

[key.go]:
    https://github.com/etcd-io/etcd/blob/main/server/etcdserver/api/v3rpc/key.go

<!-- prettier-ignore-end -->
