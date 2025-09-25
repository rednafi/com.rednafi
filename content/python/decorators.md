---
title: Untangling Python decorators
date: 2020-05-13
tags:
    - Python
slug: decorators
aliases:
    - /python/decorators/
---

**_Updated on 2022-02-13_**: _Change functools import style._

When I first learned about Python decorators, using them felt like doing voodoo magic.
Decorators can give you the ability to add new functionalities to any callable without
actually touching or changing the code inside it. This can typically yield better
encapsulation and help you write cleaner and more understandable code. However, _decorator_
is considered as a fairly advanced topic in Python since understanding and writing it
requires you to have command over multiple additional concepts like first class objects,
higher order functions, closures etc. First, I'll try to introduce these concepts as
necessary and then unravel the core concept of decorator layer by layer. So let's dive in.

## First class objects

In Python, basically everything is an object and functions are regarded as first-class
objects. It means that functions can be passed around and used as arguments, just like any
other object (string, int, float, list, and so on). You can assign functions to variables
and treat them like any other objects. Consider this example:

```py
def func_a():
    return "I was angry with my friend."


def func_b():
    return "I told my wrath, my wrath did end"


def func_c(*funcs):
    for func in funcs:
        print(func())


main_func = func_c
main_func(func_a, func_b)
```

```txt
>>> I was angry with my friend.
>>> I told my wrath, my wrath did end
```

The above example demonstrates how Python treats functions as first class citizens. First, I
defined two functions, `func_a` and `func_b` and then `func_c` takes them as parameters.
`func_c` runs the functions taken as parameters and prints the results. Then we assign
`func_c` to variable `main_func`. Finally, we run `main_func` and it behaves just like
`func_c`.

## Higher order functions

Python also allows you to use functions as return values. You can take in another function
and return that function or you can define a function within another function and return the
inner function.

```py
def higher(func):
    """This is a higher order function.
    It returns another function.
    """

    return func


def lower():
    return "I'm hunting high and low"


higher(lower)
```

```txt
>>> <function __main__.lower()>
```

Now you can assign the result of `higher` to another variable and execute the output
function.

```py
h = higher(lower)
h()
```

```txt
>>> "I'm hunting high and low"
```

Let's look into another example where you can define a nested function within a function and
return the nested function instead of its result.

```py
def outer():
    """Define and return a nested function from another function."""

    def inner():
        return "Hello from the inner func"

    return inner


inn = outer()
inn()
```

```txt
>>> 'Hello from the inner func'
```

Notice how the nested function `inner` was defined inside the `outer` function and then the
return statement of the `outer` function returned the nested function. After definition, to
get to the nested function, first we called the `outer` function and received the result as
another function. Then executing the result of the `outer` function prints out the message
from the `inner` function.

## Closures

You saw examples of inner functions at work in the previous section. Nested functions can
access variables of the enclosing scope. In Python, these non-local variables are read only
by default and we must declare them explicitly as non-local (using `nonlocal` keyword) in
order to modify them. Following is an example of a nested function accessing a non-local
variable.

```py
def burger(name):
    def ingredients():
        if name == "deli":
            return ("steak", "pastrami", "emmental")
        elif name == "smashed":
            return ("chicken", "nacho cheese", "jalapeno")
        else:
            return None

    return ingredients
```

Now run the function,

```py
ingr = burger("deli")
ingr()
```

```txt
>>> ('steak', 'pastrami', 'emmental')
```

Well, that's unusual.

The `burger` function was called with the string `deli` and the returned function was bound
to the name `ingr`. On calling `ingr()`, the message was still remembered and used to derive
the outcome although the outer function `burger` had already finished its execution.

This technique by which some data ("deli") gets attached to the code is called closure in
Python. The value in the enclosing scope is remembered even when the variable goes out of
scope or the function itself is removed from the current namespace. Decorators uses the idea
of non-local variables multiple times and soon you'll see how.

## Writing a basic decorator

With these prerequisites out of the way, let's go ahead and create your first simple
decorator.

```py
def deco(func):
    def wrapper():
        print("This will get printed before the function is called.")
        func()
        print("This will get printed after the function is called.")

    return wrapper
```

