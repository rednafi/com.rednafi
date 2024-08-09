---

title: Log context propagation in Python ASGI apps
date: 2024-08-06
tags:
    - Python
    - API

---

Let's say you have a web app that emits log messages from different layers. Your log shipper
collects and sends these messages to a destination like Datadog where you can query them.
One common requirement is to tag the log messages with some common attributes, which you can
use later to query them.

In distributed tracing, this tagging is usually known as context propagation[^1], where
you're attaching some contextual information to your log messages that you can use later for
query purposes. However, if you have to collect the context at each layer of your
application and pass it manually to the downstream ones, that'd make the whole process quite
painful.

Suppose you have a web view for an endpoint that calls another function to do something:

```python
async def view(request: Request) -> JSONResponse:
    # Collect contextual info from the header
    user_id = request.headers.get("Svc-User-Id")
    platform = request.headers.get("Svc-Platform")

    # Log the request with context
    logger.info(
        "Request started", extra={"user_id": user_id, "platform": platform}
    )

    await work()

    # Log the response too
    logger.info(
        "Request ended", extra={"user_id": user_id, "platform": platform}
    )

    return JSONResponse({"message": "Work, work work!"})


async def work() -> None:
    await asyncio.sleep(1)
    logger.info("Work done")
```

I'm using Starlette[^2] syntax for the above pseudocode, but this is valid for any generic
ASGI web app. The `view` procedure collects contextual information like `user_id` and
`platform` from the request headers. Then it tags the log statements before and after
calling the `work` function using the `extra` fields in the logger calls. This way, the log
messages have contextual info attached to them.

However, the `work` procedure also generates a log message, and that won't get tagged here.
We may be tempted to pass the contextual information to the `work` subroutine and use them
to tag the logs, but that'll quickly get repetitive and cumbersome. Passing a bunch of
arguments to a function just so it can tag some log messages also makes things unnecessarily
verbose. Plus, it's quite easy to forget to do so, which will leave you with orphan logs
with no way to query them.

It turns out we can write a simple middleware to tag log statements in a way where we won't
need to manually propagate the contextual information throughout the call chain. To
demonstrate that, here's a simple `get` endpoint server written in Starlette that'll just
return a canned response after logging a few events. The app structure looks as follows:

```txt
svc
├── __init__.py
├── log.py
├── main.py
├── middleware.py
└── view.py
```

## Configure the logger

The first step is to configure the application logger so that it emits structured log
statements in JSON where each message will look as follows:

```json
{
  "message": "Some log message",
  "timestamp": 1722794887376,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
```

Here's the log configuration logic:

```python
# log.py

import contextvars
import json
import logging
import time

# Set up the context variable with default values
default_context = {"user_id": "unknown", "platform": "unknown"}
log_context_var = contextvars.ContextVar(
    "log_context",
    default=default_context.copy(),
)


# Custom log formatter
class ContextAwareJsonFormatter(logging.Formatter):
    def format(self, record):
        log_data = {
            "message": record.getMessage(),
            # Add millisecond precision timestamp
            "timestamp": int(time.time() * 1000),
            # Get the context from the context variable in a concurreny-safe way
            # The context will be set in the middleware, so .get() will always
            # return the current context
            "tags": log_context_var.get(),
        }
        return json.dumps(log_data)


# Set up the logger
logger = logging.getLogger()
logger.setLevel(logging.INFO)

handler = logging.StreamHandler()
formatter = ContextAwareJsonFormatter()
handler.setFormatter(formatter)
logger.addHandler(handler)
```

Here's the log configuration logic:

```python
# log.py

import contextvars
import json
import logging
import time

# Set up the context variable with default values
default_context = {"user_id": "unknown", "platform": "unknown"}
log_context_var = contextvars.ContextVar(
    "log_context",
    default=default_context.copy(),
)


# Custom log formatter
class ContextAwareJsonFormatter(logging.Formatter):
    def format(self, record):
        log_data = {
            "message": record.getMessage(),
            # Add millisecond precision timestamp
            "timestamp": int(time.time() * 1000),
            # Get the context from the context variable in a concurrency-safe way
            # The context will be set in the middleware, so .get() will always return
            # the current context
            "tags": log_context_var.get(),
        }
        return json.dumps(log_data)


# Set up the logger
logger = logging.getLogger()
logger.setLevel(logging.INFO)

handler = logging.StreamHandler()
formatter = ContextAwareJsonFormatter()
handler.setFormatter(formatter)
logger.addHandler(handler)
```

The `contextvars` module manages context information across asynchronous tasks, preventing
context leakage between requests. We use a `log_context_var` context variable to store user
ID and platform information, ensuring each log entry includes relevant context for the
request.

