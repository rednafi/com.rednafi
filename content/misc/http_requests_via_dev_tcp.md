---

title: HTTP requests via /dev/tcp
date: 2024-08-08
tags:
    - TIL
    - Shell
---

I learned this neat Bash trick today where you can make a raw HTTP request using the
`/dev/tcp` file descriptor without using tools like `curl` or `wget`. This came in handy
while writing a health check script that needed to make a TCP request to a service.

The following script opens a TCP connection and makes a simple GET request to `example.com`:

```sh
#! /bin/bash

# Open a TCP connection to example.com on port 80 and assign file descriptor 3
exec 3<>/dev/tcp/example.com/80

# Send the HTTP GET request to the server
echo -e "GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n" >&3

# Read and print the server's response
cat <&3

# Close the file descriptor, terminating the TCP connection
exec 3>&-
```

Running this will print the response from the site to your console.

The snippet first opens a TCP connection to `example.com` on port 80 and assigns file
descriptor 3 to manage this connection. Using a file descriptor simplifies the process of
sending and receiving data over the TCP connection. The script sends an HTTP GET request to
the server by echoing the request to file descriptor 3.

The server's response is read and printed using `cat <&3`, which reads from the file
descriptor and prints the output to the console. Finally, the script closes the connection
by terminating file descriptor 3 with `exec 3>&-`. Using a file descriptor makes the code
cleaner. Without it, we would need to redirect input and output directly to
`/dev/tcp/example.com/80` for each read and write operation, making the script more
cumbersome and harder to read.

This is a Bash-specific trick and won't work in other shells like zsh or fish. It also
allows you to open UDP connections in the same manner. The Bash manpage explains the usage
like this:

```txt
/dev/tcp/host/port
    If host is a valid hostname or Internet address, and port
    is an integer port number or service name, bash attempts
    to open the corresponding TCP socket.

/dev/udp/host/port
    If host is a valid hostname or Internet address, and port
    is an integer port number or service name, bash attempts
    to open the corresponding UDP socket.
```

I used this to write the following health check script. I didn't want to install `curl` in a
sidecar container that just runs a single health check process, keeping things simpler.

```sh
#!/bin/bash

HOST="example.com"
PORT=80
HEALTH_PATH="/"

# Open a TCP connection to the specified host and port
exec 3<>/dev/tcp/$HOST/$PORT

# Send the HTTP GET request to the server
echo -e "GET $HEALTH_PATH HTTP/1.1\r\nHost: $HOST\r\nConnection: close\r\n\r\n" >&3

# Read the HTTP status from the server's response
HTTP_STATUS=$(head -n 1 <&3 | awk '{print $2}')
if [ "$HTTP_STATUS" == "200" ]; then
    echo "Service is healthy."
    exit 0
else
    echo "Service is not healthy. HTTP status: $HTTP_STATUS"
    exit 1
fi

# Close the file descriptor, terminating the TCP connection
exec 3>&-
```

The script makes a GET request to the service and checks that the HTTP status from the raw
response is 200. If not, it exits with a non-zero status.

Note that the script will fail if your service returns a 301 redirect code. Plus, you need
to make raw textual HTTP requests, which can become cumbersome if you need to do anything
beyond a simple GET call. At that point, you're better off using `curl`.
