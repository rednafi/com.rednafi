---
title: Tinkering with Unix domain sockets
date: 2023-03-11
slug: tinkering-with-unix-domain-socket
aliases:
    - /misc/tinkering_with_unix_domain_socket/
tags:
    - Python
    - Shell
    - Networking
---

I've always had a vague idea about what Unix domain sockets are from my experience working
with Docker for the past couple of years. However, lately, I'm spending more time in
embedded edge environments and had to explore Unix domain sockets in a bit more detail. This
is a rough documentation of what I've explored to gain some insights.

## The dry definition

Unix domain sockets (UDS) are similar to TCP sockets in a way that they allow two processes
to communicate with each other, but there are some core differences. While TCP sockets are
used for communication over a network, Unix domain sockets are used for communication
between processes running on the same computer.

A Unix domain socket is a way for programs to exchange data in a fast and efficient way
without having to worry about the overhead of network protocols like TCP/IP or UDP. It works
by creating a special file on the file system called a socket, which acts as a bidirectional
data channel between the processes. The processes can send and receive data through the
socket just like they would with a network socket. Also, just like TCP/UDP sockets, Unix
domain sockets can also be either stream-based (TCP equivalent) or datagram-based (UDP
equivalent).

Unix domain sockets are commonly used in server-client applications, such as web servers,
databases, and email servers, where they provide a secure and efficient way for processes on
the same machine to communicate with each other. They're also used in many other types of
programs where different parts of the program need to work together or share data. Another
cool thing about them is that you can control access to your server just by tuning the
permission of the socket file on the system.

## Prerequisites

I'm running these experiments on an M-series Macbook pro. However, any Unix-y environment
will work as long as you can run the following tools:

- `socat`: To create the socket servers and clients.
- `curl`: To make HTTP requests to a supported socket server.
- `jq`: To pretty print JSON payloads.
- `lsof`: To display currently listening socket server processes.

## Inspecting Unix domain sockets in your system

Most likely, there are currently multiple processes listening on different sockets in your
system. You can explore them using `lsof` with the following command:

```sh
sudo lsof -U
```

This will return a list of all Unix domain socket files and the server process PIDs that are
currently listening on them:

```txt
COMMAND   PID  USER FD  TYPE             DEVICE SIZE/OFF NODE NAME
launchd   1  root   3u  unix 0x25269ff9edd05165      0t0      /private//var/run/syslog
launchd   1  root   4u  unix 0x25269ff9edd0522d      0t0      ->0x25269ff9edd05165
launchd   1  root   6u  unix 0x25269ff9edd052f5      0t0      /private/var/run/cupsd
launchd   1  root   7u  unix 0x25269ff9edd053bd      0t0      /var/rpc/ncalrpc/NETLOGON
launchd   1  root   8u  unix 0x25269ff9edd05485      0t0      /var/run/vpncontrol.sock
launchd   1  root   9u  unix 0x25269ff9edd0554d      0t0      /var/run/portmap.socket
...
```

You can also filter out the socket files by their process names. Docker processes listen on
a few socket files:

```sh
sudo lsof -U  -a -c 'com.docker'
```

This will return:

```txt
COMMAND     PID    USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
com.docke 15451 rednafi   10u  unix 0x25269ff9edcfd55d      0t0    vpnkit-bridge-fd.sock
com.docke 15451 rednafi   11u  unix 0x25269ff9edcfd625      0t0    vpnkit-bridge.sock
com.docke 15451 rednafi   12u  unix 0x25269ff9edcfd6ed      0t0    vpnkit.port.sock
com.docke 15451 rednafi   13u  unix 0x25269ff9edcfd7b5      0t0    vpnkit.data.sock
com.docke 15451 rednafi   14u  unix 0x25269ff9edcfd87d      0t0    httpproxy.sock
com.docke 15451 rednafi   15u  unix 0x25269ff9edcfd3cd      0t0    backend.sock
...
```

## Creating a Unix domain socket

Running the following command on your terminal will create a stream-based Unix domain
socket:

```sh
socat unix-listen:/tmp/stream.sock,fork STDOUT
```

This process listens on the `/tmp/stream.sock` and prints the incoming data to the `stdout`.
The `fork` portion on the command ensures that multiple clients can be connected to the
server process and they'll be served by forking child processes.

From another console, you can try to send data to the socket file as a client:

```sh
echo "hello world" | socat - unix-connect:/tmp/stream.sock
```

Running this command will send the `hello world` string to the `/tmp/stream.sock` file and
the server process will print it on the standard output stream.

Similarly, you can also create a datagram-based socket server with `socat` like this:

```sh
socat unix-recvfrom:/tmp/datagram.sock,fork STDOUT
```

Now send data to the server with this:

```sh
echo "hello world" | socat - unix-sendto:/tmp/datagram.sock
```

## Connecting to Docker engine via a Unix domain socket

By default, Docker runs through a non-networked UNIX socket. It can also optionally
communicate using SSH or a TLS (HTTPS) socket. On MacOS, the socket file can be found in
`~/.docker/run/docker.sock`. We can make HTTP requests against the listening socket server
and use Docker engine's RESTful API suite.

**Checking the engine's version number:** The following command uses `curl` to spawn a
client process and send a request against the Docker engine running in my local system.

```sh
curl --unix-socket  ~/.docker/run/docker.sock http://localhost/version | jq
```

This returns (truncated output for readability):

```json
{
  "Platform":{
    "Name":"Docker Desktop 4.17.0 (99724)"
  },
  "Components":[
    "..."
  ],
  "Version":"20.10.23",
  "ApiVersion":"1.41",
  "MinAPIVersion":"1.12",
  "GitCommit":"6051f14",
  "GoVersion":"go1.18.10",
  "Os":"linux",
  "Arch":"arm64",
  "KernelVersion":"5.15.49-linuxkit",
  "BuildTime":"2023-01-19T17:31:28.000000000+00:00"
}
```

