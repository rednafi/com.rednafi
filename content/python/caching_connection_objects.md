---
title: Caching connection objects in Python
date: 2022-03-16
tags:
    - Python
    - TIL
---

To avoid instantiating multiple DB connections in Python apps, a common approach is to
initialize the connection objects in a module once and then import them everywhere. So,
you'd do this:

```python
# src.py
import boto3  # Pip install boto3
import redis  # Pip install redis

dynamo_client = boto3.client("dynamodb")
redis_client = redis.Redis()
```

However, this adds import time side effects to your module and can turn out to be expensive.
In search of a better solution, my first instinct was to go for `functools.lru_cache(None)`
to immortalize the connection objects in memory. It works like this:

```python
from __future__ import annotations

# In Py < 3.9, use 'functools.lru_cache(None)'.
from functools import cache

import boto3
import redis


@cache
def get_dynamo_client() -> boto3.session.Session.client:
    return boto3.client("dynamodb")


@cache
def get_redis_client() -> redis.Redis:
    return redis.Redis()
```

This way, the connection objects returned by the functions are cached and any subsequent
calls to the functions will provide the same connection objects from the cache without
reinitializing them.

One problem with the above approach is—how complex the implementation of the `cache`
decorator is. Underneath, the `functools.cache` decorator is an alias for
`functools.lru_cache(None)` and it employs a **Least Recently Used** cache eviction policy.
While this policy is quite useful when you need it but to cache simple connection objects,
arguably, the complexity and the overhead of the `cache` decorator offset its benefits.
There's a simpler way to do it and James Powell on Twitter pointed[^1] me to it. This works
as follows:

```python
# src.py
from __future__ import annotations

import boto3
import redis

_cache = {}


def get_dynamo_client(
    service_name: str = "dynamodb",
) -> boto3.session.Session.client:
    """Immortalize the Dynamo client object so that this function
    always returns the same connection object ."""

    if service_name not in _cache:
        _cache[service_name] = boto3.client(service_name)

    return _cache[service_name]


def get_redis_client(service_name: str = "redis") -> redis.Redis:
    """Immortalize Redis connection object."""

    if service_name not in _cache:
        _cache[service_name] = redis.Redis()

    return _cache[service_name]
```

Is this singleton pattern? Probably so.

[^1]: [Caching connections in Python — Twitter](https://twitter.com/rednafi/status/1503465791987273729?s=20&t=GlzWHBF_y0ZR-uKHVSP40Q)
