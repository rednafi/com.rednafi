---
title: Pushing real-time updates to clients with Server-Sent Events (SSEs)
date: 2023-04-08
slug: server-sent-events
aliases:
    - /python/server_sent_events/
tags:
    - Python
    - Networking
---

In multi-page web applications, a common workflow is where a user:

- Loads a specific page or clicks on some button that triggers a long-running task.
- On the server side, a background worker picks up the task and starts processing it
  asynchronously.
- The page shouldn't reload while the task is running.
- The backend then communicates the status of the long-running task in real-time.
- Once the task is finished, the client needs to display a success or an error message
  depending on the final status of the finished task.

The de facto tool for handling situations where real-time bidirectional communication is
necessary is WebSocket[^1]. However, in the case above, you can see that the communication
is mostly unidirectional where the client initiates some action in the server and then the
server continuously pushes data to the client during the lifespan of the background job.

In Django, I usually go for the channels[^2] library whenever I need to do any real-time
communication over WebSockets. It's a fantastic tool if you need real-time full duplex
communication between the client and the server. But it can be quite cumbersome to set up,
especially if you're not taking full advantage of it or not working with Django. Moreover,
WebSockets can be quite flaky and usually have quite a bit of overhead. So, I was looking
for a simpler alternative and found out that Server-Sent Events (SSEs) work quite nicely
when all I needed was to stream some data from the server to the client in a unidirectional
manner.

## Server-Sent Events (SSEs)

Server-Sent Events (SSE)[^3] is a way for a web server to send real-time updates to a web
page without the need for the page to repeatedly ask for updates. Instead of the page asking
the server for new data every few seconds, the server can just send updates as they happen,
like a live stream. This is useful for things like live chat, news feeds, and stock tickers
but won't work in situations where you also need to send real-time updates from the client
to the server. In the latter scenarios, WebSockets are kind of your only option.

SSEs are sent over traditional HTTP. That means they don't need any special protocol or
server implementation to get working. WebSockets on the other hand, need full-duplex
connections and new WebSocket servers like Daphne to handle the protocol. In addition, SSEs
have a variety of features that WebSockets lack by design such as automatic reconnection,
event IDs, and the ability to send arbitrary events. This is quite nice since on the
browser, you won't have to write additional logic to handle reconnections and stuff.

The biggest reason why I wanted to explore SSE is because of its simplicity and the fact
that it plays in the HTTP realm. If you want to learn more about how SSEs stack up against
WebSockts, I recommend this post[^4] by Germano Gabbianelli.

## The wire protocol

The wire protocol works on top of HTTP and is quite simple. The server needs to send the
data maintaining the following structure:

```txt
HTTP/1.1 200 OK
date: Sun, 02 Apr 2023 20:17:53 GMT
server: uvicorn
content-type: text/event-stream
access-control-allow-origin: *
cache-control: no-cache
Transfer-Encoding: chunked

event: start
data: streaming started

id: 0

data: message 1

: this is a comment

data: message 2

retry: 5000
```

Here, the server header needs to set the MIME type to `text/event-stream` and ask the client
not to cache the response by setting the `cache-control` header to `no-cache`. Next, in the
message payload, only the `data` field is required, everything else is optional. Let's break
down the message structure:

- `event`: This is an optional field that specifies the name of the event. If present, it
  must be preceded by the string 'event:'. If not present, the event is considered to have
  the default name 'message'.

- `id`: This is an optional field that assigns an ID to the event. If present, it must be
  preceded by the string 'id:'. Clients can use this ID to resume an interrupted connection
  and receive only events that they have not yet seen.

- `data:` This field is required and contains the actual message data that the server wants
  to send to the client. It must be preceded by the string 'data:' and can contain any
  string of characters.

- `retry`: This is an optional field that specifies the number of milliseconds that the
  client should wait before attempting to reconnect to the server in case the connection is
  lost. If present, it must be preceded by the string 'retry:'.

Each message must end with double newline characters `("\n\n")`. Yep, this is part of the
protocol. The server can send multiple messages in a single HTTP response, and each message
will be treated as a separate event by the client.

## A simple example

In this section, I'll prop up a simple HTTP streaming server with starlette[^5] and collect
the events from the browser. Here's the complete server implementation:

