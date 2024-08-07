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
application and pass it manually to the downstream ones, that'd make the whole logging
process quite painful.

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

    return JSONResponse({"message": "Did some work!"})


async def work() -> None:
    await asyncio.sleep(1)
    logger.info("Work done after 1 second")
```

I'm using Starlette[^2] syntax for the above pseudocode, but this is valid for any generic
ASGI web app. The `view` procedure collects contextual information like `user_id` and
`platform` from the request headers. Then it tags the log statements before and after
calling the `work` function using the `extra` fields in the logger calls. This way, the log
messages have contextual info attached to them.

However, the `work` procedure also generates a log message, and that won't get tagged here.
We may be tempted to pass the contextual information to the `work` subroutine and use them
to tag the logs, but that quickly gets repetitive and cumbersome. Passing a bunch of
arguments to a function just so it can tag some log messages also makes things unnecessarily
verbose. Plus, it's quite easy to forget to do so, which will leave you with logs with no
way to query them.

It turns out middlewares allow us to tag log statements in a way where we won't need to
manually propagate the contextual information throughout the call chain. To demonstrate how
to do it, here's a simple `get` endpoint server written in Starlette that'll just return a
canned response after logging a few things. The app structure looks as follows:

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

import logging
import json
import time
from typing import Any


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        log_record = {
            "message": record.getMessage(),
            # Defaults to current time in milliseconds if not set
            "timestamp": record.__dict__.get(
                "timestamp", int(time.time() * 1000)
            ),
            # Defaults to empty dict if not set
            "tags": record.__dict__.get("tags", {}),
        }
        return json.dumps(log_record)


class ContextFilter(logging.Filter):
    def __init__(self) -> None:
        super().__init__()
        self.context = {}

    def set_context(self, **kwargs: Any) -> None:
        self.context.update(kwargs)

    def filter(self, record: logging.LogRecord) -> bool:
        record.tags = self.context
        return True


# Set up logger
logger = logging.getLogger()
logger.setLevel(logging.INFO)  # Set the default logging level

console_handler = logging.StreamHandler()
console_handler.setLevel(logging.INFO)  # Set the logging level for the handler

context_filter = ContextFilter()  # Set filter
console_handler.addFilter(context_filter)
logger.addFilter(context_filter)

json_formatter = JsonFormatter()  # Set formatter
console_handler.setFormatter(json_formatter)

logger.addHandler(console_handler)  # Add handler to logger
```

Since this is application code, it's okay to configure the root logger. We define a
`JsonFormatter` that formats log statements by including the message, the current timestamp
in milliseconds, and any additional tags. If `timestamp` or `tags` aren't provided, the
formatter uses the current time and an empty dictionary.

The `ContextFilter` class defines a `set_context` method to set arbitrary contextual values
in the middleware (explained in the next section). The `filter` method in `ContextFilter`
updates the contextual information dynamically each time the logger emits a message,
ensuring every log entry includes relevant context like user ID or platform information.
Finally, we set up a custom handler, attach the `JsonFormatter` and `ContextFilter` to it,
and add this handler to the root logger instance.

## Write a middleware that tags the log statements automatically

With log formatting out of the way, here's how to write the middleware to update the logger
so that all the log messages within a request-response cycle get automatically tagged:

```python
# middleware.py

from collections.abc import Callable, Awaitable
import logging
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response
from svc.log import ContextFilter
from starlette.types import ASGIApp


class LogContextMiddleware(BaseHTTPMiddleware):
    def __init__(self, app: ASGIApp) -> None:
        super().__init__(app)
        self.logger = logging.getLogger()
        self.context_filter = next(
            f for f in self.logger.filters if isinstance(f, ContextFilter)
        )

    async def dispatch(
        self,
        request: Request,
        call_next: Callable[[Request], Awaitable[Response]],
    ) -> Response:
        # Extract user information from the request (headers or parameters)
        user_id = request.headers.get("Svc-User-ID", "unknown")
        platform = request.headers.get("Svc-Platform", "unknown")

        # Set context in the logger
        self.context_filter.set_context(user_id=user_id, platform=platform)

        # Log the incoming request
        self.logger.info("Handling request")

        response = await call_next(request)

        # Log the outgoing response
        self.logger.info("Finished handling request")

        # Clear context after request is handled
        self.context_filter.set_context(**{})

        return response
```

The `LogContextMiddleware` class inherits from `starlette.BaseHTTPMiddleware` and
initializes with the application. It fetches the root logger and the `ContextFilter`
instance during initialization.

The `dispatch` method is called automatically for each request. It extracts `user_id` and
`platform` from the request headers and sets these values in the `ContextFilter` to tag log
messages. Now the middleware logs the incoming request, processes it, logs the outgoing
response, and then clears the context so that we don't leak the context information across
requests. This way, our view won't need to be peppered with repetitive request and response
logging.

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
    return JSONResponse({"message": "Did some work!"})


async def work() -> None:
    await asyncio.sleep(1)
    logging.info("Work done after 1 second")
```

Notice there's no repetitive request-response log statements in the `view` function, and
we're not passing the log context anywhere explicitly. The middleware will ensure that the
request and response logs are always emitted and all the logs, including the one coming out
of the `work` function, are tagged with the contextual information.

## Wire everything together

The logging configuration and middleware can be wired up like this:

```python
# main.py

from starlette.routing import Route
import uvicorn
from starlette.applications import Starlette
from svc.middleware import LogContextMiddleware
from svc.view import view
from starlette.middleware import Middleware

middlewares = [Middleware(LogContextMiddleware)]

app = Starlette(routes=[Route("/", view)], middleware=middlewares)

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
```

To instantiate the logger config, we import the `log.py` in the `__init__.py` module:

```python
# __init__.py

from svc import log  # noqa
```

Now the application can be started with:

```sh
python -m svc.main
```

And then we can make a request to the server as follows:

```sh
curl http://localhost:8000/ -H 'Svc-User-Id: 1234' -H 'Svc-Platform: ios'
```

On the server, the request will emit the following log messages:

```txt
INFO:     Started server process [92156]
INFO:     Waiting for application startup.
INFO:     Application startup complete.
INFO:     Uvicorn running on http://0.0.0.0:8000 (Press CTRL+C to quit)
{
  "message": "Handling request",
  "timestamp": 1722968949312,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
{
  "message": "Work done after 1 second",
  "timestamp": 1722968950313,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
{
  "message": "Finished handling request",
  "timestamp": 1722968950313,
  "tags": {
    "user_id": "1234",
    "platform": "ios"
  }
}
INFO:     127.0.0.1:64863 - "GET / HTTP/1.1" 200 OK
```

And we're done. You can find the fully working code in this GitHub gist[^3].

[^1]: [Context propagation](https://opentelemetry.io/docs/concepts/context-propagation/)

[^2]: [Starlette](https://www.starlette.io/)

[^3]: [Complete example](https://gist.github.com/rednafi/dc2016a8ea0e2405b943f023bfb18142)
