---
title: Taming conditionals with bitmasks
date: 2023-07-29
tags:
    - Python
    - TIL
---

The 100k context window of Claude 2[^1] has been a huge boon for me since now I can paste a
moderately complex problem to the chat window and ask questions about it. In that spirit,
it recently refactored some pretty gnarly conditional logic for me in such an elegant manner
that it absolutely blew me away. Now, I know how bitmasks[^2] work and am aware of the
existence of `enum.Flag`[^3] in Python. However, it never crossed my mind that flags can be
leveraged to trim conditional branches in such a clever manner that Claude illustrated. But
once I looked at the proposed solution, the whole thing immediately clicked for me.

## The conundrum

Here's a problem that's similar to what I was trying to solve. Let's say we have instances
of a `Client` entity that need to be notified when some special event occurs in our system.
The notification can happen in three ways: email, webhook, and postal mail. These are the
three attributes on the `Client` class that determine which notification method will be
used:

```python
@dataclass
class Client:
    email: str
    url: str
    address: str
```

The business logic requires that the system must abide by the following rules while sending
notifications:

* If only `email` is populated, send an email.
* If only `url` is populated, send a webhook.
* If only `address` is populated, send a postal mail.
* If `email` and `url` are populated, send an email and a webhook.
* If `email` and `address` are populated, only send an email.
* If `url` and `address` are populated, only send a webhook.
* If all three are populated, send both an email and a webhook.
* At least one attribute must be populated, or it's an error.

Notice how the business logic wants to minimize sending notifications via postal mail.
Postal mails are expensive and will only be sent if `address` is the only attribute on the
`Client` instance. In any other cases, emails and webhooks are preferred.

## First shot

The `notify` function takes in a `Client` object and sprouts a few conditional branches to
send notifications while maintaining the business constraints.


```python
def notify(client: Client) -> None:
    """Apply business logic and invoke the desired notification handlers."""

    if client.email and not client.url and not client.address:
        send_email()

    elif client.url and not client.email and not client.address:
        send_webhook()

    elif client.address and not client.email and not client.url:
        send_mail()

    elif client.email and client.url and not client.address:
        send_email()
        send_webhook()

    elif client.email and client.address and not client.url:
        send_email()

    elif client.url and client.address and not client.email:
        send_webhook()

    elif client.email and client.url and client.address:
        send_email()
        send_webhook()

    else:
        raise ValueError("at least one attribute must be populated")
```

Whoa! Lots of if-else branches for such a simple scenario. Since there are 3 attributes in
the complete *set*, we have to make sure we're writing `2^3=8` branches to cover all the
possible *subsets*. For 4, 5, 6 ... attributes, the number of branches will increase as
powers of 2: `2^4=16`, `2^5=32`, `2^6=64` ... and so on. Then our tests will need to be
able to verify each of these branches. We can try to apply De Morgan's law to simplify
some of the negation logic.

> *De Morgan's laws allow us to take the negation of a conditional statement and distribute
> it across the operators, changing ANDs to ORs and vice versa, and flipping the negation of
> each component. This can help simplify complex boolean logic statements.*

So this:

```python
if client.email and not client.url and not client.address:
    ...
```

Can become:

```python
if client.email and not (client.url or client.address):
    ...
```

However, that still doesn't reduce the number of branches. Bitmasks can help us to get out
of this pothole.

## A quick primer on bitwise operations & bitmasking

Bitwise operations allow manipulating numbers at the individual bit level. This is useful
for compactly storing and accessing data, performing fast calculations, and implementing
low-level algorithms. Here's a list of bitwise operations:

* **Bitwise AND (&)**: Takes two numbers and performs the logical AND operation on each pair
of corresponding bits. Returns a number where a bit is 1 only if that bit is 1 in both input
numbers.

* **Bitwise OR (|)**: Takes two numbers and performs the logical OR operation on each pair
of corresponding bits. Returns a number where a bit is 1 if that bit is 1 in either or both
input numbers.

* **Bitwise XOR (^)**: Takes two numbers and performs the logical XOR (exclusive OR)
operation on each pair of corresponding bits. Returns a number where a bit is 1 if that bit
is 1 in exactly one of the input numbers (but not both).

* **Bitwise NOT (~)**: Takes a single number and flips all its bits.

* **Left shift (<<)**: Shifts the bits of a number to the left by a specified number of
positions. Zeros are shifted in on the right. Equivalent to multiplying by `2^n` where `n`
is the number of positions shifted.

* **Right shift (>>)**: Shifts the bits of a number to the right by a specified number of
positions. Zeros are shifted in on the left. Equivalent to integer division by `2^n`.

Here's an example displaying these operators:

```python
a = 60  # 60    = 0011 1100
b = 13  # 13    = 0000 1101
print(a & b)  # 12    = 0000 1100 (0011 1100 & 0000 1101 = 0000 1100)
print(a | b)  # 61    = 0011 1101 (0011 1100 | 0000 1101 = 0011 1101)
print(a ^ b)  # 49    = 0011 0001 (0011 1100 ^ 0000 1101 = 0011 0001)
print(~a)  # -61   = 1100 0011 (~0011 1100 = 1100 0011)
print(a << 2)  # 240   = 1111 0000 (0011 1100 << 2 = 1111 0000)
print(a >> 2)  # 15    = 0000 1111 (0011 1100 >> 2 = 0000 1111)
```

Bitmasks are integers that represent a set of flags using bits as boolean values. Bitmasking
uses bitwise operators to manipulate and access these flags. A common use of bitmasks is to
compactly store multiple boolean values or options in a single integer, where each bit
position has a specific meaning if it is `1`. In the next section, we'll use this capability
to clip the conditional statements in the `notify` function.

For example, here's a bitmask representing text style options:

```python
# Flags
BOLD = 1  # 0000 0001
ITALIC = 2  # 0000 0010
UNDERLINE = 4  # 0000 0100

# Bitmask
STYLE = BOLD | ITALIC  # 0000 0011 - bold and italic
```

We use powers of 2 (1, 2, 4, 8, etc.) for the flag values so that each bit position
corresponds to a single flag, and the flags can be combined using bitwise OR without
overlapping. This allows testing and accessing each flag independently:

```python
has_bold = STYLE & BOLD == BOLD  # True
has_italic = STYLE & ITALIC == ITALIC  # True
has_underline = STYLE & UNDERLINE == UNDERLINE  # False
```

And toggle an option on or off using XOR:

```python
STYLE ^= BOLD  # Toggles BOLD bit on/off
```

You can do a ton of other cool stuff with bitwise operations and bitmasks. However, this
is pretty much all we need to know to curtail the twisted conditional branching
necessitated by the business logic. Check out this incredibly in-depth article[^4] from Real
Python on this topic if you want to dig deeper into bitwise operations.

## Pruning conditional branches with flags

With all the intros and primers out of the way, we can now start working towards making the
`notify` function more tractable and testable. We'll do that in 3 phases:

* First, we're gonna define a flag-type enum called `NotifyStatus` which will house all the
valid states our notification system can be in. Any state that's not explicitly defined as
an enum variant is invalid.

* Second, we'll write a function named `get_notify_status` that'll take in a `Client` object
as input, apply the business logic and return the appropriate `NotifyStatus` enum variant.
This function won't be responsible for dispatching the actual notification handlers; rather,
it'll just map the attribute values of the `Client` instance to a fitting enum variant. We
do this to keep the core business logic devoid of any external dependencies—following Gary
Bernhardt's functional core, imperative shell[^5] ethos.

* Finally, we'll define the `notify` function that'll just accept the enum variant returned
by the previous function and invoke the desired notification handlers.

The `NotifyStatus` enum is defined as follows:

```python
class NotifyStatus(Flag):
    # Valid primary variants (flags)
    EMAIL = 1
    URL = 2
    ADDRESS = 4

    # Valid composite variants (bitmasks)
    EMAIL_URL = EMAIL | URL
    EMAIL_ADDRESS = EMAIL | ADDRESS
    URL_ADDRESS = URL | ADDRESS
    EMAIL_URL_ADDRESS = EMAIL | URL | ADDRESS
```

Here, the `EMAIL`, `URL`, and `ADDRESS` variants correspond to the eponymous attributes on
the `Client` instance. Then we define the composite variants (bitmasks) to compactly
represent the valid states the system can be in. For example, `EMAIL_URL = EMAIL | URL`
means that on the `Client` instance, `email` and `url` attributes are populated but
`address` isn't. Likewise, `EMAIL_URL_ADDRESS` denotes that all the attributes are
populated. The biggest benefit we get from this is that we don't need to write the negation
logic explicitly; the bitmasks encode that information inherently. This representation will
grossly simplify the implementation of the business logic.

Now, let's write the `get_notify_status` function that'll take in an instance of `Client`
and return the appropriate `NotifyStatus` variant based on our business logic:

```python
def get_notify_status(client: Client) -> NotifyStatus:
    status = 0
    if client.email:
        status |= NotifyStatus.EMAIL.value
    if client.url:
        status |= NotifyStatus.URL.value
    if client.address:
        status |= NotifyStatus.ADDRESS.value
    if status == 0:
        raise ValueError("Invalid status")

    return NotifyStatus(status)
```

This is the full implementation of our business logic in its entirety. It checks which of
the notification attributes among `email`, `url`, and `address` are populated on the
`Client` object. For each one that is populated, it picks the corresponding variant from the
`NotifyStatus` enum and sets the variant bit in the status integer using bitwise OR. If all
three attributes are empty, it raises a `ValueError`. The final value of status is then used
to return the correct `NotifyStatus` enum variant.

On the last step, the `notify` function can take the `NotifyStatus` variant returned by the
`get_notify_status` function and dispatch the correct notification handlers like this:

```python
def notify(notify_status: NotifyStatus) -> None:
    # Mapping between enum variants and notification handlers
    actions = {
        NotifyStatus.EMAIL: [send_email],
        NotifyStatus.URL: [send_webhook],
        NotifyStatus.ADDRESS: [send_mail],
        NotifyStatus.EMAIL_URL: [send_email, send_webhook],
        NotifyStatus.EMAIL_ADDRESS: [send_email],
        NotifyStatus.URL_ADDRESS: [send_webhook],
        NotifyStatus.EMAIL_URL_ADDRESS: [send_email, send_webhook],
    }

    if notify_status not in actions:
        raise ValueError("invalid notify status")

    for action in actions[notify_status]:
        action()
```

Observe how we've totally eliminated conditional statements from the `notify` function. The
key takeaway here is that the program flow is now flatter and easier to follow. The core
business logic is neatly tucked inside the `get_notify_status` routine, and the
`NotifyStatus` enum explicitly defines all the valid states that the system can be in. This
also means that if a new notification channel pops up, all we'll need to do is update three
flat constructs and write the corresponding tests instead of battling with the twisted
conditional statements that we started with. Not too shabby, eh?


[^1]: [Claude 2](https://www.anthropic.com/index/claude-2)
[^2]: [Bitmasks](https://stackoverflow.com/questions/10493411/what-is-bit-masking)
[^3]: [enum.Flag](https://docs.python.org/3/library/enum.html#enum.Flag)
[^4]: [Python bitwise operators](https://realpython.com/python-bitwise-operators/)
[^5]: [Functional core, imperative shell](https://www.destroyallsoftware.com/screencasts/catalog/functional-core-imperative-shell)
