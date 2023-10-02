---
title: Signal handling in a multithreaded socket server
date: 2023-02-26
tags:
    - Python
    - Networking
---

While working on a multithreaded socket server in an embedded environment, I realized that
the default behavior of Python's `socketserver.ThreadingTCPServer` requires some extra work
if you want to shut down the server gracefully in the presence of an interruption signal.
The intended behavior here is that whenever any of `SIGHUP`, `SIGINT`, `SIGTERM`, or
`SIGQUIT` signals are sent to the server, it should:

-   Acknowledge the signal and log a message to the output console of the server.
-   Notify all the connected clients that the server is going offline.
-   Give the clients enough time (specified by a timeout parameter) to close the requests.
-   Close all the client requests and then shut down the server after the timeout exceeds.

Here's a quick implementation of a multithreaded echo server and see what happens when you
send `SIGINT` to shut down the server:

```python
# server.py

from __future__ import annotations

import logging
import socketserver

logging.basicConfig(level=logging.INFO)


class RequestHandler(socketserver.BaseRequestHandler):
    """Handler that handles an incoming client request."""

    def handle(self) -> None:
        conn = self.request
        while True:
            data = conn.recv(1024)

            if not data:
                break

            logging.info(f"recv: {data!r}")
            conn.sendall(data)


if __name__ == "__main__":
    with socketserver.ThreadingTCPServer(
        ("localhost", 9999), RequestHandler
    ) as server:
        server.serve_forever()
```

Here's the client code:

```python
# client.py

import logging
import socket
import time

logging.basicConfig(level=logging.INFO)

HOST = "localhost"  # The server's hostname or IP address.
PORT = 9999  # The port used by the server.

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.connect((HOST, PORT))
    while True:
        time.sleep(1)
        s.sendall(b"hello world")

        data = s.recv(1024)

        if not data:
            break

        logging.info(f"Received {data!r}")
```

Here, the server logs and echoes back whatever the client sends and the client just sends
the string `hello world` continuously in a `while` loop. This is pretty much the canonical
multithreaded server-client example that's found in the `socketserver` docs. In the client
code, the only thing that's a little different is that within the `while` loop, a
`time.sleep(1)` function was added to simulate the client performing some processing tasks.
Also, without the `sleep`, the server would've flooded the stdout with the client message
logs and made the demonstration difficult.

Let's run the server and the client in two separate processes and then send a `SIGINT`
signal to the server by clicking `Ctrl + C` on the server console:

![multi-threaded socket server][image_1]

At first, the server just ignores the signal, and clicking `Ctrl + C` multiple times crashes
the server down with this nasty traceback (full traceback trimmed for brevity):

```txt
Traceback (most recent call last):
  File "/Users/rednafi/Canvas/personal/reflections/server.py", line 137,
  in <module>
    server.serve_forever()
  File "/Users/rednafi/.asdf/installs/Python/3.11.1/lib/python3.11/socketserver.py",
  line 233, in serve_forever
    ready = selector.select(poll_interval)
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/Users/rednafi/.asdf/installs/Python/3.11.1/lib/python3.11/selectors.py",
  line 415, in select
    fd_event_list = self._selector.poll(timeout)
                    ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
KeyboardInterrupt
...
```

## Multithreaded socket server with graceful shutdown

What we want here is that whenever the server gets `SIGHUP`, `SIGINT`, `SIGTERM`, or
`SIGQUIT`, it should notify the clients and gracefully shut itself down. I played around
with the `socketserver.ThreadingTCPServer` API for a while to come up with a solution that
worked nicely for my use case. Here's the full server implementation:

