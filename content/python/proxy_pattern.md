---
title: Implementing proxy pattern in Python
date: 2020-06-16
tags:
    - Python
---

In Python, there's a saying that *[design patterns] are anti-patterns*. Also, in the realm
of dynamic languages, design patterns have the notoriety of injecting additional abstraction
layers to the core logic and making the flow gratuitously obscure. Python's dynamic nature
and the treatment of functions as [first-class] objects often make Java-ish design patterns
redundant.

Instead of littering your code with seemingly over-engineered patterns, you can almost
always take the advantage of Python's first-class objects, duck-typing, monkey-patching etc
to accomplish the task at hand. However, recently there is one design pattern that I find
myself using over and over again to write more maintainable code and that is the Proxy
pattern. So I thought I'd document it here for future reference.

## The proxy pattern

Before diving into the academic definition, let's try to understand the Proxy pattern from
an example.

Have you ever used an access card to go through a door? There are multiple options to open
that door i.e. it can be opened either using access card or by pressing a button that
bypasses the security. The door's main functionality is to open but there is a proxy added
on top of it to add some functionality. Let me better explain it using the code example
below:

```python
# src.py


class Door:
    def open_method(self) -> None:
        pass


class SecuredDoor:
    def __init__(self) -> None:
        self._klass = Door()

    def open_method(self) -> None:
        print(f"Adding security measure to the method of {self._klass}")


secured_door = SecuredDoor()
secured_door.open_method()
```

```
>>> Adding security measure to the method of <__main__.Door object at 0x7f9dab3b6670>
```

The above code snippet concretizes the example given before. Here, the `Door` class has a
single method called `open_method` which denotes the action of *opening* on the `Door`
object. This method gets extended in the `SecuredDoor` class and in this case, I've just
added a print statement to the method of the latter class.

Notice how the class `Door` was called from `SecuredDoor` via [composition]. In the case of
proxy pattern, you can substitute primary object with the proxy object without any
additional changes in the code. This conforms to the [Liskov Substitution Principle]. It
states:

> Objects of a superclass shall be replaceable with objects of its subclasses without
> breaking the application. That requires the objects of your subclasses to behave in the
> same way as the objects of your superclass.

The `Door` object can be replaced by the `SecuredDoor` and the `SecuredDoor` class does not
introduce any new methods, it only extends the functionality of the `open_method` of the
`Door` class.

In plain words:

> Using the proxy pattern, a class represents the functionality of another class.

Wikipedia says:

> A proxy, in its most general form, is a class functioning as an interface to something
> else. A proxy is a wrapper or agent object that is being called by the client to access
> the real serving object behind the scenes. Use of the proxy can simply be forwarding to
> the real object, or can provide additional logic. In the proxy extra functionality can be
> provided, for example caching when operations on the real object are resource intensive,
> or checking preconditions before operations on the real object are invoked.

Pedagogically, the proxy pattern belongs to a family of patterns called the
[structural pattern].

## Why use it?

### Loose coupling

Proxy pattern let's you easily decouple your core logic from the added functionalities that
might be needed on top of that. The modular nature of the code makes maintaining and
extending the functionalities of your primary logic a lot quicker and easier.

Suppose, you're defining a `division` function that takes takes two integer as arguments and
returns the result of the division between them. It also handles edge cases like
`ZeroDivisionError` or `TypeError` and logs them properly.

```python
# src.py
from __future__ import annotations

import logging
from typing import Union

logging.basicConfig(level=logging.INFO)

Number = Union[int, float]


def division(a: Number, b: Number) -> float:
    try:
        result = a / b
        return result

    except ZeroDivisionError:
        logging.error(f"Argument b cannot be {b}")

    except TypeError:
        logging.error("Arguments must be integers/floats")


print(division(1.9, 2))
```

```
>>> 0.95
```

You can see this function is already doing three things at once which violates the
[Single Responsibility Principle]. SRP says that a function or class should have only one
reason to change. In this case, a change in any of the three responsibilities can force the
function to change. Also this means, changing or extending the function can be difficult to
keep track of.

Instead, you can write two classes. The primary class `Division` will only implement the
core logic while another class `ProxyDivision` will extend the functionality of `Division`
by adding exception handlers and loggers.

```python
# src.py
from __future__ import annotations

import logging
from typing import Union

logging.basicConfig(level=logging.INFO)

Number = Union[int, float]


class Division:
    def div(self, a: Number, b: Number) -> Number:
        return a / b


class ProxyDivision:
    def __init__(self) -> None:
        self._klass = Division()

    def div(self, a: Number, b: Number) -> Number:
        try:
            result = self._klass.div(a, b)
            return result

        except ZeroDivisionError:
            logging.error(f"Argument b cannot be {b}")

        except TypeError:
            logging.error("Arguments must be integers/floats")


klass = ProxyDivision()
print(klass.div(2, 0))
```