**Listing the containers:** This command lists all the running containers on my machine.

```sh
curl --unix-socket \
    ~/.docker/run/docker.sock http://localhost/containers/json | jq
```

**Listing the images:**

```sh
curl --unix-socket \
    ~/.docker/run/docker.sock http://localhost/images/json | jq
```

**Downloading a container:** This allows you to programmatically download the `hello-world`
image from Dockerhub:

```sh
curl -sX POST \
    -H 'Content-Type: application/json' \
    --unix-socket ~/.docker/run/docker.sock \
    'http://localhost/images/create?fromImage=hello-world:latest'
```

**Listening for docker events:** This API call lets you listen for all incoming events from
the docker engine. You can run the following command on one terminal and send events from
another:

```sh
curl --no-buffer --unix-socket \
    ~/.docker/run/docker.sock http://localhost/events | jq
```

Here, the `--no-buffer` flag is necessary for instructing `curl` to send the output events
to the input stream of `jq` without doing any buffering. This allows `jq` to pretty-print
the outputs in real-time. Now from another console if you run the following command, you'll
see events pouring into the console that's listening for them:

```sh
docker run hello-world
```

The complete list of APIs can be found here[^1].

## Writing a Unix domain socket server in Python

You can quickly write a simple server that allows clients to connect to it via Unix domain
sockets. If the clients exist on the same machine then, a UDS server has the advantage of
having lower overhead than its networked TCP counterpart.

The following server uses Python's `socketserver` module to create a stream-based echo
server:

```py
# server.py

from __future__ import annotations

import logging
import socketserver
from pathlib import Path

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


class Server(socketserver.ThreadingUnixStreamServer):
    def server_activate(self) -> None:
        logging.info("Server started on %s", self.server_address)
        super().server_activate()


if __name__ == "__main__":
    # Remove the socket file if it already exists.
    # UDS doesn't let you reuse the socket file.
    socket_path = Path("/tmp/stream.sock")
    if socket_path.exists():
        socket_path.unlink()

    with Server(str(socket_path), RequestHandler) as server:
        server.serve_forever()
```

Here, `socketserver.ThreadingUnixStreamServer` enables us to create a server that allows
multiple clients to be connected to it via Unix domain sockets. The server spins up a new
thread to serve each new client and does bi-directional communication via UDS. The client
code is quite similar to a TCP client:

```py
# client.py

import socket
import time
import logging

logging.basicConfig(level=logging.INFO)

ADDRESS = "/tmp/stream.sock"

with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as s:
    s.connect(ADDRESS)

    while True:
        time.sleep(1)
        s.sendall(b"hello world")
        data = s.recv(1024)
        logging.info(f"Received {data!r}")
```

The client connects to the server through the `/tmp/stream.sock` socket and sends a static
`hello world` string to it. The server then sends that data back and the client sends it to
the stdout stream.

Running the server and client as two separate processes will yield the following output:

![echo client server][image_1]

## Exposing an HTTP application via a Unix domain socket

Webservers usually allow you to expose HTTP applications via Unix domain sockets. In Python,
the uvicorn[^2] ASGI server lets you do this quite easily. This can come as handy whenever
you need to spin up a local server and all the clients are running on the same machine or
you're running your server behind a proxy. Here's an example of a simple webserver built
with starlette[^3] and served with uvicorn.

```py
# server.py (http server)

from __future__ import annotations

from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import HTMLResponse
from starlette.routing import Route


async def index(request: Request) -> HTMLResponse:
    return HTMLResponse(
        """<h1>Hello, world!</h1>
    <p>This is the index page.</p>"""
    )


app = Starlette(
    debug=True,
    routes=[
        Route("/index", index),
    ],
)
```

You can expose this server through a UDS like this:

```sh
uvicorn --uds /tmp/stream.sock server:app
```

Calling this API with `curl` from another console will return the HTML content in the
response:

```sh
curl --unix-socket /tmp/stream.sock http://localhost/index
```

```txt
<h1>Hello, world!</h1>
    <p>This is the index page.</p>
```

If you want to access this server from a browser, you'll need to make sure that your reverse
proxy server (Nginx / Apache / Caddy) is configured to relay the incoming request from the
network to the UDS server. For a quick and dirty approach, you can use `socat` to proxy the
request from a `HOST:PORT` pair to the UDS server like this:

```sh
uvicorn --uds /tmp/stream.sock server:app \
    & socat tcp-listen:9999,fork unix-connect:/tmp/stream.sock &
```

The `uvicorn` command spins up a webserver in the background as before and listens on the
socket file `/tmp/stream.sock`. Then we're using `socat` to create a forking TCP server that
handles the incoming HTTP requests from the network and relays them to the webserver via
UDS. It also relays the server's responses back to the client—doing the work of a reverse
proxy.

You can then head over to your browser and go to `http://localhost:9999`. This will display
the HTML page:

![reverse proxy access][image_2]

[^1]: [Docker engine API](https://docs.docker.com/engine/api/latest/)

[^2]: [Uvicorn](https://www.uvicorn.org/)

[^3]: [Starlette](https://www.starlette.io/)

[^4]:
    [Understanding sockets](https://www.digitalocean.com/community/tutorials/understanding-sockets)
    [^4]

[^5]:
    [Fun with Unix domain sockets](https://simonwillison.net/2021/Jul/13/unix-domain-sockets/)
    [^5]

[image_1]:
    https://blob.rednafi.com/static/images/tinkering_with_unix_domain_socket/img_1.png
[image_2]:
    https://blob.rednafi.com/static/images/tinkering_with_unix_domain_socket/img_2.png
