from contracts.protos.message_pb2 import SearchRequest


def consume() -> str:
    # Let's say we get the message from a message broker and return it
    return b"\n\x04test\x10\x01\x18\x02"


def deserialize(serialized_message: str) -> SearchRequest:
    search_request = SearchRequest()
    search_request.ParseFromString(serialized_message)
    return search_request


if __name__ == "__main__":
    serialized_message = consume()
    search_request = deserialize(serialized_message)
    print(search_request)