```python
# server.py

from __future__ import annotations

import logging
import os
import signal
import socket
import socketserver
import threading
import time
from types import FrameType
from typing import Callable

logging.basicConfig(level=logging.INFO)


class RequestHandler(socketserver.BaseRequestHandler):
    server: SocketServer

    def notify_clients_when_server_is_interrupted(self) -> None:
        logging.info("Server interrupted, notifying all clients...")
        self.request.sendall(b"SHUTDOWN")
        self.request.sendall(b"")

    def setup(self) -> None:
        # Prevent new connections from being accepted when the server is
        # shutting down.
        if self.server._is_interrupted:
            self.notify_clients_when_server_is_interrupted()

    def handle(self) -> None:
        conn = self.request
        while True:
            data = conn.recv(1024)

            if self.server._is_interrupted:
                self.notify_clients_when_server_is_interrupted()
                break

            if not data:
                break

            logging.info(f"recv: {data!r}")
            conn.sendall(data)


class SocketServer(socketserver.ThreadingTCPServer):
    reuse_address = True
    daemon_threads = True
    block_on_close = False
    _is_interrupted = False

    def server_activate(self) -> None:
        logging.info(
            "PID:%s. Server started on %s:%s",
            os.getpid(),
            *self.server_address,
        )
        super().server_activate()

    def get_request(self) -> tuple[socket.socket, str]:
        conn, addr = super().get_request()
        logging.info("Starting connection from %s:%s", *addr)
        return conn, addr

    def shutdown_request(
        self, request: socket.socket | tuple[bytes, socket.socket]
    ) -> None:
        if isinstance(request, socket.socket):
            logging.info(
                "Closing connection from %s:%s",
                *request.getpeername(),
            )
        super().shutdown_request(request)

    def shutdown(self) -> None:
        logging.info("Server is shutting down...")
        super().shutdown()

    def handle_signal(
        self, timeout: int
    ) -> Callable[[int, FrameType | None], None]:
        """A simple signal handler factory that takes in some additional
        parameters and passes them to the actual signal handler. Defines
        and returns the final handler.
        """

        def handler(signum: int, _: FrameType | None) -> None:
            deadline = time.monotonic() + timeout
            signame = signal.Signals(signum).name
            self._is_interrupted = True

            while (current_time := time.monotonic()) < deadline:
                delta = int(deadline - current_time) + 1
                logging.info(
                    "%s received, closing server in %s seconds..."
                    % (signame, delta)
                )
                time.sleep(1)

            self.server_close()
            self.shutdown()

        return handler


if __name__ == "__main__":
    with SocketServer(("localhost", 9999), RequestHandler) as server:
        for sig in (
            signal.SIGHUP,
            signal.SIGINT,
            signal.SIGTERM,
            signal.SIGQUIT,
        ):
            signal.signal(sig, server.handle_signal(timeout=5))

        t = threading.Thread(target=server.serve_forever)

        t.start()
        t.join()
```

Apart from a few extra methods that perform logging and signal handling, the overall
structure of this server is similar to the vanilla multithreaded server from the previous
section. In the `RequestHandler`, we have defined a custom
`notify_clients_when_server_is_interrupted` method that notifies all clients whenever the
server receives an interruption signal. This is a custom method that's not defined in the
`BaseRequestHandler` class. The notify method logs the status of the interruption signal and
then sends a `SHUTDOWN` message to the clients. Afterward, it closes the client connection.

The `setup` method extends the eponymous method from the `BaseRequestHandler` class and
calls the `notify_clients_when_server_is_interrupted` method. This ensures that whenever the
server is shutting down, it refuses any new client connections. Within the handle method, in
the data processing `while` loop, we check the value of the `_is_interrupted` flag on the
server instance. If the value is `True`, we call the notify method. The value of this flag
is managed by the `SocketServer` class. Calling the notify method from within the data
processing loop will notify all currently connected clients.

Next, we define a new server class called `SocketServer` that inherits from the
`socketserver.ThreadingTCPServer` class. The `reuse_address`, `daemon_threads`, and
`block_on_close` class variables override the default values inherited from the base
`ThreadingTCPServer` class. Here are the explanations for each:

1. `reuse_address`: This variable determines whether the server can reuse a socket that's
   still in the TIME_WAIT[^1] state after a previous connection has been closed. If this
   variable is set to `True`, the server can reuse the socket. Otherwise, the socket will be
   unavailable for a short period of time after it's closed.

2. `daemon_threads`: This variable determines whether the server's worker threads should be
   daemon threads. Daemon threads are threads that run in the background and don't prevent
   the Python interpreter from exiting when they are still running. If this variable is set
   to `True`, the server's worker threads will be daemon threads. I found that daemon
   threads work better when I need to shut down the server that's connected to multiple
   long-running clients.

3. `block_on_close`: This variable determines whether the server should block until all
   client connections have been closed before shutting down. If this variable is set to
   `True`, the server will block until all client connections have been closed. Otherwise,
   the server will shut down immediately, even if there are still active client connections.
   We want to set it to `False` since we'll handle the graceful shutdown in a custom signal
   handler method on the server class.