```py
# server.py
from __future__ import annotations

import asyncio
import logging
from typing import AsyncGenerator

from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import Response, StreamingResponse
from starlette.routing import Route

logging.basicConfig(level=logging.INFO)


# This is just so that you can head over to an index page
# and run the client side code.
async def index(request: Request) -> Response:
    return Response("SSE demo", media_type="text/plain")


async def stream(request: Request) -> StreamingResponse:
    async def _stream() -> AsyncGenerator[str, None]:
        attempt = 0  # Give up after 3 attempts.

        while True:
            # Start sending messages.
            yield "event: start\n"  # Sets the type of the next message to 'start'.
            yield "data: streaming started\n\n"  # A 'start' event message.

            yield f"id: {attempt}\n\n"  # Sends the id field.

            yield "data: message 1\n\n"  # A default event message.

            yield ": this is a comment\n\n"  # Keep-alive comment.

            yield "data: message 2\n\n"  # Another default event message.

            yield "retry: 5000\n\n"  # Controls autoretry from the client side (ms).

            # Wait for a second so that we're not flooding the client with messages.
            await asyncio.sleep(1)
            attempt += 1

            # Give up after 3 attempts to avoid dangling connections.
            if attempt == 3:
                # Close the connection
                yield "data: closing connection\n\n"
                break

    response = StreamingResponse(
        _stream(),
        headers={
            "Content-Type": "text/event-stream",
            "Access-Control-Allow-Origin": "*",
            "Cache-Control": "no-cache",
        },
    )
    return response


routes = [
    Route("/", endpoint=index),
    Route("/stream", endpoint=stream),
]


app = Starlette(debug=True, routes=routes)
```

The server exposes a `/stream` endpoint that will just continuously send data to any
connected client. The `stream` function returns a `StreamingResponse` object that the
framework uses to send SSE messages to the client. Internally, it defines an asynchronous
generator function `_stream` which produces a sequence of messages that follows the SSE wire
protocol and yields them line by line.

The index `/` page is there so that you can head over to it in your browser and paste the
client-side code.

You can run this server with `uvicorn` via the following command:

```sh
uvicorn server:app --port 5000 --reload
```

This will expose the server to the localhost's port `5000`. Now you can head over to your
browser, go to the `localhost:5000` URL and paste this following snippet to the dev console
to catch the streamed data from the client side:

```js
// client.js

// Connect to the event stream server.
const eventSource = new EventSource("http://localhost:5000/stream");

// Log something when the client connects to the server.
eventSource.onconnect = (event) => console.log("connected to the server");

// Log a message while closing the connection.
eventSource.onclose = (event) => console.log("closing connection");

// Log an error message on account of an error.
eventSource.onerror = (event) => console.log("an error occured");

// This is how you can attach an event listener to a custom event.
eventSource.addEventListener("start", (event) => {
  console.log(`start event: ${event.data}`);
});

// Log the default message.
eventSource.onmessage = (event) => {
  console.log(`Default event: ${event.data}`);

  // Don't reconnect when the server closes the connection.
  if (event.data === "closing connection") eventSource.close();
};
```

Notice, how the client API is quite similar to the WebSocket API but simpler. Once you've
pasted the code snippet to the browser console, you'll be able to see the streamed data from
the server that looks like this:

```txt
start event: streaming started
Default event: message 1
Default event: message 2
start event: streaming started
Default event: message 1
Default event: message 2
start event: streaming started
Default event: message 1
Default event: message 2
Default event: closing connection
```

## A more practical example

This section will demonstrate the scenario that was mentioned at the beginning of this post
where loading a particular page in your browser will trigger a long-running asynchronous
celery[^6] task in the background. While the task runs, the server will communicate the
progress with the client.

Once the task is finished, the server will send a specific message to the client and it'll
update the DOM to let the user know that the task has been finished. The workflow only
requires unidirectional communication and SSE is a perfect candidate for this situation.

To test it out, you'll need to install a few dependencies. You can `pip install` them as
such:

```sh
pip install 'celery[redis]' jinja2 starlette uvicorn
```

You'll also need to set up a Redis server that Celery will use for broker communication. If
you have Docker installed in your system, you can run the following command to start a Redis
server:

```sh
docker run --name dev-redis -d -h localhost -p 6379:6379 redis:alpine
```

The application will live in a directory called `sse` with the following structure:

```txt
sse
├── __init__.py
├── index.html # Client side SSE code.
└── views.py # Server side SSE code.
```

The `view.py` contains the server implementation that looks like this:

```py
from __future__ import annotations

import json
import logging
import time
from typing import TYPE_CHECKING, AsyncGenerator

from celery import Celery
from celery.result import AsyncResult
from starlette.applications import Starlette
from starlette.responses import StreamingResponse
from starlette.routing import Route
from starlette.templating import Jinja2Templates

if TYPE_CHECKING:
    from starlette.requests import Request
    from starlette.responses import Response

logging.basicConfig(level=logging.INFO)


templates = Jinja2Templates(directory="./")

celery_app = Celery("tasks", backend="redis://", broker="redis://")


@celery_app.task()
def background() -> str:
    time.sleep(5)
    return "Hello from background task..."


async def index(request: Request) -> Response:
    task_id = background.apply_async(queue="default")
    logging.info("Task id: %s", task_id)
    response = templates.TemplateResponse("index.html", {"request": request})
    response.set_cookie("task_id", task_id)
    return response


async def task_status(request: Request) -> StreamingResponse:
    task_id = request.path_params["task_id"]

    async def stream() -> AsyncGenerator[str, None]:
        task = AsyncResult(task_id, app=celery_app)
        logging.info("Task state: %s", task.state)
        attempt = 0  # Give up and close the connection after 10 attempts.
        while True:
            data = {
                "state": task.state,
                "result": task.result,
            }
            logging.info("Server sending data: %s", data)

            # Send a stringified JSON SSE message.
            yield f"data: {json.dumps(data)}\n\n"
            attempt += 1

            # Close the connection when the task has successfully finished.
            if data.get("state") == "SUCCESS":
                break

            # Give up after 10 attempts to avoid dangling connections.
            if attempt > 10:
                data["state"] = "UNFINISHED"
                data["result"] = "Task is taking too long to complete."
                yield f"data: {json.dumps(data)}\n\n"
                break

            # Sleep for a second so that we're not flooding the client with messages.
            time.sleep(1)

    response = StreamingResponse(
        stream(),
        headers={
            "Content-Type": "text/event-stream",
            "Access-Control-Allow-Origin": "*",
            "Cache-Control": "no-cache",
        },
    )
    return response


routes = [
    Route("/index", endpoint=index),
    Route("/task_status/{task_id}", endpoint=task_status),
]

# Add session middleware
app = Starlette(debug=True, routes=routes)
```

Here, first, we're setting up celery and connecting it to the local Redis instance. Next up,
the `background` function simulates some async work where it just waits for a while and
returns a message. The `index` view calls the asynchronous background task and sets the id
of the task as a session cookie with `response.set_cookie("task_id", task_id)`. The frontend
JavaScript will look for this `task_id` cookie to identify a running background task.

Then we expose a `task_status` endpoint that takes in the value of a `task_id` and streams
the status of the running task to the frontend as SSE messages. To avoid dangling
connections, we stream the task status for 10 seconds before giving up.

Now on the client side, the `index.html` looks like this:

```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <style>
      html, body {
      height: 100%;
      margin: 0;
      }
      .centered {
      height: 100%;
      display: flex;
      justify-content: center;
      align-items: center;
      }
    </style>
    <link rel="shortcut icon" href="#" />
  </head>
  <body class="centered">
    <div>
      <h1>SSE Demo</h1>
      <h2>Message</h2>
      <p id="message">Waiting for server-sent message...</p>
    </div>
  </body>
  <script>
    // Get result from server sent events.
    async function waitForResult() {
      console.log("Waiting for task result...");

      // Collect the task_id from the session cookie.
      const taskId = await waitForTaskIdCookie();

      // Connect to the task_status streaming endpoing.
      const eventSource = new EventSource(`/task_status/${taskId}/`);

      // This will get triggered when the server sends an update on
      // the task status
      eventSource.onmessage = function(event) {
        console.log("Task result:", event.data);

        // Parser the JSONified event message.
        const data = JSON.parse(event.data);

        // Log the message to the console.
        const message = `Server sent: ${data.result}`;

        if(data.state === "SUCCESS") {
          document.getElementById("message").innerHTML = message;
          eventSource.close(); // Close the connection from the client side.

        } else if(data.state === "UNFINISHED") {
          document.getElementById("message").innerHTML = message;
          eventSource.close(); // Close the connection from the client side.
        }
      };

      eventSource.onerror = function(event) {
        console.log("Error:", event);
      };
      eventSource.onopen = function(event) {
        console.log("Connection opened:", event);
      };
      eventSource.onclose = function(event) {
        console.log("Connection closed:", event);
      };
    }
    // Wait for the task_id cookie to be set from the server.
    async function waitForTaskIdCookie() {
      while(true) {
        const taskId = getCookie("task_id");
        if(taskId) {
          console.log("Found task_id cookie:", taskId);
          return taskId;
        }

        // Wait for 300ms between each iteration so that we don't overwhelm
        // the client.
        console.log("Waiting for task_id cookie...");
        await sleep(300);
      }
    }
    // Get cookie value by name.
    function getCookie(cookieName) {
      const cookieString = document.cookie;
      if(!cookieString) {
        return null;
      }
      const cookies = cookieString.split("; ");
      for(const cookie of cookies) {
        if(cookie.startsWith(`${cookieName}=`)) {
          return cookie.split("=")[1];
        }
      }
      return null;
    }
    // Sleep for given milliseconds.
    function sleep(ms) {
      return new Promise((resolve) => setTimeout(resolve, ms));
    }
    // Call the function when the page has finished loading.
    window.onload = function() {
      waitForResult();
    };
  </script>
</html>
```