Before using the decorator, let's define a simple function without any parameters.

```py
def ans():
    print(42)
```

Treating the functions as first-class objects, you can use your decorator like this:

```py
ans = deco(ans)
ans()
```

```txt
>>> This will get printed before the function is called.
    42
    This will get printed after the function is called.
```

In the above two lines, you can see a very simple decorator in action. Our `deco` function
takes in a target function, manipulates the target function inside a `wrapper` function and
then returns the `wrapper` function. Running the function returned by the decorator, you'll
get your modified result. To put it simply, decorators wraps a function and modifies its
behavior.

> The decorator function runs at the time the decorated function is imported/defined, not
> when it is called.

Before moving onto the next section, let's see how we can get the return value of target
function instead of just printing it.

```py
def deco(func):
    """This modified decorator also returns the result of func."""

    def wrapper():
        print("This will get printed before the function is called.")
        ret = func()
        print("This will get printed after the function is called.")
        return ret

    return wrapper


def ans():
    return 42
```

In the above example, the wrapper function returns the result of the target function and the
wrapper itself. This makes it possible to get the result of the modified function.

```py
ans = deco(ans)
print(ans())
```

```txt
>>> This will get printed before the function is called.
    This will get printed after the function is called.
    42
```

Can you guess why the return value of the decorated function appeared in the last line
instead of in the middle like before?

## The @ syntactic sugar

The way you've used decorator in the last section might feel a little clunky. First, you
have to type the name `ans` three times to call and use the decorator. Also, it becomes
harder to tell apart where the decorator is actually working. So Python allows you to use
decorator with the special syntax `@`. You can apply your decorators while defining your
functions, like this:

```py
@deco
def func(): ...


# Now call your decorated function just like a normal one
func()
```

Sometimes the above syntax is called the **pie syntax** and it's just a syntactic sugar for
`func = deco(func)`.

## Decorating functions with arguments

The naive decorator that we've implemented above will only work for functions that take no
arguments. It'll fail and raise `TypeError` if your try to decorate a function having
arguments with `deco`. Now let's create another decorator called `yell` which will take in a
function that returns a string value and transform that string value to uppercase.

```py
def yell(func):
    def wrapper(*args, **kwargs):
        ret = func(*args, **kwargs)
        ret = ret.upper() + "!"
        return ret

    return wrapper
```

Create the target function that returns string value.

```py
@yell
def hello(name):
    return f"Hello {name}"
```

```py
hello("redowan")
```

```txt
>>> 'HELLO REDOWAN!'
```

Function `hello` takes a `name:string` as parameter and returns a message as string. Look
how the `yell` decorator is modifying the original return string, transforming that to
uppercase and adding an extra `!` sign without directly changing any code in the `hello`
function.

## Solving identity crisis

In Python, you can introspect any object and its properties via the interactive shell. A
function knows its identity, docstring etc. For instance, you can inspect the built in
`print` function in the following ways:

```py
print
```

```txt
>>> <function print>
```

```py
print.__name__
```

```txt
>>> 'print'
```

```py
print.__doc__
```

```txt
>>> "print(value, ..., sep=' ', end='\\n', file=sys.stdout, flush=False)\n\nPrints the
values to a stream, or to sys.stdout by default.\nOptional keyword arguments:\nfile:  a
file-like object (stream); defaults to the current sys.stdout.\nsep:   string inserted
between values, default a space.\nend:   string appended after the last value, default
a newline.\nflush: whether to forcibly flush the stream."
```

```py
help(print)
```

```txt
>>> Help on built-in function print in module builtins:

    print(...)
    print(value, ..., sep=' ', end='\n', file=sys.stdout, flush=False)

    Prints the values to a stream, or to sys.stdout by default.
    Optional keyword arguments:
    file:  a file-like object (stream); defaults to the current sys.stdout.
    sep:   string inserted between values, default a space.
    end:   string appended after the last value, default a newline.
    flush: whether to forcibly flush the stream.
```

This introspection works similarly for functions that you defined yourself. I'll be using
the previously defined `hello` function.

```py
hello.__name__
```