```
>>> ERROR:root:Argument b cannot be 0
    None
```

In the example above, since both `Division` and `ProxyDivision` class implement the same
interface, you can swap out the `Division` class with `ProxyDivision` and vice versa. The
second class neither inherits directly from the first  class nor it adds any new method to
it. This means you can easily write another class to extend the functionalities of
`Division` or `DivisionProxy` class without touching their internal logics directly.

### Enhanced testability

Another great advantage of using the proxy pattern is enhanced testability. Since your core
logic is loosely coupled with the extended functionalities, you can test them out
separately. This makes the test more succinct and modular. It's easy to demonstrate the
benefits with our previously mentioned `Division` and `ProxyDivision` classes. Here, the
logic of the primary class is easy to follow and since this class only holds the core logic,
it's crucial to write unit test for this before testing the added functionalities.

Testing out the `Division` class is much cleaner than testing the previously defined
`division` function that tries to do multiple things at once. Once you're done testing the
primary class, you can proceed with the additional functionalities. Usually, this decoupling
of core logic from the cruft and the encapsulation of additional functionalities result in
more reliable and rigorous unit tests.

## Proxy pattern with interface

In the real world, your class won't look like the simple `Division` class having only a
single method. Usually your primary class will have multiple methods and they will carry out
multiple sophisticated tasks. By now, you probably have grasped the fact that the proxy
classes need to implement all of the methods of the primary class. While writing a proxy
class for a complicated primary class, the author of that class might forget to implement
all the methods of the primary class.This will lead to a violation of the proxy pattern.
Also, it can be hard to follow all the methods of the primary class if the class is large
and complicated.

Here, the solution is an interface that can signal the author of the proxy class about all
the methods that need to be implemented. An *interface* is nothing but an abstract class
that dictates all the methods a concrete class needs to implement. However, interfaces can't
be initialized independently. You'll have to make a subclass of the interface and implement
all the methods defined there. Your subclass will raise error if it fails to implement any
of the methods of the interface. Let's look at a minimal example of how you can write an
interface using Python's `abc.ABC` and `abc.abstractmethod` and achieve proxy pattern with
that.

```python
# src.py
from abc import ABC, abstractmethod


class Interface(ABC):
    """Interfaces of Interface, Concrete & Proxy should
    be the same, because the client should be able to use
    Concrete or Proxy without any change in their internals.
    """

    @abstractmethod
    def job_a(self, user: str) -> None:
        pass

    @abstractmethod
    def job_b(self, user: str) -> None:
        pass


class Concrete(Interface):
    """This is the main job doer. External services like
    payment gateways can be a good example.
    """

    def job_a(self, user: str) -> None:
        print(f"I am doing the job_a for {user}")

    def job_b(self, user: str) -> None:
        print(f"I am doing the job_b for {user}")


class Proxy(Interface):
    def __init__(self) -> None:
        self._concrete = Concrete()

    def job_a(self, user: str) -> None:
        print(f"I'm extending job_a for user {user}")

    def job_b(self, user: str) -> None:
        print(f"I'm extending job_b for user {user}")


if __name__ == "__main__":
    klass = Proxy()
    print(klass.job_a("red"))
    print(klass.job_b("nafi"))
```

```
>>> I'm extending job_a for user red
    None
    I'm extending job_b for user nafi
    None
```

It's evident from the above workflow that you'll need to define an `Interface` class first.
Python provides abstract base classes as `ABC` in the `abc` module. Abstract class
`Interface` inherits from `ABC` and defines all the methods that the concrete class will
have to implement later. `Concrete` class inherits from the interface and implements all the
methods defined in it. Notice how each method in the `Interface` class is decorated with the `@abstractmethod` decorator. If your knowledge on decorator is fuzzy, then checkout [this]
post on Python decorators. The `@abstractmethod` decorator turns a normal method into an
abstract method which means that the method is nothing but a blueprint of the required
methods that the concrete subclass will have to implement later. You can't directly
instantiate `Interface` or use any of the abstract methods without making subclasses of the
interface and implementing the methods.

The second class `Concrete` is the actual class that inherits from the abstract base class
(interface) `Interface` and implements all the methods mentioned as abstract methods. This
is a real class that you can instantiate and the methods can be used directly. However, if
you forget to implement any of the abstract methods defined in the `Interface` then you'll
invoke `TypeError`.

The third class `Proxy` extends the functionalities of the base concrete class `Concrete`.
It calls the `Concrete` class using the composition pattern and implements all the methods.
However, in this case, I used the results from the concrete methods and extended their
functionalities without code duplication.

## Another practical example

