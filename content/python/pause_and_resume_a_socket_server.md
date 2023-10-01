---
title: Pausing and resuming a socket server in Python
date: 2023-02-05
tags:
    - Python
    - Networking
---

I needed to write a socket server in Python that would allow me to intermittently pause the
server loop for a while, run something else, then get back to the previous request-handling
phase; repeating this iteration until the heat death of the universe. Initially, I opted for
the low-level `socket` module to write something quick and dirty. However, the
implementation got hairy pretty quickly. While the `socket` module gives you plenty of
control over how you can tune the server's behavior, writing a server with robust signal and
error handling can be quite a bit of boilerplate work.

Thankfully, I found out that Python is already shipped with a higher level library named
`socketserver`[^1] that uses the `socket` module underneath but gives you more tractable
hooks to latch onto and build fairly robust servers where the low-level details are handled
for you. Not only that, `socketserver` makes it easy to write a sever that can concurrently
handle multiple clients either by spinning child threads or forking child processes.

While all this sounds good and dandy, my primary objective was to be able to write a server
that can pause serving the clients every now and then, do some work and then come back to
the previous work. Here's how I did it with a multi-threaded socket server:

```python
from __future__ import annotations

import logging
import socket
import socketserver
import time

logging.basicConfig(level=logging.INFO)


class RequestHandler(socketserver.BaseRequestHandler):
    def setup(self) -> None:
        logging.info("Start request.")

    def handle(self) -> None:
        conn = self.request
        while True:
            data = conn.recv(1024)

            if not data:
                break

            logging.info(f"recv: {data!r}")
            conn.sendall(data)

    def finish(self) -> None:
        logging.info("Finish request.")


class ThreadingTCPServer(socketserver.ThreadingTCPServer):
    _timeout = 5  # seconds
    _start_time = time.monotonic()

    def server_activate(self) -> None:
        logging.info("Server started on %s:%s", *self.server_address)
        super().server_activate()

    def get_request(self) -> tuple[socket.socket, str]:
        conn, addr = super().get_request()
        logging.info("Connection from %s:%s", *addr)
        return conn, addr

    def service_actions(self) -> None:
        if time.monotonic() - self._start_time > self._timeout:
            logging.info("Server paused, something else is running...")
            self._start_time = time.monotonic()


if __name__ == "__main__":
    with ThreadingTCPServer(("localhost", 9999), RequestHandler) as server:
        server.serve_forever()
```

This is a simple echo server that receives client connections and reflects back the data
sent by the clients. The server can handle multiple client connections simultaneously using
the `ThreadingTCPServer` class. This class is derived from the
`socketserver.ThreadingTCPServer` class and is responsible for implementing the server's
main loop, which listens for incoming client connections and creates a separate thread for
each one to handle the incoming request.

The `RequestHandler` class is used to handle each incoming request. This class is derived
from the `socketserver.BaseRequestHandler` class and is responsible for handling the
connection between a client and the server. It implements the `setup`, `handle`, and
`finish` methods to perform any necessary initialization work, handle the incoming data, and
clean up after the request has been processed. In the `setup` and `finish` methods, we're
only printing some message to indicate that these methods are called before and after the
`handle` method respectively. In the `handle` method, we're collecting the data sent by the
clients and echoing them back. Here, inside the `while` loop, `conn.recv` is a blocking
method and will keep reading from the clients indefinitely. We need the server to break out
from this, do something else, and then get back to it gracefully.

In the `__main__` section of the code snippet, a `ThreadingTCPServer` object is created and
the server is started using the `serve_forever` method. This method will continuously run
the server loop, listen for incoming connections and create a separate thread for each one
to handle the request.

The `ThreadingTCPServer` class implements `server_activate` and `get_request` methods. These
two methods are already implemented in the base and we're just calling the methods from
there with some additonal logging. Here, `server_activate` prints out the server's IP
address and port. Similarly, the `get_request` method calls the eponymous method from the
superclass and logs the IP and the port of the incoming clients.

The server also implements a `service_actions` method that is called by the server loop.
This is where we're periodically pausing the server and performing some blocking actions. In
this case, the `service_actions` method checks the current time and compares it to the start
time of the server. If the difference is greater than the specified `timeout`, the server is
paused and a message is printed to the console indicating that something else is running.
Then after one iteration, the start time is updated so that the server gets paused again
after the `timeout` period.

To test the server out, here's a simple client that sends some data to the server:

```python
# client.py

import socket
import time
import logging

logging.basicConfig(level=logging.INFO)

HOST = "localhost"  # The server's hostname or IP address.
PORT = 9999  # The port used by the server.

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.connect((HOST, PORT))
    while True:
        time.sleep(1)
        s.sendall(b"hello world")
        data = s.recv(1024)
        logging.info(f"Received {data!r}")
```

This client connects to the server via port `9999` and sends the `b'hello world'` byte
string. The server will capture and echo it back to the client which the client will print
as `Received ...`. You can run the server in one console with `python server.py` and the
client in another one with the `python client.py` command.

![server-client][image_1]

You'll see that the server will pause every 5 seconds, do something else in a blocking
manner and then come back to handle the client requests. If you attach a second client from
another console, you'll see that the server can also handle that while retaining the
expected behavior. The server will pause even if there's no client sending requests to the
server. You can test that behavior by detaching all the clients from the server.

Now, we could also make the work in the `serving_actions` non-blocking by spinning a new
thread or process and doing the work there. However, for the task that I was tackling,
simply running the function in a blocking manner was enough.


[^1]: [socketserver](https://docs.python.org/3/library/socketserver.html)


[image_1]: https://user-images.githubusercontent.com/30027932/221395153-5044d50e-e12d-45f4-b816-5416f69d0308.png
