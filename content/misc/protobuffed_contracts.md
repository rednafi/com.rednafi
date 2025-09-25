---
title: Protobuffed contracts
date: 2024-05-10
slug: protobuffed-contracts
aliases:
    - /misc/protobuffed_contracts/
tags:
    - API
    - Networking
---

People typically associate Google's Protocol Buffer[^1] with gRPC[^2] services, and
rightfully so. But things often get confusing when discussing protobufs because the term can
mean different things:

- A binary protocol for efficiently serializing structured data.
- A language used to specify how this data should be structured.

In gRPC services, you usually use both: the protobuf language in proto files defines the
service interface, and then the clients use the same proto files to communicate with the
services.

However, protobuf can be used in non-gRPC contexts for anything that requires a strict
interface. You can optionally choose to use the more compact serialization format that gRPC
tools offer, or just keep using JSON if you prefer. I've seen this use case in several
organizations over the past few years, though I haven't given it much thought. It definitely
has its benefits!

Defining your service contracts with protobuf:

- Allows you to generate message serializers and deserializers in almost any language of
  your choice.
- You can choose from a set of serialization formats.
- The service contracts are self-documented, and you can simply hand over the proto files to
  your service users.
- Different parts of a service or a fleet of services can be written in different languages,
  as long as their communication conforms to the defined contracts.

For example, consider an event-driven application that sends messages to a message broker
when an event occurs. A consumer then processes these messages asynchronously. Both the
producer and consumer need to agree on a message format, which is defined by a contract. The
workflow usually goes as follows:

- Define the message contract using the protobuf DSL.
- Generate the code for serializing/deserializing the messages in the language of your
  choice.
- On the publisher side, serialize the message using the generated code.
- On the consumer side, generate code from the same contract and deserialize the message
  with that.

## Define the contract

You can define your service interface in a `.proto` file. Let's say we want to emit some
event in a search service when a user queries something. The query message structure can be
defined as follows:

```proto
// ./search/protos/message.proto

syntax = "proto3";

message SearchRequest {
  string query = 1;
  int32 page_number = 2;
  int32 results_per_page = 3;
}
```

I'm using proto3 syntax, and you can find more about that on the official guide[^3]. Next,
you can install the gRPC tools for your preferred programming language to generate the
interfacing code that'll be used to serialize and deserialize the messages.

Here's how it looks in Python:

- Install `grpcio-tools`.
- Generate the interface. From the directory where your proto files live, run:

    ```sh
    python -m grpc_tools.protoc -I. \
        --python_out=contracts \
        --grpc_python_out=contracts protos/message.proto
    ```

- This will generate the following files in the root directory:

    ```txt
    search
    ├── contracts
    │   └── protos
    │       ├── message_pb2.py
    │       └── message_pb2_grpc.py
    └── protos
        └── message.proto
    ```

## Serialize and publish

Once you have the contracts in place and have generated the interfacing code, here's how you
can serialize a message payload before publishing it to an event stream:

```py
# ./search/services/publish.py

from contracts.protos.message_pb2 import SearchRequest


def serialize(query: str, page_number: int, results_per_page: int) -> str:
    search_request = SearchRequest(
        query=query, page_number=page_number, results_per_page=results_per_page
    )

    # Serialize the search request to a compact binary string
    return search_request.SerializeToString()


def publish(serialized_message: str) -> None:
    # Publish the message to a message broker
    ...


if __name__ == "__main__":
    serialized_message = serialize("foo bar", 1, 5)
    publish(serialized_message)
```

The code is structured in the following manner now:

```txt
search
├── contracts
│   ├── __init__.py
│   └── protos
│       ├── message_pb2.py
│       └── message_pb2_grpc.py
├── protos
│   └── message.proto
└── services
    ├── __init__.py
    └── publish.py
```

## Deserialize and consume

On the consumer side, if you have access to the proto files, you can generate the interface
code again via the same commands as before and use it to deserialize the message

as follows:

```py
# ./search/services/consume.py

from contracts.protos.message_pb2 import SearchRequest


def get_message() -> str:
    # Let's say we get the message from a message broker and return it
    return b"\n\x04test\x10\x01\x18\x02"


def deserialize(serialized_message: str) -> SearchRequest:
    search_request = SearchRequest()
    search_request.ParseFromString(serialized_message)
    return search_request


def consume(message: SearchRequest) -> None: ...


if __name__ == "__main__":
    serialized_message = get_message()
    search_request = deserialize(serialized_message)
    consume(search_request)
```

You can even save the proto files in a common repo, generate the interfacing code
automatically for multiple languages, and package them up via CI whenever some changes are
merged into the main branch. Then the services can just update those protocol packages and
use the serializers and deserializers as needed.

[^1]: [Protobuf](https://protobuf.dev/)

[^2]: [GRPC](https://grpc.io/)

[^3]: [Proto3 syntax](https://protobuf.dev/programming-guides/proto3/)