```txt
>>> 'wrapper'
```

```py
help(hello)
```

```txt
>>> Help on function wrapper in module __main__:
    wrapper(*args, **kwargs)
```

Now what's going on there. The decorator `yell` has made the function `hello` confused about
its own identity. Instead of reporting its own name, it takes the identity of the inner
function `wrapper`. This can be confusing while doing debugging. You can fix this by using
builtin `wraps` decorator from the `functools` module. This will make sure that the original
identity of the decorated function stays preserved.

```py
import functools


def yell(func):
    @wraps(func)
    def wrapper(*args, **kwargs):
        ret = func(*args, **kwargs)
        ret = ret.upper() + "!"
        return ret

    return wrapper


@yell
def hello(name):
    "Hello from the other side."
    return f"Hello {name}"
```

```py
hello("Galaxy")
```

```txt
>>> 'HELLO GALAXY!'
```

Introspecting the `hello` function decorated with modified decorator will give you the
desired result.

```py
hello.__name__
```

```txt
>>> 'hello'
```

```py
help(hello)
```

```txt
>>> Help on function hello in module __main__:

    hello(name)
        Hello from the other side.
```

## Decorators in the wild

Before moving on to the next section let's see a few real world examples of decorators. To
define all the decorators, we'll be using the following template that we've perfected so
far.

```py
from functools import wraps


def decorator(func):
    @wraps(func)
    def wrapper(*args, **kwargs):
        # Do something before
        ret = func(*args, **kwargs)
        # Do something after
        return ret

    return wrapper
```

### Timer

Timer decorator will help you time your callables in a non-intrusive way. It can help you
while debugging and profiling your functions.

```py
from functools import wraps
from time import perf_counter


def timer(func):
    """This decorator prints out the execution time of a callable."""

    @wraps(func)
    def wrapper(*args, **kwargs):
        start_time = perf_counter()
        ret = func(*args, **kwargs)
        end_time = perf_counter()
        run_time = end_time - start_time
        print(f"Finished running {func.__name__} in {run_time:.4f} seconds.")
        return ret

    return wrapper


@timer
def dothings(n_times):
    for _ in range(n_times):
        return sum((i**3 for i in range(100_000)))
```

In the above way, we can introspect the time it requires for function `dothings` to complete
its execution.

```py
dothings(100_000)
```

```txt
>>> Finished running dothings in 0.0353 seconds.
    24999500002500000000
```

### Exception logger

Just like the `timer` decorator, we can define a logger decorator that will log the state of
a callable. For this demonstration, I'll be defining a exception logger that will show
additional information like timestamp, argument names when an exception occurs inside of the
decorated callable.

```py
from functools import wraps
from datetime import datetime


def logexc(func):
    @wraps(func)
    def wrapper(*args, **kwargs):
        # Stringify the arguments
        args_rep = [repr(arg) for arg in args]
        kwargs_rep = [f"{k}={v!r}" for k, v in kwargs.items()]
        sig = ", ".join(args_rep + kwargs_rep)

        # Try running the function
        try:
            return func(*args, **kwargs)
        except Exception as e:
            print(
                "Time: ",
                datetime.now().strftime("%Y-%m-%d [%H:%M:%S]"),
            )
            print("Arguments: ", sig)
            print("Error:\n")
            raise

    return wrapper


@logexc
def divint(a, b):
    return a / b
```

Let's invoke ZeroDivisionError to see the logger in action.

```py
divint(1, 0)
```

```txt
>>> Time:  2020-05-12 [12:03:31]
    Arguments:  1, 0
    Error:

        ------------------------------------------------------------

        ZeroDivisionError         Traceback (most recent call last)
        ....
```

The decorator first prints a few info regarding the function and then raises the original
error.

### Validation & runtime checks

Python's type system is strongly typed, but very dynamic. For all its benefits, this means
some bugs can try to creep in, which more statically typed languages (like Java) would catch
at compile time. Looking beyond even that, you may want to enforce more sophisticated,
custom checks on data going in or out. Decorators can let you easily handle all of this, and
apply it to many functions at once.

