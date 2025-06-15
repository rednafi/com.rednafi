---
title: Escaping the template pattern hellscape in Python
date: 2023-07-01
tags:
    - Python
---

Over the years, I've used the template pattern[^1] across multiple OO languages with varying
degrees of success. It was one of the first patterns I learned in the primordial hours of my
software engineering career, and for some reason, it just feels like the natural way to
tackle many real-world code-sharing problems. Yet, even before I jumped on board with the
composition over inheritance[^2] camp, I couldn't help but notice how using this particular
inheritance technique spawns all sorts of design and maintenance headaches as the codebase
starts to grow.

## An epiphany

This isn't an attempt to explain why you should prefer composition over inheritance
(although you should), as it's a vast topic and much has been said regarding this. Also,
only after a few years of reading concomitant literatures and making enough mistakes in
real-life codebases, it dawned on me that opting for inheritance as the default option leads
to a fragile design. So I won't even attempt to tackle that in a single post and will refer
to a few fantastic prior arts that proselytized me to the composition cult.

The goal of this article is not to focus on the wider spectrum of how to transform
subclass-based APIs to use composition but rather to zoom in specifically on the template
pattern and propose an alternative way to solve a problem where this pattern most naturally
manifests. In the first portion, the post will explain what the template pattern is and how
it gradually leads to an intractable mess as the code grows. In the latter segments, I'll
demonstrate how I've designed a real-world service by adopting the obvious and natural path
of inheritance-driven architecture. Then, I'll explain how the service can be refactored to
escape the quagmire that I've now started to refer to as the **template pattern hellscape**.

Only a few moons ago, while watching Hynek Schlawack's Python 2023 talk aptly titled
"Subclassing, Composition, Python, and You"[^3] and reading his fantastic blog post
"Subclassing in Python Redux"[^4], the concept of adopting composition to gradually phase
out subclass-oriented design from my code finally clicked for me. However, it's not always
obvious to me how to locate inheritance metastasis and exactly where to intervene to make
the design better. This post is my attempt to distill some of my learning from those
resources and focus on improving only a small part of the gamut.

## The infectious template pattern

You're consciously or subconsciously implementing the template pattern when your API design
follows these steps:

- You have an Abstract Base Class (ABC) with abstract methods.
- The ABC also includes one or more concrete methods.
- The concrete methods in the ABC depend on the concrete implementation of the abstract
  methods.
- API users are expected to inherit from the ABC and provide concrete implementations for
  the abstract methods.
- Users then utilize the concrete methods defined in the ABC class.

This pattern enables the sharing of concrete method implementations with subclasses.
However, the concrete methods of the baseclass are only valid when the user inherits from
the base and implements the abstract methods. Attempting to instantiate the baseclass
without implementing the abstract methods will result in a `TypeError`. Only the subclass
can be initialized once all the abstract methods have been implemented.

Observe this example:

```py
from abc import ABC, abstractmethod


class Base(ABC):
    def concrete_method(self) -> None:
        # This depends on abstract_method. The user is expected to create
        # a subclass from Base and implement abstract_method.
        return self.abstract_method()

    @abstractmethod
    def abstract_method(self) -> None:
        raise NotImplementedError


class Sub(Base):
    def abstract_method(self) -> None:
        """Providing a concrete implementation for the 'abstract_method' from
        the Base class."""
        print(
            "I'm a concrete implementation of the 'abstract_method' of Base."
        )
```

This is how you use it:

```py
sub = Sub()

# Notice how we're only using the 'concrete_method' defined in the Base class
sub.concrete_method()
```

Here, the abstract `Base` class is defined by inheriting from the `abc.ABC` class. Inside
`Base`, there's a `concrete_method` that relies on an `abstract_method`. The
`concrete_method` is defined to call `abstract_method`, expecting that subclasses will
provide their own implementation of `abstract_method`. If a subclass of `Base` fails to
implement `abstract_method`, calling `concrete_method` on an instance of that subclass will
raise a `NotImplementedError`.

