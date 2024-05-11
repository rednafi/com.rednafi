from contracts.protos.message_pb2 import SearchRequest


def serialize(query: str, page_number: int, results_per_page: int) -> str:
    search_request = SearchRequest(
        query=query, page_number=page_number, results_per_page=results_per_page
    )

    # Serialize the search request to a compact binary string
    return search_request.SerializeToString()


def publish(serialized_message: str) -> None:
    # Publish the message to a message broker
    pass


if __name__ == "__main__":
    serialized_message = serialize("foo bar", 1, 5)
    publish(serialized_message)