Imagine this: you have a set of functions, each returning a dictionary, which (among other
fields) includes a field called “summary.” The value of this summary must not be more than
30 characters long; if violated, that's an error. Here is a decorator that raises a
`ValueError` if that happens:

```py
from functools import wraps


def validate_summary(func):
    @wraps(func)
    def wrapper(*args, **kwargs):
        ret = func(*args, **kwargs)
        if len(ret["summary"]) > 30:
            raise ValueError("Summary exceeds 30 character limit.")
        return ret

    return wrapper


@validate_summary
def short_summary():
    return {"summary": "This is a short summary"}


@validate_summary
def long_summary():
    return {"summary": "This is a long summary that exceeds character limit."}


print(short_summary())
print(long_summary())
```

```txt
>>> {'summary': 'This is a short summary'}

    -------------------------------------------------------------------

    ValueError                       Traceback (most recent call last)

    <ipython-input-178-7375d8e2a623> in <module>
            19
            20 print(short_summary())
    ---> 21 print(long_summary())
    ...
```

### Retry

Imagine a situation where your defined callable fails due to some I/O related issues and
you'd like to retry that again. Decorator can help you to achieve that in a reusable manner.
Let's define a `retry` decorator that will rerun the decorated function multiple times if an
HTTP error occurs.

```py
import requests
from functools import wraps


def retry(func):
    """This will rerun the decorated callable 3 times if
    the callable encounters http 500/404 error."""

    @wraps(func)
    def wrapper(*args, **kwargs):
        n_tries = 3
        tries = 0
        while True:
            resp = func(*args, **kwargs)
            if (
                resp.status_code == 500
                or resp.status_code == 404
                and tries < n_tries
            ):
                print(f"retrying... ({tries})")
                tries += 1
                continue
            break
        return resp

    return wrapper


@retry
def getdata(url):
    resp = requests.get(url)
    return resp


resp = getdata("https://httpbin.org/get/1")
resp.text
```

```txt
>>> retrying... (0)
    retrying... (1)
    retrying... (2)

    '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">\n<title>404 Not Found</
    title>\n<h1>Not Found</h1>\n<p>The requested URL was not found on the server.  If
    you entered the URL manually please check your spelling and try again.</p>\n'
```

## Applying multiple decorators

You can apply multiple decorators to a function by stacking them on top of each other. Let's
define two simple decorators and use them both on a function.

```py
from functools import wraps


def greet(func):
    """Greet in English."""

    @wraps(func)
    def wrapper(*args, **kwargs):
        ret = func(*args, **kwargs)
        return "Hello " + ret + "!"

    return wrapper


def flare(func):
    """Add flares to the string."""

    @wraps(func)
    def wrapper(*args, **kwargs):
        ret = func(*args, **kwargs)
        return "🎉 " + ret + " 🎉"

    return wrapper


@flare
@greet
def getname(name):
    return name


getname("Nafi")
```

```txt
>>> '🎉 Hello Nafi! 🎉'
```

The decorators are called in a bottom up order. First, the decorator `greet` gets applied on
the result of `getname` function and then the result of `greet` gets passed to the `flare`
decorator. The decorator stack above can be written as `flare(greet(getname(name)))`. Change
the order of the decorators and see what happens!

## Decorators with arguments

While defining the `retry` decorator in the previous section, you may have noticed that I've
hard coded the number of times I'd like the function to retry if an error occurs. It'd be
handy if you could inject the number of tries as a parameter into the decorator and make it
work accordingly. This is not a trivial task and you'll need three levels of nested
functions to achieve that.

Before doing that let's cook up a trivial example of how you can define decorators with
parameters.

```py
from functools import wraps


def joinby(delimiter=" "):
    """This decorator splits the string output of the
    decorated function by a single space and then joins
    them using a user specified delimiter."""

    def outer_wrapper(func):
        @wraps(func)
        def inner_wrapper(*args, **kwargs):
            ret = func(*args, **kwargs)
            ret = ret.split(" ")
            ret = delimiter.join(ret)
            return ret

        return inner_wrapper

    return outer_wrapper


@joinby(delimiter=",")
def hello(name):
    return f"Hello {name}!"


@joinby(delimiter=">")
def greet(name):
    return f"Greetings {name}!"


@joinby()
def goodbye(name):
    return f"Goodbye {name}!"


print(hello("Nafi"))
print(greet("Redowan"))
print(goodbye("Delowar"))
```