Let's play around with one last real-world example to concretize the concept. Suppose, you
want to collect data from an external API endpoint. To do so, you hit the endpoint with
`GET` requests from your HTTP client and collect the responses in `json` format. Then say,
you also want to inspect the response `header` and the `arguments` that were passed while
making the request.

Now, in the real world, public APIs will often impose rate limits and when you go over the
limit with multiple get requests, your client will likely throw an http connection-timeout
error. Say, you want to handle this exceptions outside of the core logic that will send the
HTTP `GET` requests.

Again, let's say you also want to cache the responses if the client has seen the arguments
in the requests before. This means, when you send requests with the same arguments multiple
times, instead of hitting the APIs with redundant requests, the client will show you the
responses from the cache. Caching improves API response time dramatically.

For this demonstration, I'll be using Postman's publicly available `GET` API.

```txt
https://postman-echo.com/get?foo1=bar_1&foo2=bar_2
```

This API is perfect for the demonstration since it has a rate limiter that kicks in
arbitrarily and make the client throw `ConnectTimeOut` and `ReadTimeOutError`. See how this
workflow is going to look like:

* Define an interface called `IFetchUrl` that will implement three abstract methods. The
first method `get_data` will fetch data from the URL and serialize them into `json` format.
The second method `get_headers` will probe the data and return the header as a dictionary.
The third method `get_args` will also probe the data like the second method but this time it
will return the query arguments as a dictionary. However, in the interface, you won't be
implementing anything inside the methods.

* Make a concrete class named `FetchUrl` that will derive from interface `IFetchUrl`. This
time you'll implement all three methods defined in the abstract class. However, you
shouldn't handle any edge cases here. The method should contain pure logic flow without any
extra fluff.

* Make a proxy class called `ExcFetchUrl`. It will also inherit from the interface but this
time you'll add your exception handling logics here. This class also adds logging
functionality to all the methods. Here you call the concrete class `FetchUrl` in a
composition format and avoid code repetition by using the methods that's been already
implemented in the concrete class. Like the `FetchUrl` class, here too, you've to implement
all the methods found in the abstract class.

* The fourth and the final class will extend the `ExcFetchUrl` and add caching functionality
to the `get_data` method. It will follow the same pattern as the `ExcFetchUrl` class.

Since, by now, you're already familiar with the workflow of the proxy pattern, let's dump
the entire 110 line solution all at once.

```python
from __future__ import annotations

import functools
import logging
import sys
from abc import ABC, abstractmethod
from datetime import datetime
from pprint import pprint

import httpx
from httpx import ConnectTimeout, ReadTimeout
from typing import Any


logging.basicConfig(level=logging.INFO)

D = dict[str, Any]


class IFetchUrl(ABC):
    """Abstract base class. You can't instantiate this independently."""

    @abstractmethod
    def get_data(self, url: str) -> D:
        pass

    @abstractmethod
    def get_headers(self, data: D) -> D:
        pass

    @abstractmethod
    def get_args(self, data: D) -> D:
        pass


class FetchUrl(IFetchUrl):
    """Concrete class that doesn't handle exceptions and loggings."""

    def get_data(self, url: str) -> D:
        with httpx.Client() as client:
            response = client.get(url)
            data = response.json()
            return data

    def get_headers(self, data: D) -> D:
        return data["headers"]

    def get_args(self, data: D) -> D:
        return data["args"]


class ExcFetchUrl(IFetchUrl):
    """This class can be swapped out with the FetchUrl class.
    It provides additional exception handling and logging."""

    def __init__(self) -> None:
        self._fetch_url = FetchUrl()

    def get_data(self, url: str) -> D:
        try:
            data = self._fetch_url.get_data(url)
            return data

        except ConnectTimeout:
            logging.error("Connection time out. Try again later.")
            sys.exit(1)

        except ReadTimeout:
            logging.error("Read timed out. Try again later.")
            sys.exit(1)

    def get_headers(self, data: D) -> D:
        headers = self._fetch_url.get_headers(data)
        logging.info(f"Getting the headers at {datetime.now()}")
        return headers

    def get_args(self, data: D) -> D:
        args = self._fetch_url.get_args(data)
        logging.info(f"Getting the args at {datetime.now()}")
        return args


class CacheFetchUrl(IFetchUrl):
    def __init__(self) -> None:
        self._fetch_url = ExcFetchUrl()
        self.get_data = functools.lru_cache()(self.get_data)  # type: ignore

    def get_data(self, url: str) -> D:
        data = self._fetch_url.get_data(url)
        return data

    def get_headers(self, data: D) -> D:
        headers = self._fetch_url.get_headers(data)
        return headers

    def get_args(self, data: D) -> D:
        args = self._fetch_url.get_args(data)
        return args


if __name__ == "__main__":
    # url = "https://postman-echo.com/get?foo1=bar_1&foo2=bar_2"

    fetch = CacheFetchUrl()
    for arg1, arg2 in zip([1, 2, 3, 1, 2, 3], [1, 2, 3, 1, 2, 3]):
        url = f"https://postman-echo.com/get?foo1=bar_{arg1}&foo2=bar_{arg2}"
        print(f"\n {'-'*75}\n")
        data = fetch.get_data(url)
        print(f"Cache Info: {fetch.get_data.cache_info()}")  # type: ignore
        pprint(fetch.get_headers(data))
        pprint(fetch.get_args(data))
```