The snippet also provides an example subclass called `Sub`, which inherits from `Base`.
`Sub` overrides the `abstract_method` and provides its own implementation. In this case, it
just prints a statement. By subclassing `Base` and implementing `abstract_method`, `Sub`
becomes a concrete class that can be instantiated. The purpose of this design pattern is to
define a common interface through the `Base` class, with the expectation that subclasses
will implement specific behavior by overriding the abstract methods, while still providing a
way to call those methods through the concrete methods defined in the baseclass. This
seemingly innocuous and often convenient bi-directional relationship between the base and
sub class tends to become infectious and introduces complexity into all the subclasses that
inherit from the base.

## The dark side of the moon

Template pattern seems like the obvious way of sharing code and it almost always is one of
the first things that people learn while familiarizing themselves with how OO works in
Python. Plus, it's used extensively in the standard library. For example, in the
`collections.abc` module, there are a few ABCs that you can subclass to build your own
containers. I wrote about this[^5] a few years back. Here's how you can subclass
`collections.abc.Sequence` to implement a tuple-like immutable datastructure:

```py
from typing import Any
from collections.abc import Sequence


class CustomSequence(Sequence):
    def __init__(self, *args: Any) -> None:
        self._data = list(args)

    def __getitem__(self, index: int) -> Any:
        return self._data[index]

    def __len__(self) -> int:
        return len(self._data)
```

You'd use the class as such:

```py
seq = CustomSequence(1, 2, 3, 4)
assert seq[0] == 1
assert len(seq) == 4
```

We're inheriting from the `Sequence` ABC and implementing the required abstract methods.
Here's the first issue: how do we even know which methods to implement and which methods we
get for free? You can consult the documentation[^6] and learn that `__getitem__` and
`__len__` are the abstract methods that subclasses are expected to implement. In return, the
base `Sequence` class gives you `__contains__`, `__iter__`, `__reversed__`, `index`, and
`count` as mixin methods. You can also print out the abstract methods by accessing the
`Sequence.__abstractmethod__` attribute. Sure, you're getting a lot of concrete methods for
free, but suddenly you're dependent on some out-of-band information to learn about the
behavior of your specialized `CustomSequence` class.

The following three sections will briefly explore the issues that deceptively creep up on
your codebase when you opt for the template pattern.

### Elusive public API

You've already seen the manifestation of this issue in the `CustomSequence` example. The
subclass-oriented code-sharing pattern like this makes it difficult to discover the public
API of your specialized class because many of its functionalities come from the concrete
mixin methods provided by the base `Sequence` class. Now, this isn't too terrible for a tool
in the standard library since they're usually quite well-documented, and you can always
resort to inspecting the subclass instance to learn about the abstract and concrete methods.

Not all subclass-driven design is bad, and the standard library makes judicious use of the
template pattern. However, in an application codebase that you might be writing, this
elusive nature of the public API can start becoming recalcitrant. Your code may not be as
well-documented as the standard library, or instantiating the subclass may be expensive,
making introspection difficult. You're basically trading off readability for writing
ergonomics. There's nothing wrong in doing that as long as you're aware of the tradeoffs.
All I'm trying to say is that it's a non-ideal default.

### Namespace pollution

If you introspect the previously defined subclass with `dir(CustomSequence)`, you'll get the
following result. I've removed the common attributes that every class inherits from `object`
for brevity and annotated the abstract and mixin method names for clarity.

```py
[
    "__abstractmethods__",  # Allows you to list out the abstract methods
    "__class_getitem__",  # Used for generic typing
    "__contains__",  # Provided by the base
    "__getitem__",  # You implement
    "__iter__",  # Provided by the base
    "__len__",  # You implement
    "__reversed__",  # Provided by the base
    "count",  # Provided by the base
    "index",  # Provided by the base
]
```

From the above list, it's evident that all the methods from the baseclass and the subclass
live in the same namespace. The moment you're inheriting from some baseclass, you have no
control over what that class is bringing over to your subclass's namespace and effectively
polluting it. It's like a more sneaky version of `from foo import *`.

