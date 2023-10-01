---
title: Declarative payloads with TypedDict in Python
date: 2022-03-11
tags:
    - Python
    - Typing
---

While working with microservices in Python, a common pattern that I see is—the usage of
dynamically filled dictionaries as payloads of REST APIs or message queues. To understand
what I mean by this, consider the following example:

```python
# src.py
from __future__ import annotations

import json
from typing import Any

import redis  # Do a pip install.


def get_payload() -> dict[str, Any]:
    """Get the 'zoo' payload containing animal names and attributes."""

    payload = {"name": "awesome_zoo", "animals": []}

    names = ("wolf", "snake", "ostrich")
    attributes = (
        {"family": "Canidae", "genus": "Canis", "is_mammal": True},
        {"family": "Viperidae", "genus": "Boas", "is_mammal": False},
    )
    for name, attr in zip(names, attributes):
        payload["animals"].append(  # type: ignore
            {"name": name, "attribute": attr},
        )
    return payload


def save_to_cache(payload: dict[str, Any]) -> None:
    # You'll need to spin up a Redis db before instantiating
    # a connection here.
    r = redis.Redis()
    print("Saving to cache...")
    r.set(f"zoo:{payload['name']}", json.dumps(payload))


if __name__ == "__main__":
    payload = get_payload()
    save_to_cache(payload)
```

Here, the `get_payload` function constructs a payload that gets stored in a Redis DB in the `save_to_cache` function. The `get_payload` function returns a dict that denotes a contrived
payload containing the data of an imaginary zoo. To execute the above snippet, you'll need
to spin up a Redis database first. You can use Docker[^1] to do so.
Install and configure Docker on your system and run:

```sh
docker run -d -p 6379:6379 redis:alpine
```

If you run the above snippet after instantiating the Redis server, it'll run without
raising any error. You can inspect the content saved in Redis with the following command
(assuming you've got `redis-cli` and `jq` installed in your system):

```txt
echo "get zoo:awesome_zoo" | redis-cli | jq
```

This will return the following payload to your console:

```json
{
  "name": "awesome_zoo",
  "animals": [
    {
      "name": "wolf",
      "attribute": {
        "family": "Canidae",
        "genus": "Canis",
        "is_mammal": true
      }
    },
    {
      "name": "snake",
      "attribute": {
        "family": "Viperidae",
        "genus": "Boas",
        "is_mammal": false
      }
    }
  ]
}
```

Although this workflow is functional in runtime, there's a big gotcha here! It's really
difficult to picture the shape of the `payload` from the output of the `get_payload`
function; as it dynamically builds the dictionary. First, it declares a dictionary with
two fields—`name` and `animals`. Here, `name` is a string value that denotes the name of
the zoo. The other field `animals` is a list containing the names and attributes of the
animals in the zoo. Later on, the for-loop fills up the dictionary with nested data
structures. This charade of operations makes it difficult to reify the final shape of the
resulting `payload` in your mind.

In this case, you'll have to inspect the content of the Redis cache to fully understand the
shape of the data. Writing code in the above manner is effortless but it makes it really
hard for the next person working on the codebase to understand how the payload looks without
tapping into the data storage. There's a better way to declaratively communicate the shape
of the payload that doesn't involve writing unmaintainably large docstrings. Here's how you
can leverage `TypedDict` and `Annotated` to achieve the goals:

```python
# src.py
from __future__ import annotations

import json

# In < Python 3.8, import 'TypedDict' from 'typing_extensions'.
# In < Python 3.9, import 'Annotated' from 'typing_extensions'.
from typing import Annotated, Any, TypedDict

import redis  # Do a pip install.


class Attribute(TypedDict):
    family: str
    genus: str
    is_mammal: bool


class Animal(TypedDict):
    name: str
    attribute: Attribute


class Zoo(TypedDict):
    name: str
    animals: list[Animal]


def get_payload() -> Zoo:
    """Get the 'zoo' payload containing animal names and attributes."""

    payload: Zoo = {"name": "awesome_zoo", "animals": []}

    names = ("wolf", "snake", "ostrich")
    attributes: tuple[Attribute, ...] = (
        {"family": "Canidae", "genus": "Canis", "is_mammal": True},
        {"family": "Viperidae", "genus": "Boas", "is_mammal": False},
    )
    for name, attr in zip(names, attributes):
        payload["animals"].append({"name": name, "attribute": attr})
    return payload


def save_to_cache(payload: Annotated[Zoo, dict]) -> None:
    # You'll need to spin up a Redis db before instantiating
    # a connection here.
    r = redis.Redis()
    print("Saving to cache...")
    r.set(f"zoo:{payload['name']}", json.dumps(payload))


if __name__ == "__main__":
    payload: Zoo = get_payload()
    save_to_cache(payload)
```

Notice, how I've used `TypedDict` to declare the nested structure of the payload `Zoo`. In
runtime, instances of typed-dict classes behave the same way as normal dicts. Here, `Zoo`
contains two fields—`name` and `animals`. The `animals` field is annotated as `list[Animal]`
where `Animal` is another typed-dict. The `Animal` typed-dict houses another typed-dict
called `Attribute` that defines various properties of the animal.

Taking a look at the typed-dict `Zoo` and following along its nested structure, the final
shape of the payload becomes clearer without us having to look for example payloads. Also,
Mypy can check whether the payload conforms to the shape of the annotated type. I used
`Annotated[Zoo, dict]` in the input parameter of `save_to_cache` function to communicate
with the reader that an instance of the class `Zoo` is a dict that conforms to the contract
laid out in the type itself. The type `Annotated` can be used to add any arbitrary metadata
to a particular type.

In runtime, this snippet will exhibit the same behavior as the previous one. Mypy also
approves this.

## Handling missing key-value pairs

By default, the type checker will structurally validate the shape of the dict annotated with
a `TypedDict` class and all the key-value pairs expected by the annotation must be present
in the dict. It's possible to lax this behavior by specifying *totality*. This can be
helpful to deal with missing fields without letting go of type safety. Consider this:

```python
from __future__ import annotations

from typing import TypedDict


class Attribute(TypedDict):
    family: str
    genus: str
    is_mammal: bool


animal_attribute: Attribute = {
    "family": "Hominidae",
    "genus": "Homo",
}  # Mypy will complain about the missing 'is_mammal' key.
```

Mypy will complain about the missing key:

```txt
src.py:12: error: Missing key "is_mammal" for TypedDict "Attribute"
    animal_attribute: Attribute = {
                                  ^
Found 1 error in 1 file (checked 1 source file)
```

You can relax this behavior like this:

```python
...


class Attribute(TypedDict, total=False):
    family: str
    genus: str
    is_mammal: bool


...
```

Now Mypy will no longer complain about the missing field in the annotated dict. However,
this will still disallow arbitrary keys that isn't defined in the `TypedDict`. For
example:

```python
...

# Mypy will complain as the key 'species' doesn't exist in the TypedDict.
animal_attribute["species"] = "Sapiens"

...
```

```txt
src.py:17: error: TypedDict "Attribute" has no key "species"
    animal_attribute["species"] = "Sapiens"
                    ^
Found 1 error in 1 file (checked 3 source files)
make: *** [Makefile:134: mypy] Error 1

```

Sweet type safety without being too strict about missing fields!

[^1]: [Docker](https://www.docker.com/)
[^2]: [PEP 589 – TypedDict: Type hints for dictionaries with a fixed set of keys](https://peps.python.org/pep-0589/) [^2]
