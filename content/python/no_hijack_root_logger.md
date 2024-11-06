---
title: Please don't hijack my Python root logger
date: 2024-08-03
tags:
    - Python
---

With the recent explosion of LLM tools, I often like to kill time fiddling with different
LLM client libraries and SDKs in one-off scripts. Lately, I've noticed that some newer tools
frequently mess up the logger settings, meddling with my application logs. While it's less
common in more seasoned libraries, I guess it's worth rehashing why hijacking the root
logger isn't a good idea when writing libraries or other forms of reusable code.

In Python, when I say root logger[^1], I mean the logger instance that `logging.basicConfig`
acts on, or the one you get back when you don't specify a name in `logging.getLogger()`. The
root logger is for the application code to use and if you're a library author, you should
probably steer clear from it. If not, people using your code might get into situations as
follows.

Let's say there's a single file library named `lib.py` that decides to configure the root
logger:

```python
# lib.py
import logging

# Configuring the root logger here. Not a great idea!
logging.basicConfig(level=logging.DEBUG)


def frobnicate() -> None:
    # Using the root logger throughout the library code
    logging.debug("This is a debug message from the library.")
```

Now, let's say the user of `lib.py` imports the `frobnicate` function and configures the
root logger in the following manner:

```python
# main.py
import logging
from lib import frobnicate

# Library user attempts to reconfigure the root logger
logging.basicConfig(level=logging.INFO)


def main() -> None:
    # Use library code
    frobnicate()

    # Emit log message from the application code
    logging.info("This is an info message from the application.")


if __name__ == "__main__":
    main()
```

Since the application code has set the log threshold to `INFO`, you might think that running
the code snippet would only print the log message from the application. But instead, you'll
also get the `DEBUG` message from the library:

```txt
DEBUG:root:This is a debug message from the library.
INFO:root:This is an info message from the application.
```

It happens because before the application code had the chance to set the log level to
`INFO`, the library code hijacked the root logger and configured it during the import time
of `frobnicate`. You can test it by placing the `from lib import frobnicate` statement after
the `logging.basicConfig(...)` line in the `main.py` file. By doing so, the log
configuration in the application code gets to run before the library has the chance to
meddle with it.

This makes things confusing for the library user, and the logging how-to[^2] doc advises
against doing so:

> _It is strongly advised that you do not log to the root logger in your library. Instead,
> use a logger with a unique and easily identifiable name, such as the **name** for your
> library's top-level package or module. Logging to the root logger will make it difficult
> or impossible for the application developer to configure the logging verbosity or handlers
> of your library as they wish._

Solving this is quite straightforward. Avoid using the root logger in your library code.
Instead, instantiate your own logger instance and configure it with your heart's content.
This way, your users get to keep using the root logger as they like, and they can also tap
into the library's log messages whenever they need to.

Here's how to achieve that in the library:

```python
# lib.py
import logging

# Create a logger object for the library
logger = logging.getLogger("lib")


def frobnicate() -> None:
    # Only use this logger object throughout the library
    logger.debug("Debug message from the library")
```

Now the library logger no longer conflicts with the application log configuration. The
application code in the `main.py` from the previous section can remain the same and running
the snippet will only print out the `INFO` message this time:

```txt
INFO:root:This is an info message from the application.
```

This setup also lets the application code access and adjust the library's logger to suit its
needs. Here's how it can be done in the `main.py` file:

```python
# main.py
import logging
from lib import frobnicate

# Configure the root logger
logging.basicConfig(level=logging.INFO)

# Get the logger object for the library. This was already created in lib.py
lib_logger = logging.getLogger("lib")

# Set the log level for the library logger to DEBUG
lib_logger.setLevel(logging.DEBUG)


def main() -> None:
    frobnicate()
    logging.info("Info message from the main")


if __name__ == "__main__":
    main()
```

Above, the library user sets up the root logger as usual while also reconfiguring the
library's logger. It's the library author's job to properly name and initialize the logger
in the library code. The logger name and the default behavior should be well-documented as
well.

This allows the application code to retrieve and customize the logger as needed. Note that
calling `getLogger` with the same name always retrieves the same logger instance.

Also, you should avoid adding any handlers to your library's logger. Doing so can complicate
things for users who may want to attach their own handlers. The logging how-to guide
strongly warns against this:

> _It is strongly advised that you do not add any handlers other than `NullHandler` to your
> library's loggers. This is because the configuration of handlers is the prerogative of the
> application developer who uses your library. The application developer knows their target
> audience and what handlers are most appropriate for their application: if you add handlers
> ‘under the hood', you might well interfere with their ability to carry out unit tests and
> deliver logs which suit their requirements._

If you're looking for a real-life example of how to minimally configure your library's
logger, check out the httpx[^3] codebase. The logging behavior is well-documented here.

You can easily reconfigure the httpx logger in your application code while making an HTTP
request like this:

```python
# Your application code
import httpx
import logging

# Get the library's logger instance
httpx_logger = logging.getLogger("httpx")

# Set the logger's log level
httpx_logger.setLevel(logging.DEBUG)

# Define a handler
console_handler = logging.StreamHandler()

# Set the handler's log level
console_handler.setLevel(logging.DEBUG)

# Define a formatter
console_formatter = logging.Formatter("%(name)s - %(levelname)s - %(message)s")

# Add the handler to the library's logger instance
httpx_logger.addHandler(console_handler)

# Set the formatter for the handler
console_handler.setFormatter(console_formatter)

# Make a request that'll emit the log messages
httpx.get("https://httpbin.org/get")
```

Running the script will print the `DEBUG` messages as follows:

```txt
httpx - DEBUG - load_ssl_context verify=True cert=None trust_env=True http2=False
httpx - DEBUG - load_verify_locations cafile='.../site-packages/certifi/cacert.pem'
httpx - INFO - HTTP Request: GET https://httpbin.org/get "HTTP/1.1 200 OK"
```

[^1]:
    [Root logger](https://docs.python.org/3/library/logging.html#:~:text=Logged%20messages%20to%20the%20module%2Dlevel%20logger%20get%20forwarded%20to%20handlers%20of%20loggers%20in%20higher%2Dlevel%20modules%2C%20all%20the%20way%20up%20to%20the%20highest%2Dlevel%20logger%20known%20as%20the%20root%20logger%3B%20this%20approach%20is%20known%20as%20hierarchical%20logging)

[^2]: [Logging how-to](https://docs.python.org/3/howto/logging.html)

[^3]:
    [Logging in httpx](https://github.com/search?q=repo%3Aencode%2Fhttpx%20logging&type=code)
