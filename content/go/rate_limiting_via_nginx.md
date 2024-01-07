---
title: Rate limiting via Nginx
date: 2024-01-06
tags:
    - Go
    - Networking
---

I needed to integrate rate limiting into a relatively small service that complements a
monolith I was working on. My initial thought was to apply it at the application layer, as
it seemed to be the simplest route.

Plus, I didn't want to muck around with load balancer configurations, and there's no
shortage of libraries that allow me to do this quickly in the app. However, this turned out
to be a bad idea. In the event of a DDoS[^1] or thundering herd[^2] incident, even if the
app rejects the influx of inbound requests, the app server workers still have to do a
minimal amount of work.

Also, ideally, rate limiting is an infrastructure concern; your app should be oblivious to
it. Implementing rate limiting in a layer in front of your app prevents rogue requests from
even reaching the app server in the event of an incident. So, I decided to spend some time
investigating how to do it at the load balancer layer. Nginx[^3] makes it quite
manageable[^4] without much fuss and the system was already using it as a reverse proxy.

For the initial pass, I chose to go with the default Nginx settings, avoiding any additional
components like a Redis layer for centralized rate limiting.

## App structure

For this demo, I'll proceed with a simple hello-world server written in Go. Here's the app
directory:

```txt
app
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── main.go
├── main_test.go
└── nginx
    ├── default.conf
    └── nginx.conf
```

The `main.go` file exposes the server at the `/greetings` endpoint on port `8080`:

```go
package main

import (
    "encoding/json"
    "net/http"
)

type HelloWorldResponse struct {
    Message string `json:"message"`
}

func helloWorldHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    response := HelloWorldResponse{Message: "Hello World"}
    json.NewEncoder(w).Encode(response)
}

func main() {
    http.HandleFunc("/greetings", helloWorldHandler)
    http.ListenAndServe(":8080", nil)
}
```

If you run the server with the `go run main.go` command and make a `curl` request to it,
it'll give you the following JSON output:

```sh
curl localhost:8080/greetings | jq
```

```json
{
  "message": "Hello World"
}
```

Now, we want to set up the rate limiter in the reverse proxy layer so that it will reject
requests when the inbound request rate exceeds 50 req/sec.

## Nginx config

The Nginx config lives in the `nginx` directory and consists of two config files:

```txt
app/nginx
├── default.conf
└── nginx.conf
```

The `nginx.conf` file is the core configuration file. It's where you define the server's
global settings, like how many worker processes to run, where to store log files, rate
limiting policies, and overarching security protocols.

Then there's the `default.conf` file, which is typically more focused on the configuration
of individual server blocks or virtual hosts. This is where you get into the specifics of
each website or service you're hosting on the server. Settings like server names, SSL
certificates, and specific location directives are defined here. It's tailored to manage the
nitty-gritty of how each site or application behaves under the umbrella of the global
settings set in `nginx.conf`.

You can have multiple `*.conf` files like `default.conf` and all of them are included in the
`nginx.conf` file.

### nginx.conf

Here's how the `nginx.conf` looks:

```nginx
events {
    worker_connections 1024;
}

http {
    # Define the rate limiting zone
    limit_req_zone $binary_remote_addr zone=mylimit:10m rate=50r/s;

    # Custom error pages should be defined within a server block
    # It's better to define this in the specific server configuration files.

    # Include server configurations from conf.d directory
    include /etc/nginx/conf.d/*.conf;
}
```

In the `nginx.conf` file, you'll find two main sections: `events` and `http`. Each of these
serves different purposes in the setup.

#### Events block

```nginx
events {
    worker_connections 1024;
}
```

This section defines settings for the `events` block, specifically the `worker_connections`
directive. It sets the maximum number of connections that each worker process can handle
concurrently to 1024.

#### HTTP block

```nginx
http {
    limit_req_zone $binary_remote_addr zone=mylimit:10m rate=50r/s;

    include /etc/nginx/conf.d/*.conf;
}
```

The `http` block contains directives that apply to HTTP/S traffic.

-   **Set the rate limiting policy (`limit_req_zone` directive)**

    ```nginx
    limit_req_zone $binary_remote_addr zone=mylimit:10m rate=50r/s;
    ```

    This line sets up rate limiting policy using three parameters:

    -   Key (`$binary_remote_addr`): This is the client's IP address in a binary format.
        It's used as a key to apply the rate limit, meaning each unique IP address is
        subjected to the rate limit specified.

    -   Zone (`zone=mylimit:10m`): This defines a shared memory zone named `mylimit` with a
        size of 10 megabytes. The zone stores the state of each IP address, including how
        often it has accessed the server. Approximately 160,000 IP addresses can be tracked
        with this size. If the zone is full, Nginx will start removing the oldest entries to
        free up space.

    -   Rate (`rate=50r/s`): This parameter sets the maximum request rate to 50 requests per
        second for each IP address. If the rate is exceeded, additional requests may be
        delayed or rejected.

-   **Include the default.conf file**
    ```nginx
    include /etc/nginx/conf.d/*.conf;
    ```
    This directive instructs Nginx to include additional server configurations—like
    `default.conf`—from the `/etc/nginx/conf.d/` directory. This modular approach allows for
    better organization and management of server configurations.