```
---------------------------------------------------------------------------

INFO:root:Getting the headers at 2022-01-31 16:54:36.214562
INFO:root:Getting the args at 2022-01-31 16:54:36.220221
Cache Info: CacheInfo(hits=0, misses=1, maxsize=32, currsize=1)
{'accept': '*/*',
    'accept-encoding': 'gzip, deflate',
    'content-length': '0',
    'host': 'postman-echo.com',
    'user-agent': 'python-httpx/0.13.1',
    'x-amzn-trace-id': 'Root=1-5ee8a4eb-4341ae58365e4090660dfaa4',
    'x-b3-parentspanid': '044bd10726921994',
    'x-b3-sampled': '0',
    'x-b3-spanid': '503e6ceaa2a4f493',
    'x-b3-traceid': '77d5b03fe98fcc1a044bd10726921994',
    'x-envoy-external-address': '10.100.91.201',
    'x-forwarded-client-cert': 'By=spiffe://cluster.local/ns/pm-echo-istio/sa/default;
    Hash=2ed845a68a0968c80e6e0d0f49dec5ce15ee3c1f87408e56c938306f2129528b;Subject="";
    URI=spiffe://cluster.local/ns/istio-system/sa/istio-ingressgateway-service-account',
    'x-forwarded-port': '443',
    'x-forwarded-proto': 'http',
    'x-request-id': '295d0b6c-7aa0-4481-aa4d-f47f5eac7d57'}
{'foo1': 'bar_1', 'foo2': 'bar_1'}

....
```

In the `get_data` method of the `FetchUrl` class, I've used the awesome [httpx] client to
fetch the data from the URL. Pay attention to the fact that I've practically ignored all the
additional logics of error handling and logging here. The exception handling and logging
logic were added via `ExcFetchUrl` proxy class. Another class `CacheFetchUrl` further
extends the proxy class `ExcFetchUrl` by adding cache functionality to the `get_data`
method.

In the main section, you can use any of the `FetchUrl`, `ExcFetchUrl` or `CacheFetchUrl`
without any additional changes to the logic of these classes. The `FetchUrl` is the barebone
class that will fail in case of the occurrence of any exceptions. The latter classes appends
additional functionalities while maintaining the same interface.

The output basically prints out the results returned by the `get_headers` and `get_args`
methods. Also notice, how I picked the endpoint arguments to simulate caching. The
`Cache Info:` on the third line of the output shows when data is served from the cache.
Here, `hits=0` means data is served directly from the external API. However, if you inspect
the later outputs, you'll see when the query arguments get repeated ([1, 2, 3, 1, 2, 3]),
`Cache Info:` will show higher hit counts. This means that the data is being served from the
cache.

## Should you use it?

Well, yes obviously. But not always. You see, you need a little bit of planning before
orchestrating declarative solution with the proxy pattern. It's not viable to write code in
this manner in a throwaway script that you don't have to maintain in the long run. Also,
this OOP-cursed additional layers of abstraction can make your code subjectively unreadable.
So use the pattern wisely. On the flip side, proxy pattern can come in handy when you need
to extend the functionality of some class arbitrarily as it can work a gateway to the El
Dorado of loose coupling.

## References

* [Python patterns]
* [Design patterns for humans]
* [Design patterns - refactoring guru]
* [Design patterns & idioms]

[design patterns]: https://en.wikipedia.org/wiki/Design_Patterns
[composition]: https://realpython.com/inheritance-composition-python/#composition-in-python
[liskov substitution principle]: https://en.wikipedia.org/wiki/Liskov_substitution_principle
[structural pattern]: https://en.wikipedia.org/wiki/Structural_pattern
[single responsibility principle]: https://stackify.com/solid-design-principles/
[this]: /python/decorators
[httpx]: https://github.com/encode/httpx
[python patterns]: https://github.com/faif/python-patterns
[design patterns for humans]: https://github.com/kamranahmedse/design-patterns-for-humans
[design patterns - refactoring guru]: https://refactoring.guru/design-patterns/proxy/python/example
[design patterns & idioms]: https://python-3-patterns-idioms-test.readthedocs.io/en/latest/Fronting.html