```txt
>>> Hello,Nafi!
    Greetings>Redowan!
    Goodbye Delowar!
```

The decorator `joinby` takes a single parameter called `delimiter`. It splits the string
output of the decorated function by a single space and then joins them using the user
defined delimiter specified in the `delimiter` argument. The three layer nested definition
looks scary but we'll get to that in a moment. Notice how you can use the decorator with
different parameters. In the above example, I've defined three different functions to
demonstrate the usage of `joinby`. It's important to note that in case of a decorator that
takes parameters, you'll always need to pass something to it and even if you don't want to
pass any parameter (run with the default), you'll still need to decorate your function with
`deco()` instead of `deco`. Try changing the decorator on the `goodbye` function from
`joinby()` to `joinby` and see what happens.

Typically, a decorator creates and returns an inner wrapper function but here in the
`repeat` decorator, there is an inner function within another inner function. This almost
looks like a _dream within a dream_ from the movie Inception.

There are a few subtle things happening in the `joinby()` function:

- Defining `outer_wrapper()` as an inner function means that `repeat()` will refer to a
  function object `outer_wrapper`.

- The `delimiter` argument is seemingly not used in `joinby()` itself. But by passing
  `delimiter` a closure is created where the value of `delimiter` is stored until it will be
  used later by `inner_wrapper()`

## Decorators with & without arguments

You saw earlier that a decorator specifically designed to take parameters can't be used
without parameters; you need to at least apply parenthesis after the decorator `deco()` to
use it without explicitly providing the arguments. But what if you want to design one that
can used both with and without arguments. Let's redefine the `joinby` decorator so that you
can use it with parameters or just like an ordinary parameter-less decorator that we've seen
before.

```py
from functools import wraps


def joinby(_func=None, *, delimiter=" "):
    """This decorator splits the string output
    of a function by a single space and then joins that
    using a user specified delimiter."""

    def outer_wrapper(func):
        @wraps(func)
        def inner_wrapper(*args, **kwargs):
            ret = func(*args, **kwargs)
            ret = ret.split(" ")
            ret = delimiter.join(ret)
            return ret

        return inner_wrapper

    # This part enables you to use the decorator with/without arguments
    if _func is None:
        return outer_wrapper
    else:
        return outer_wrapper(_func)


@joinby(delimiter=",")
def hello(name):
    return f"Hello {name}!"


@joinby
def greet(name):
    return f"Greetings {name}!"


print(hello("Nafi"))
print(greet("Redowan"))
```

```txt
>>> Hello,Nafi!
    Greetings Redowan!
```

Here, the `_func` argument acts as a marker, noting whether the decorator has been called
with arguments or not:

If `joinby` has been called without arguments, the decorated function will be passed in as
`_func`. If it has been called with arguments, then `_func` will be None. The \* in the
argument list means that the remaining arguments can't be called as positional arguments.
This time you can use `joinby` with or without arguments and function `hello` and `greet`
above demonstrate that.

## A generic pattern

Personally, I find it cumbersome how you need three layers of nested functions to define a
generalized decorator that can be used with or without arguments. David Beazley in his
Python Cookbook[^1] shows an excellent way to define generalized decorators without writing
three levels of nested functions. It uses the built in `functools.partial` function to
achieve that. The following is a pattern you can use to define generalized decorators in a
more elegant way:

```py
from functools import partial, wraps


def decorator(func=None, foo="spam"):
    if func is None:
        return partial(decorator, foo=foo)

    @wraps(func)
    def wrapper(*args, **kwargs):
        # Do something with `func` and `foo`, if you're so inclined
        pass

    return wrapper


# Applying decorator without any parameter
@decorator
def f(*args, **kwargs):
    pass


# Applying decorator with extra parameter
@decorator(foo="buzz")
def f(*args, **kwargs):
    pass
```

Let's redefine our `retry` decorator using this pattern.