This flat namespacing makes it hard to understand which method is coming from where. In the
above case, without the annotations, you'd have a hard time discerning between the methods
that you implemented and the alien methods from the baseclass. This isn't a cardinal sin in
the Python realm if that's what you want, but it's certainly a suboptimal default.

### SRP violation & goose chase program flow

The biggest complaint I have against the template pattern is how it encourages the baseclass
to do too many things at once. I can endure poor discoverability of public APIs and
namespace pollution to some extent, but when a class tries to do too many things
simultaneously, it eventually exhibits the tendency to give birth to God Objects[^7];
breaching the SRP (Single Responsibility Principle).

Intentionally violating the SRP rarely fosters good results, and in this case, the baseclass
defines both concrete and abstract methods. Not only that, the base expects the subclasses
to implement those abstract methods so that it can use them in its concrete method
implementation. Just reading back this sentence is giving me a headache. If you design your
APIs in this manner, you'll have to carefully read through both the sub and the base class
implementations to understand how this intricate bi-directional thread is woven into your
program flow. This seems easy enough in a simple example where you can see both the base and
the sub class in a single snippet, but it quickly gets out of hand when large base and sub
classes are scattered across multiple modules. You'll need to perform the mental gymnastics
of tracking this back-and-forth logic, aka the abominable **goose chase program flow**.

## The disease and the cure

Let's examine a specific design problem and observe how it can be modeled using the template
pattern. Then, we'll explore an alternative solution that replaces the inheritance-driven
design with composition.

### Designing with template pattern

The following code snippet mimics a real-world webhook[^8] dispatcher that takes a message
and posts it to a callback URL via HTTP `POST` request. First, we'll commit the cardinal sin
of modeling the domain with the template pattern and then we'll try to find a way out of the
quandary. Here it goes:

```py
from dataclasses import dataclass, field, asdict
from uuid import uuid4
from abc import ABC, abstractmethod


@dataclass(frozen=True)
class Message:
    ref: str = field(default_factory=lambda: str(uuid4()))
    body: str = ""


class BaseWebhook(ABC):
    def send(self) -> None:
        url = self.get_url()
        data = self.get_message()
        print(f"sending {data} to {url}")

    @abstractmethod
    def get_message(self) -> dict[str, str]:
        raise NotImplementedError

    @abstractmethod
    def get_url(self) -> str:
        raise NotImplementedError


class Webhook(BaseWebhook):
    def __init__(self, message: Message) -> None:
        self.message = message

    def get_message(self) -> dict[str, str]:
        # Assume that we're doing other side effects and adding more data in
        # runtime
        return asdict(self.message)

    def get_url(self) -> str:
        return "https://webhook.site/foo"
```

Here's how you'll orchestrate the classes:

```py
message = Message(body="Hello World")
webhook = Webhook(message)

# This just prints:
# sending
# {
#   'ref': '4635cfe0-825e-4f40-9c7b-04275b1c809e',
#   'body': 'Hello World'
# } to https://webhook.site/foo
webhook.send()
```

We start by defining an immutable `Message` container to store our webhook message. Next, we
write an abstract `BaseWebhook` class that inherits from `abc.ABC`. This class serves as a
template for the webhook functionality and declares two abstract methods: `get_message()`
and `get_url()`. Type annotations are used to indicate the return types of these methods.
Any subclasses derived from `BaseWebhook` must implement these abstract methods. The
`send()` method, implemented in the baseclass, uses the concrete implementations of the
abstract methods to perform webhook dispatching. In this case, we simulate the HTTP POST
functionality by printing the message and destination URL.

The `Webhook` class is a subclass of `BaseWebhook` and provides concrete implementations of
the abstract methods. It accepts a single `Message` object as a parameter in its
constructor. The `get_url()` method returns a fixed URL, while the `get_message()` method
converts the `Message` object into a serializable dictionary representation using
`dataclasses.asdict()`.