Going forward, the `SocketServer` class overrides the `server_activate`, `get_request`,
`shutdown_request`, and `shutdown` methods from the base class. All of them just log a few
key pieces of information to the console and calls the methods from the parent class
verbatim. The interesting part happens in the custom `handle_signal` method. When an
interruption signal is sent to the server, the `handle_signal` method is activated. The
method takes an integer parameter `timeout` which specifies how many seconds the server
should wait before shutting down after receiving the signal.

The method then returns the actual signal handler function that takes two parameters: an
integer `signum` representing the signal number and a `FrameType` object which represents
the current stack frame. The function is responsible for handling the signal by making the
server wait for `timeout` seconds before shutting it down gracefully.

First, the method sets a variable `_is_interrupted` to `True` to indicate that the server
has received an interruption signal. Then, the method enters a `while` loop that continues
until the current time exceeds the `deadline` time, which is calculated by adding the
`timeout` to the current monotonic time. During each iteration of the `while` loop, the
method logs a message to the console to indicate that the signal has been received and the
server will be closed in a certain number of seconds. The `delta` variable is calculated as
the difference between the deadline and the current monotonic time, plus `1`. This ensures
that the logging message displays an accurate countdown of the remaining time until the
server shuts down.

Once the `deadline` exceeds and the `while` loop completes, the method calls
`server_close()` and `shutdown()` methods of the server to close the requests and shut
itself down gracefully. The `server_close()` method closes the listening socket and stops
accepting new client connections, while the `shutdown()` method stops all active client
connections and waits for them to finish processing their current requests. However, in this
case, since we are giving the clients enough time to close the connections and using daemon
threads to process the requests, calling `shutdown()` will immediately close all the client
requests and bring down the server.

Finally, in the `__main__` section, we instantiate the `SocketServer` class and register the
`RequestHandler`. Then we register the signal handler with a timeout of `5` seconds. This
means, upon receiving the interruption signal, the server will wait `5` seconds before
shutting itself down. Notice, how we're running the `server.serve_forever` method in a new
thread. That's because our custom signal handler explicitly calls the `shutdown` of the
server instance and the `shutdown` method can only be called when the `serve_forever` loop
is running in a different thread. From the shutdown[^2] documentation:

> Tell the serve_forever() loop to stop and wait until it does. shutdown() must be called
> while serve_forever() is running in a different thread otherwise it will deadlock.

Now that the server is coded to shut down gracefully, we also expect the client to behave
properly. That means, whenever the client receives the `SHUTDOWN` message, it should
immediately close the connection. Here's a slightly modified version of the vanilla socket
client code that we've seen before:

```python
# client.py

import logging
import socket
import time

logging.basicConfig(level=logging.INFO)

HOST = "localhost"  # The server's hostname or IP address.
PORT = 9999  # The port used by the server.

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.connect((HOST, PORT))
    while True:
        time.sleep(1)
        s.sendall(b"hello world")

        data = s.recv(1024)

        if data == b"SHUTDOWN":
            logging.info("Closing connection...")
            break

        if not data:
            break

        logging.info(f"Received {data!r}")
```

The only difference between this and the previous client is that this client will break out
of the process loop when it encounters the `SHUTDOWN` message from the server. Now to see
the whole thing in action, you can fire up the server and the client from two different
terminals. Once both the server and client are running, try sending a `SIGINT` or any of the
three other handled signals. You see that the server acknowledges the interruption signal,
gives the clients enough time to disconnect, then shut itself down in a graceful manner:

![error handling in multi-threaded socket server][image_2]

[^1]: [TIME_WAIT](https://totozhang.github.io/2016-01-31-tcp-timewait-status/)
[^2]:
    [shutdown](https://docs.python.org/3/library/socketserver.html#socketserver.BaseServer.shutdown)

[^3]: [socketserver](https://docs.python.org/3/library/socketserver.html) [^3]

[image_1]:
    https://user-images.githubusercontent.com/30027932/221752665-a6a1584d-e7bf-48b4-93a4-7679bc915682.png
[image_2]:
    https://user-images.githubusercontent.com/30027932/222344540-ace10d97-81f5-47d4-bf83-4ec505a72f74.png