```py
from functools import partial, wraps


def retry(func=None, n_tries=4):
    if func is None:
        return partial(retry, n_tries=n_tries)

    @wraps(func)
    def wrapper(*args, **kwargs):
        tries = 0
        while True:
            ret = func(*args, **kwargs)
            if (
                ret.status_code == 500
                or ret.status_code == 404
                and tries < n_tries
            ):
                print(f"retrying... ({tries})")
                tries += 1
                continue
            break
        return ret

    return wrapper


@retry
def getdata(url):
    resp = requests.get(url)
    return resp


@retry(n_tries=2)
def getdata_(url):
    resp = requests.get(url)
    return resp


resp1 = getdata("https://httpbin.org/get/1")
print("-----------------------")
resp2 = getdata_("https://httpbin.org/get/1")
```

```txt
>>> retrying... (0)
    retrying... (1)
    retrying... (2)
    retrying... (3)
    -----------------------
    retrying... (0)
    retrying... (1)
```

In this case, you do not have to write three level nested functions and the
`functools. partial` takes care of that. Partials can be used to make new derived functions
that have some input parameters pre-assigned.Roughly `partial` does the following:

```py
def partial(func, *part_args):
    def wrapper(*extra_args):
        args = list(part_args)
        args.extend(extra_args)
        return func(*args)

    return wrapper
```

This eliminates the need to write multiple layers of nested factory function get a
generalized decorator.

## Defining decorators with classes

This time, I'll be using a class to compose a decorator. Classes can be handy to avoid
nested architecture while writing decorators. Also, it can be helpful to use a class while
writing stateful decorators. You can follow the pattern below to compose decorators with
classes.

```py
import functools


class ClassDeco:
    def __init__(self, func):
        # Does the work of the 'functools.wraps' in methods.
        functools.update_wrapper(self, func)
        self.func = func

    def __call__(self, *args, **kwargs):
        # You can add some code before the function call
        ret = self.func(*args, **kwargs)
        # You can also add some code after the function call

        return ret
```

Let's use the above template to write a decorator named `Emphasis` that will add bold tags
`<b></b>`to the string output of a function.

```py
import functools


class Emphasis:
    def __init__(self, func):
        functools.update_wrapper(self, func)
        self.func = func

    def __call__(self, *args, **kwargs):
        ret = self.func(*args, **kwargs)
        return "<b>" + ret + "</b>"


@Emphasis
def hello(name):
    return f"Hello {name}"


print(hello("Nafi"))
print(hello("Redowan"))
```

```txt
>>> <b>Hello Nafi</b>
    <b>Hello Redowan</b>
```

The `__init__()` method stores a reference to the function num_calls and can do other
necessary initialization. The `__call__()` method will be called instead of the decorated
function. It does essentially the same thing as the `wrapper()` function in our earlier
examples. Note that you need to use the `functools.update_wrapper()` function instead of
`@functools.wraps`.

Before moving on, let's write a stateful decorator using classes. Stateful decorators can
remember the state of their previous run. Here's a stateful decorator called `Tally` that'll
keep track of the number of times decorated functions are called in a dictionary. The keys
of the dictionary will hold the names of the functions and the corresponding values will
hold the call count.

```py
import functools


class Tally:
    def __init__(self, func):
        functools.update_wrapper(self, func)
        self.func = func
        self.tally = {}
        self.n_calls = 0

    def __call__(self, *args, **kwargs):
        self.n_calls += 1
        self.tally[self.func.__name__] = self.n_calls

        print("Callable Tally:", self.tally)
        return self.func(*args, **kwargs)


@Tally
def hello(name):
    return f"Hello {name}!"


print(hello("Redowan"))
print(hello("Nafi"))
```

```txt
>>> Callable Tally: {'hello': 1}
    Hello Redowan!
    Callable Tally: {'hello': 2}
    Hello Nafi!
```

## A few more examples

### Caching return values

Decorators can provide an elegant way of memoizing function return values. Imagine you have
an expensive API and you'd like call that as few times as possible. The idea is to save and
cache values returned by the API for particular arguments, so that if those arguments appear
again, you can serve the results from the cache instead of calling the API again. This can
dramatically improve your applications' performance. Here I've simulated an expensive API
call and provided caching with a decorator.