In this structure, the user of the `Webhook` class only needs to initialize the class and
call the `send()` method on the instance. The `send()` method, however, lives in the
`BaseWebhook` class, not the specialized `Webhook` subclass. It utilizes the concrete
implementations of abstract methods to deliver the `send()` functionality. In the following
section, we'll explore a method to avoid this weird back-and-forth program flow.

### Finding salvation in strategy pattern

There are multiple ways and conflicting opinions on how to get out of the hole we've dug for
ourselves. Some even like to spend more time prattling around the philosophy of how OO is
terrible and how, if it weren't for Java's huge influence on Python, we wouldn't be in this
mess, rather than attempting to solve the actual problem. So instead of trying to cover
every possible solution under the sun, I'll go through the one that has worked for me fairly
well.

We'll refactor the code in the previous section to take advantage of composition and
structural subtyping[^9] support in Python. Long story short, structural subtyping refers to
the ability to ensure type safety based on the structure or shape of an object rather than
its explicit inheritance hierarchy. This allows us to define and enforce contracts based on
the presence of specific attributes or methods, rather than relying on a specific class or
inheritance relationship.

This is achieved through the use of the `typing.Protocol` class introduced in Python 3.8. By
defining a protocol using the `typing.Protocol` class, we can specify the expected
attributes and methods that an object should have to satisfy the protocol. Any object that
matches the structure defined by the protocol can be treated as if it conforms to that
protocol, enabling more flexible and dynamic type-checking in Python. This conformity is
usually checked by a type-checking tool like mypy[^10]. If you want to learn more, check out
Glyph's post titled "I Want a New Duck"[^11]. Here's how I refactored it:

```py
from dataclasses import dataclass, field, asdict
from uuid import uuid4
from typing import Protocol


@dataclass(frozen=True)
class Message:
    ref: str = field(default_factory=lambda: str(uuid4()))
    body: str = ""


class Retriever(Protocol):
    def get_message(self, message: Message) -> dict[str, str]: ...

    def get_url(self) -> str: ...


class Dispatcher(Protocol):
    def dispatch(self, url: str, data: dict[str, str]) -> None: ...


class HookRetriever:
    def get_message(self, message: Message) -> dict[str, str]:
        # Assume that we're doing other side effects and adding more data in
        # runtime
        return asdict(message)

    def get_url(self) -> str:
        return "https://webhook.site/foo"


class HookDispatcher:
    def dispatch(self, url: str, data: dict[str, str]) -> None:
        print(f"Sending {data} to {url}")


@dataclass
class Webhook:
    message: Message
    retriever: Retriever
    dispatcher: Dispatcher

    def send(self) -> None:
        url = self.retriever.get_url()
        data = self.retriever.get_message(self.message)
        return self.dispatcher.dispatch(url, data)
```

The classes can be wired together as follows:

```py
message = Message(body="Hello World")
retriever = HookRetriever()
dispatcher = HookDispatcher()

webhook = Webhook(message, retriever, dispatcher)

# This prints the same thing as before:
# sending
# {
#   'ref': '4635cfe0-825e-4f40-9c7b-04275b1c809e',
#   'body': 'Hello World'
# } to https://webhook.site/foo
webhook.send()
```

We've agreed that the `BaseWebhook` class tries to do too many things at once. The first
step to disentangling a class is to identify its responsibilities and create multiple
component classes where each new class will only have one responsibility. Here, the base
class retrieves the necessary data and dispatches the webhook using that data at the same
time. The `Retriever` and `Dispatcher` protocol classes will formalize the shape and
structure of those component classes. These protocols work like the ABCs, but you don't need
to inherit from them to ensure interface conformity; the type checker will do it for you.

The `Retriever` class has two methods: `get_message` and `get_url`, which fetch message and
URL data respectively. Similarly, the `Dispatcher` protocol has only a `dispatch` method
that sends the webhook. In either case, the protocol methods don't implement anything; they
work just like the abstract methods of the ABCs, and the protocol classes themselves can't
be instantiated. Then the `HookRetriever` and `HookDispatcher` components implicitly
implement the protocol classes. Notice that neither of the components inherits from the
protocol classes. The type checker will ensure that they conform to the defined protocols.