The `ContextAwareJsonFormatter` formats log statements to include the message, timestamp in
milliseconds, and context tags. The context is retrieved using `log_context_var.get()`,
ensuring concurrency-safe access. The context variable is set in the middleware, so
`log_context_var.get()` always returns the current context for each request.

Next, we set up a `StreamHandler`, attach the `ContextAwareJsonFormatter` to it, and add the
handler to the root logger.

## Write a middleware that tags the log statements automatically

With log formatting out of the way, here's how to write the middleware to update the logger
so that all the log messages within a request-response cycle get automatically tagged:

```python
# middleware.py

import logging
from collections.abc import Awaitable, Callable

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

from svc.log import default_context, log_context_var


# Middleware for setting log context
class LogContextMiddleware(BaseHTTPMiddleware):
    async def dispatch(
        self,
        request: Request,
        call_next: Callable[[Request], Awaitable[Response]],
    ) -> Response:
        context = default_context.copy()
        user_id = request.headers.get("Svc-User-Id")
        platform = request.headers.get("Svc-Platform")

        if user_id:
            context["user_id"] = user_id
        if platform:
            context["platform"] = platform

        # Hydrate the log context
        token = log_context_var.set(context)

        try:
            logging.info("From middleware: request started")
            response = await call_next(request)
            logging.info("From middleware: request ended")
        finally:
            # Reset the context after the request is processed
            log_context_var.reset(token)

        return response
```

The `LogContextMiddleware` class inherits from `starlette.BaseHTTPMiddleware` and get
initialized with the application.

The `dispatch` method is called automatically for each request. It extracts `user_id` and
`platform` from the request headers and sets these values in the `log_context_var` to tag
log messages. Then it logs the incoming request, processes it, logs the outgoing response,
and then clears the context so that we don't leak the context information across requests.
This way, our view function won't need to be peppered with repetitive log statements.

## Write the simplified view

Setting up the logger and middleware drastically simplifies our endpoint view since we won't
need to tag the logs explicitly or add request-response logs in each view. It looks like
this now:

```python
# view.py

import asyncio
import logging

from starlette.requests import Request
from starlette.responses import JSONResponse


async def view(request: Request) -> JSONResponse:
    await work()
    logging.info("From view function: work finished")
    return JSONResponse({"message": f"Work work work!!!"})


async def work() -> None:
    logging.info("From work function: work started")
    await asyncio.sleep(1)
```

Notice there's no repetitive request-response log statements in the `view` function, and
we're not passing the log context anywhere explicitly. The middleware will ensure that the
request and response logs are always emitted and all the logs, including the one coming out
of the `work` function, are tagged with the contextual information.

## Wire everything together

The logging configuration and middleware can be wired up like this:

```python
# main.py

import uvicorn
from starlette.applications import Starlette
from starlette.middleware import Middleware
from starlette.routing import Route

from svc.middleware import LogContextMiddleware
from svc.view import view

middlewares = [Middleware(LogContextMiddleware)]

app = Starlette(routes=[Route("/", view)], middleware=middlewares)

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
```

To instantiate the logger config, we import `log.py` in the `__init__.py` module:

```python
# __init__.py

from svc import log  # noqa
```

Now the application can be started with:

```sh
python -m svc.main
```

And then we can make a request to the server:

```sh
curl http://localhost:8000/ -H 'Svc-User-Id: 1234' -H 'Svc-Platform: ios'
```

On the server, the request will emit the following log messages:

```plaintext
INFO:     Started server process [41848]
INFO:     Waiting for application startup.
INFO:     Application startup complete.
INFO:     Uvicorn running on http://0.0.0.0:8000 (Press CTRL+C to quit)
{
  "message": "From middleware: request started",
  "timestamp": 1723166008113,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
{
  "message": "From work function: work started",
  "timestamp": 1723166008113,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
{
  "message": "From view function: work finished",
  "timestamp": 1723166009114,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
{
  "message": "From middleware: request ended",
  "timestamp": 1723166009115,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
INFO:     127.0.0.1:54780 - "GET / HTTP/1.1" 200 OK
```

And we're done. You can find the fully working code in this GitHub gist[^3].

_Note: The previous version[^4] of this example wasn't concurrency safe and used a shared
logger filter, leaking context information during concurrent requests. This was pointed out
in this GitHub comment[^5]._

[^1]: [Context propagation](https://opentelemetry.io/docs/concepts/context-propagation/)

[^2]: [Starlette](https://www.starlette.io/)

[^3]: [Complete example](https://gist.github.com/rednafi/dc2016a8ea0e2405b943f023bfb18142)

[^4]:
    [Previous version of this example](https://web.archive.org/web/20240806220817/https://rednafi.com/python/log_context_propagation/)

[^5]:
    [GitHub discussion on context leakage in log statements](https://gist.github.com/rednafi/dc2016a8ea0e2405b943f023bfb18142?permalink_comment_id=5148207#gistcomment-5148207)