When the index page is loaded, the server starts a background task and sets the
`task_id=<task_id>` session cookie. The HTML above then defines a paragraph element to show
the message streamed from the server:

```html
<body class="centered">
  <div>
    <h1>SSE Demo</h1>
    <h2>Message</h2>
    <p id="message">Waiting for server-sent message...</p>
  </div>
</body>
```

The JavaScript code defines a function named `waitForResult()` that listens for updates on
the status of a long-running task that is being executed on the server. The function first
waits for the `task_id` to be set in a cookie by calling `waitForTaskIdCookie()`. Once the
`task_id` is obtained, the function creates a new `EventSource` object that connects to the
streaming endpoint on the server using the ID to get updates on the status of the task.

The `EventSource` object is set up with four event listeners: `onmessage`, `onerror`,
`onopen`, and `onclose`. The `onmessage` listener is triggered when the server sends an
update on the task status. The listener first logs the updated task status and then checks
if the state of the task is `SUCCESS` or `UNFINISHED`. In either case, the client fetches
the message element on the DOM and updates it with the result of the background task
streamed by the server.

The client-side SSE API will automatically keep reconnecting if the connection fails for
some reason. This is handy since you don't have to write any additional logic to make the
connection more robust. However, you do need to be mindful about closing the connection from
the client side once you've received the final task status. The `onmessage` event listener
explicitly closes the connection with `eventSource.close()` once the final message about a
specific task has reached the client from the server.

The `onerror` listener handles errors that occur with the connection. The `onopen` callback
is called when the connection is successfully opened, and `onclose` gets called when the
connection is closed.

The `waitForTaskIdCookie()` function that is called by the entrypoint waits for the
`task_id` to be set in a cookie by repeatedly calling `getCookie()` until the ID is
obtained. The function waits for `300ms` between each iteration so that it doesn't overwhelm
the client.

The `getCookie()` function is a utility function that returns the value of a cookie given
its name.

Finally, the code sets the window.onload event listener to call the `waitForResult()`
function when the page has finished loading.

Now, go to the `sse` directory and start the server with the following command:

```sh
uvicorn views:app --port 5000 --reload
```

On another terminal, start the celery workers:

```sh
celery -A views.celery_app worker -l info -Q default -c 1
```

Finally, head over to your browser and go to `http://localhost:5000/index` page and see that
the server has triggered a background job. Once the job finishes after 5 seconds, the client
shows a message:

<video
  src="https://user-images.githubusercontent.com/
30027932/229604497-0a0b058f-32dd-4219-a68f-9cd35b250334.mov"
  controls="controls"
  style="max-width: 730px"
  alt="server sent events demo"> </video>

Notice, how the server pushes the result of the task automatically once it finishes.

## Limitations

While SSE-driven pages are much easier to bootstrap than their WebSocket counterparts—apart
from only supporting unidirectional communication, they suffer from a few other limitations:

- SSE is limited to sending text data only. If an application needs to send binary data, it
  must encode the data as text before sending it over SSE.
- SSE connections are subject to the same connection limitations as HTTP connections. In
  some cases, a large number of SSE connections can overload the server, leading to
  performance issues. However, this can be mitigated by taking advantage of connection
  multiplexing in HTTP/2.

[^1]: [WebSocket](https://en.wikipedia.org/wiki/WebSocket)

[^2]: [channels](https://channels.readthedocs.io/en/stable/)

[^3]: [SSE](https://en.wikipedia.org/wiki/Server-sent_events)

[^4]: [SSE vs WebSockets](https://germano.dev/sse-websockets/)

[^5]: [starlette](https://www.starlette.io/)

[^6]: [celery](https://docs.celeryq.dev/en/stable/getting-started/introduction.html)

[^7]:
    [Using server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)
    [^7]
