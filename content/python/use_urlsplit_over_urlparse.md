---
title: Prefer urlsplit over urlparse to destructure URLs
Date: 2022-09-10
tags:
    - Python
---

TIL from this[^1] video that Python's `urllib.parse.urlparse`[^2] is quite slow at parsing
URLs. I've always used `urlparse` to destructure URLs and didn't know that there's a faster
alternative to this in the standard library. The official documentation also recommends the
alternative function.

The `urlparse` function splits a supplied URL into multiple seperate components and returns
a `ParseResult` object. Consider this example:

```python
In [1]: from urllib.parse import urlparse

In [2]: url = "https://httpbin.org/get?q=hello&r=22"

In [3]: urlparse(url)
Out[3]: ParseResult(
        scheme='https', netloc='httpbin.org',
        path='/get', params='', query='q=hello&r=22',
        fragment=''
    )
```

You can see how the function disassembles the URL and builds a `ParseResult` object with the
URL components. Along with this, the `urlparse` function can also parse an obscure type of
URL that you'll most likely never need. If you notice closely in the previous example,
you'll see that there's a `params` argument in the `ParseResult` object. This `params`
argument gets parsed whether you need it or not and that adds some overhead. The `params`
field will be populated if you have a URL like this:

```python
In [1]: from urllib.parse import urlparse

In [2]: url = "https://httpbin.org/get;a=mars&b=42?q=hello&r=22"

In [3]: urlparse(url)
Out[4]: ParseResult(
    scheme='https', netloc='httpbin.org', path='/get',
    params='a=mars&b=42', query='q=hello&r=22', fragment=''
    )
```

Notice the parts in the URL that appears after `https://httpbin.org/get`. There's a
semicolon and a few more parameters succeeding that—`;a=mars&b=42`. The resulting
`ParseResult` now has the `params` field populated with the parsed param value
`a=mars&b=42`. Unless you need this param support, there's a better and faster alternative
to this in the standard library. The `urlsplit`[^3] function does the same thing as
`urlparse` minus the param parsing and is twice as fast. Here's how you'd use `urlsplit`:

```python
In [1]: from urllib.parse import urlsplit

In [2]: url = "https://httpbin.org/get?q=hello&r=22"

In [3]: urlsplit(url)
Out[3]: SplitResult(
    scheme='https', netloc='httpbin.org', path='/get',
    query='q=hello&r=22', fragment=''
    )
```

The `urlsplit` function returns a `SplitResult` object similar to the `ParseResult` object
you've seen before. Notice there's no `param` argument in the output here. I measured the
speed difference like this:

```python
In [1]: from urllib.parse import urlparse, urlsplit

In [2]: url = "https://httpbin.org/get?q=hello&r=22"

In [3]: %timeit urlparse(url)
1.7 µs ± 2.91 ns per loop (
    mean ± std. dev. of 7 runs, 1,000,000 loops each)

In [4]: %timeit urlsplit(url)
885 ns ± 10.9 ns per loop (
    mean ± std. dev. of 7 runs, 1,000,000 loops each)
```

Wow, that's almost 2x speed improvement. Although this shouldn't be much of an issue in a
real codebase but it can matter if you are parsing URLs in a critical hot path.

[^1]:
    [python: don't use urlparse - Anthony Sottile](https://www.youtube.com/watch?v=ABJvdsIANds)

[^2]: [urlparse](https://docs.python.org/3/library/urllib.parse.html#urllib.parse.urlparse)
[^3]: [urlsplit](https://docs.python.org/3/library/urllib.parse.html#urllib.parse.urlsplit)