### default.conf

The `default.conf` file, included in the previously discussed `nginx.conf`, mainly
configures a server block in Nginx. We'll use the rate limiting policy defined there in the
`default.conf` file. Here's the content:

```nginx
server {
    listen 80 default_server;

    # Custom JSON response for 429 errors
    error_page 429 = @429;
    location @429 {
        default_type application/json;
        return 429 '{"status": 429, "message": "Too Many Requests"}';
    }

    location / {
        # Apply rate limiting
        limit_req zone=mylimit burst=10 nodelay;
        limit_req_status 429;  # Set the status code for rate-limited requests

        # Proxy settings - adjust as necessary for your application
        proxy_pass http://app:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

This file currently contains a `server` block where we employ the rate limiting policy and
set up the reverse proxy.

#### Server Block

```nginx
server {
    listen 80 default_server;
    ...
}
```

This section defines the server block, with Nginx listening on port 80, the default port for
HTTP traffic. The `default_server` parameter indicates that this server block should be used
if no other matches are found.

#### Custom error handling

```nginx
error_page 429 = @429;
location @429 {
    default_type application/json;
    return 429 '{"status": 429, "message": "Too Many Requests"}';
}
```

By default, when a client experiences rate limiting, the server returns an HTTP 503 error
with an HTML page. But we want to return 429 (Too many requests) error code with an error
message in a JSON payload. This section does that.

#### Location block

```nginx
location / {
    limit_req zone=mylimit burst=10 nodelay;
    limit_req_status 429;
    ...
}
```

The `location /` block applies to all requests to the root URL and its subdirectories.

-   **Apply the rate limiting policy**

    ```nginx
    limit_req zone=mylimit burst=10 nodelay;
    limit_req_status 429;
    ```

    These directives enforce the rate limiting policy set in `nginx.conf`. The `limit_req`
    directive uses the previously defined `mylimit` zone. The `burst` parameter allows a
    burst of 10 requests above the set rate before enforcing the limit. The `nodelay` option
    ensures that excess requests within the burst limit are processed immediately without
    delay. `limit_req_status` sets the HTTP status code for rate-limited requests to 429.

-   **Configure the proxy**

    ```nginx
    proxy_pass http://app:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection 'upgrade';
    proxy_set_header Host $host;
    proxy_cache_bypass $http_upgrade;
    ```

    These lines configure Nginx to act as a reverse proxy. Requests to this server are
    forwarded to an application server running on `http://app:8080`. The directives also
    handle HTTP headers to properly manage the connection and caching between the client,
    reverse proxy, and backend application server.

## Containerize everything

The `Dockerfile` builds the hello-world service:

```dockerfile
FROM golang:1.21 as build

WORKDIR /go/src/app
COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -o /go/bin/app

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/app /
CMD ["/app"]
```

Then we orchestrate the app with reverse proxy in the `docker-compose.yml` file:

```yml
version: "3.8"

services:
  app:
    build: .
    ports:
      - "8080:8080"

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf
      - ./nginx/default.conf:/etc/nginx/conf.d/default.conf
    depends_on:
      - app
```

The docker-compose file defines two services: `app` and `nginx`. The `app` service exposes
port 8080, meaning the app will be accessible on this port from outside the Docker
environment.

The `nginx` service sits in front of the app and is configured to expose port 80. All the
external requests will hit the default port 80 where the reverse proxy will relay the
request to the backend `app`. The custom Nginx configuration volumes are mounted in the
`volumes` section.

## Take it for a spin

Navigate to the `app` directory and start the system with the following command:

```sh
docker compose up -d
```

Now make 200 concurrent `curl` requests to see the rate limiter in action:

```sh
seq 200 | xargs -n 1 -P 100 bash -c 'curl -s location/greetings | jq'
```

This returns:

```json
{
  "message": "Hello World"
}
{
  "message": "Hello World"
}
...
{
  "status": 429,
  "message": "Too Many Requests"
}
{
  "status": 429,
  "message": "Too Many Requests"
}
{
  "message": "Hello World"
}
{
  "status": 429,
  "message": "Too Many Requests"
}
```

See the deployed service in action (might not be available later):

```sh
seq 200 | xargs -n 1 -P 100 bash -c 'curl -s 34.138.11.32/greetings | jq'
```

This will print the same output as the local service.

Nginx uses the leaky bucket algorithm to enforce the rate limiting, where requests arrive at
the bucket at various rates and leave the bucket at fixed rate. I had fun reading about it
here[^5].

Find the complete implemention[^6] on GitHub.

Fin!

[^1]:
    [What is a DDoS attack?](https://www.cloudflare.com/learning/ddos/what-is-a-ddos-attack/)

[^2]:
    [The “thundering herd” problem - Nick Groenen](https://nick.groenen.me/notes/thundering-herd/)

[^3]: [Nginx](https://www.nginx.com/)
[^4]:
    [Rate limiting with Nginx and Nginx plus](https://www.nginx.com/blog/rate-limiting-nginx/)

[^5]: [Leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket)
[^6]: [Complete implementation](https://github.com/rednafi/nginx-ratelimit)