```py
import time


def api(a):
    """API takes an integer and returns the square value of it.
    To simulate a time consuming process, I've added some time delay to it.
    """

    print("The API has been called...")

    # This will delay 3 seconds
    time.sleep(3)

    return a * a


api(3)
```

```txt
>>> The API has been called...
    9
```

You'll see that running this function takes roughly 3 seconds. To cache the result, we can
use Python's built in functools.lru_cache to save the result against an argument in a
dictionary and serve that when it encounters the same argument again. The only drawback here
is, all the arguments need to be hashable.

```py
import functools


@functools.lru_cache(maxsize=32)
def api(a):
    """API takes an integer and returns the square value of it.
    To simulate a time consuming process, I've added some time delay to it.
    """

    print("The API has been called...")

    # This will delay 3 seconds
    time.sleep(3)

    return a * a


api(3)
```

```txt
>>> 9
```

Least Recently Used (LRU) Cache organizes items in order of use, allowing you to quickly
identify which item hasn't been used for the longest amount of time. In the above case, the
parameter `max_size` refers to the maximum numbers of responses to be saved up before it
starts deleting the earliest ones. While you run the decorated function, you'll see first
time it'll take roughly 3 seconds to return the result. But if you rerun the function again
with the same parameter it'll spit the result from the cache almost instantly.

### Unit Conversion

The following decorator converts length from SI units to multiple other units without
polluting your target function with conversion logics.

```py
from functools import wraps


def convert(func=None, convert_to=None):
    """This converts value from meter to others."""

    if func is None:
        return partial(convert, convert_to=convert_to)

    @wraps(func)
    def wrapper(*args, **kwargs):
        print(f"Conversion unit: {convert_to}")

        ret = func(*args, **kwargs)

        # Adding conversion rules
        if convert_to is None:
            return ret

        elif convert_to == "km":
            return ret / 1000

        elif convert_to == "mile":
            return ret * 0.000621371

        elif convert_to == "cm":
            return ret * 100

        elif convert_to == "mm":
            return ret * 1000

        else:
            raise ValueError("Conversion unit is not supported.")

    return wrapper
```

Let's use that on a function that returns the area of a rectangle.

```py
@convert(convert_to="mile")
def area(a, b):
    return a * b


area(1, 2)
```

```txt
>>> Conversion unit: mile
    0.001242742
```

Using the convert decorator on the area function shows how it prints out the transformation
unit before returning the desired result. Experiment with other conversion units and see
what happens.

### Function registration

The following is an example of registering logger function in Flask framework. The decorator
`register_logger` doesn't make any change to the decorated `logger` function. Rather it
takes the function and registers it in a list called `logger_list` every time it's invoked.

```py
from flask import Flask, request

app = Flask(__name__)
logger_list = []


def register_logger(func):
    logger_list.append(func)
    return func


def run_loggers(request):
    for logger in logger_list:
        logger(request)


@register_logger
def logger(request):
    print(request.method, request.path)


@app.route("/")
def index():
    run_loggers(request)
    return "Hello World!"


if __name__ == "__main__":
    app.run(host="localhost", port="5000")
```

If you run the server and hit the `http://localhost:5000/` url, it'll greet you with a
`Hello World!` message. Also you'll able to see the printed `method` and `path` of your HTTP
request on the terminal. Moreover, if you inspect the `logger_list`, you'll find the
registered logger there. You'll find a lot more real life usage of decorators in the Flask
framework.

[^1]: [Python Cookbook - David Beazley](https://realpython.com/asins/1449340377/)

[^2]:
    [Primer on Python decorator - Real Python](https://realpython.com/primer-on-python-decorators/)
    [^2]

[^3]:
    [Decorators in Python - Datacamp](https://www.datacamp.com/community/tutorials/decorators-python)
    [^3]

[^4]:
    [5 reasons you need to write python decorators](https://www.oreilly.com/content/5-reasons-you-need-to-learn-to-write-python-decorators/)
    [^4]
