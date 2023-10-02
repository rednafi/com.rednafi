---
title: Declaratively transform dataclass fields in Python
date: 2022-03-20
tags:
    - Python
---

While writing microservices in Python, I like to declaratively define the shape of the data
coming in and out of JSON APIs or NoSQL databases in a separate module. Both `TypedDict` and
`dataclass` are fantastic tools to communicate the shape of the data with the next person
working on the codebase.

Whenever I need to do some processing on the data before starting to work on that, I prefer
to transform the data via dataclasses. Consider this example:

```python
# src.py
from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass
class WebhookPayload:
    """Save data to DynamoDB."""

    url: str
    request_payload: dict
    response_payload: dict
    status_code: int

    def to_dynamodb_item(self) -> None:
        ...
```

The above class defines the structure of a payload that'll be saved in a DynamoDB table. To
make things simpler, I want to serialize the `request_payload`, `response_payload`, and
`status_code` fields to JSON string before saving them to the DB. Usually, I'd do it in the
`to_dynamodb_item` like this:

```python
# src.py
from __future__ import annotations

from dataclasses import dataclass
import json
from typing import Any


@dataclass
class WebhookPayload:
    """Save data to DynamoDB."""

    # Snip...

    def to_dynamodb_item(self) -> dict[str, Any]:
        request_payload = json.dumps(self.request_payload)
        response_payload = json.dumps(self.response_payload)
        status_code = json.dumps(self.response_payload)
        # ...
        dyanmodb_item = ...
        return dynamodb_item
```

However, keeping track of this `json.dumps` transformation that's buried in a method can be
difficult. Also, it can be hard to track the fields that need to be deserialized whenever
you want the rich data structures back. Another disadvantage is that you'll have to perform
the same transformation again if you need serialized fields in another method. A better way
is to take advantage of the `__post_init__` hook exposed by dataclasses. Here's how you can
do it:

```python
# src.py
from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any


@dataclass
class WebhookPayload:
    """Save data to DynamoDB."""

    url: str
    request_payload: dict | None
    response_payload: dict | None
    status_code: int | None
    _json_transform: bool = field(default=True, repr=False)
    _json_fields: tuple[str, ...] = field(
        default=(
            "request_payload",
            "response_payload",
            "status_code",
        ),
        repr=False,
    )

    def __post_init__(self) -> None:
        if not self._json_transform:
            return

        for field in self._json_fields:
            # Here's where the magic happens!
            setattr(self, field, json.dumps(getattr(self, field)))

    def to_dynamodb_item(self) -> dict[str, Any]:
        ...


if __name__ == "__main__":
    wh = WebhookPayload(
        url="https//:httpbin.org/post",
        request_payload={"hello": "world"},
        response_payload=None,
        status_code=None,
    )
    print(wh)
```

Running the script will print the following:

```txt
WebhookPayload(
    url='https//:httpbin.org/post',
    request_payload='{"hello": "world"}',
    response_payload='null',
    status_code='null'
)
```

Notice, how the intended fields are now JSON encoded. Python calls the `__post_init__` hook
of a dataclass after calling the `__init__` method. If you don't generate any init by
decorating the target class with `@dataclass(init=False)`, in that case, the `__post_init__`
hook won't be executed.

The `field` function with `repr=False` allows us to exclude the configuration fields like
`_json_transform` and `_json_fields` from the final `__repr__` of the class. Notice that
these two fields are absent in the final representation of the dataclass instance.

You can turn off the JSON conversion by setting the `_json_transform` to `False`:

```python
# src.py

...
WebhookPayload(
    url="https//:httpbin.org/post",
    request_payload={"hello": "world"},
    response_payload=None,
    status_code=None,
    _json_transform=False,
)
```

You can also add or remove fields to be transformed by changing the value of the
`_json_fields` iterable of the class:

```python
# src.py

...
WebhookPayload(
    url="https//:httpbin.org/post",
    request_payload={"hello": "world"},
    response_payload=None,
    status_code=None,
    _json_fields=("status_code",),
)
```

This will only serialize the `status_code` field. Neat!

## References

[^1]:
    [Post-init processing — Python docs](https://docs.python.org/3/library/dataclasses.html#post-init-processing)
    [^1]