The question is, how does the type checker know which class is supposed to conform to which
protocol? The answer lies in the final `Webhook` class. We define a final dataclass that
takes instances of the `Message`, `Retriever`, and `Dispatcher` classes in the constructor.
Notice that while adding type hints to the `retriever` and `dispatcher` parameters of the
dataclass constructor, we're using the protocol classes instead of the concrete ones. This
is how the type checker knows that whatever instance is passed to the `retriever` and
`dispatcher` parameters must conform to the `Retriever` and `Dispatcher` protocols,
respectively. Note that we've completely eliminated subclassing from our public API.
Injecting dependencies in this manner is also known as the strategy pattern[^12].

The `Webhook` class now has a hierarchical namespace instead of a flat one, unlike our
inheritance-based friend. You'll have to be explicit about where a method is coming from
when calling it. So if you need to access the fetched URL, you'll need to explicitly call
`self.retriever.get_url()`. The `self` namespace has only one user-defined public method,
`.send()`, which can be called to dispatch the webhook from a `Webhook` instance. This also
means you no longer have to deal with goose chase program flow since all the dependencies
flow towards the final `Webhook` class.

On the flip side, you'll need to do more work while initializing the `Webhook` class. The
`Message`, `HookRetriever`, and `HookDispatcher` classes need to be instantiated first and
then passed explicitly to the constructor of the `Webhook` class to instantiate it. You're
basically trading writing ergonomics for readability. Instantiating the template subclass
was a lot easier for sure.

## Tradeoffs

Opting in for composition isn't free, as it usually leads to more verbose code
orchestration. If you're passing all the dependencies explicitly, as shown above, wiring the
code together will be more complex. However, in return, you get a more readable and testable
design substrate. So, I'm more than happy to make the tradeoff. Additionally, avoiding
namespace pollution means that one attribute access has now turned into two or more
attribute accesses, which can cause performance issues in tight conditions.

Moreover, you can't just take your inheritance-heavy API and suddenly turn it into a
composable one. It usually requires planning and designing from the ground up, where you
might decide that the ROI isn't good enough to justify the effort of refactoring. Plus, in a
language like Python, you can't always escape inheritance, nor should you try to do so.

> _Yet behold, it need not be the customary stratagem that thou graspest at each moment
> thine heart yearns to commune code amidst classes._

[^1]:
    [Template pattern](https://refactoring.guru/design-patterns/template-method/python/example)

[^2]:
    [Composition over inheritance - Brandon Rhodes](https://python-patterns.guide/gang-of-four/composition-over-inheritance/)

[^3]:
    [Subclassing, composition, Python, and you - Hynek Schlawack](https://www.youtube.com/watch?v=k8MT5liCQ7g)

[^4]:
    [Subclassing in Python redux - Hynek Schlawack](https://hynek.me/articles/python-subclassing-redux/)

[^5]:
    [Interfaces, mixins and building powerful custom data structures in Python](/python/mixins/)

[^6]:
    [Sequence docs](https://docs.python.org/3/library/collections.abc.html#collections.abc.Sequence)

[^7]: [God objects](https://blog.devgenius.io/code-smell-14-god-objects-b84b75b702)

[^8]: [Webhook](https://zapier.com/blog/what-are-webhooks/)

[^9]: [Structural subtyping](https://rednafi.com/python/structural_subtyping/)

[^10]: [Mypy](https://mypy-lang.org/)

[^11]: [I want a new duck - Glyph](https://blog.glyph.im/2020/07/new-duck.html)

[^12]: [Strategy pattern](https://refactoring.guru/design-patterns/strategy/python/example)

[^13]:
    [End of object inheritance - Augie Fackler, Nathaniel Manista](https://www.youtube.com/watch?v=3MNVP9-hglc)
    [^13]
