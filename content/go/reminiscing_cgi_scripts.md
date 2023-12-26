---
title: Reminiscing CGI scripts
date: 2023-12-25
tag:
    - Go
    - TIL
mermaid: true
---

I've always had a thing for old-school web tech. By the time I joined the digital fray, CGI
scripts were pretty much relics, but the term kept popping up in tech forums and discussions
like ghosts from the past. So, I got curious, started reading about them, and wanted to see
if I could reason about them from the first principles. Writing one from the ground up with
nothing but Go's standard library seemed like a good idea.

Turns out, the basis of the technology is deceptively simple, but CGI scripts mostly went
out of fashion because of their limitations around performance.

## What are those

CGI scripts, or Common Gateway Interface scripts, emerged in the early 1990s as a solution
for creating dynamic web content. They acted as intermediaries between the web server and
external applications, allowing servers to process user input and return personalized
content. This made them essential for adding interactivity to websites, such as form
submissions and dynamic page updates.

The key function of CGI scripts was to handle data from web forms, process it, and then
generate an appropriate response. The server then takes this response and displays it on a
new web page. Here's how the process might look:

<!-- prettier-ignore-start -->

{{< mermaid >}}
sequenceDiagram
    participant U as Client
    participant S as Server
    participant C as CGI Script

    U->>S: Post request with a dynamic field value
    S->>C: Execute the CGI script in a new process
    Note right of C: CGI script receives the value
    C-->>S: Process and return result
    S-->>U: Respond with result
{{< /mermaid >}}

<!-- prettier-ignore-end -->

## How to write one

CGI scripts are usually written in dynamic scripting languages like Perl, Ruby, Python, or
even Bash. However, they can also be written in a static language where the server will need
execute the compiled binary. For this demo, we're going to write the server in Go, but the
CGI script itself will be written in Bash.

Here's the plan:

-   Set up a basic HTTP server in Go.
-   The server will await an HTTP POST request containing a form field called `name`.
-   Upon receiving the request, the server will extract the value of `name`.
-   A Bash CGI script is then called with the value of `name`, outputting `Hello <name>` in
    HTML.
-   The server will then return this HTML response to the client.

The server lives in a single `main.go` script. I'm leaving out Go's verbose error handling
for clarity.

```go
package main

import (
    "fmt"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
)

// Leaving out all the error handling for brevity
func handler(w http.ResponseWriter, r *http.Request) {

    // ParseForm parses the raw query from the URL and updates r.Form.
    r.ParseForm()
    name := r.FormValue("name")

    // Get the current working directory
    cwd, _ := os.Getwd()
    scriptPath := filepath.Join(cwd, "cgi-script.sh")

    // Execute the script with the name as an argument
    out, _ := exec.Command(scriptPath, name).Output()

    // Set the content type and write the output to the response
    w.Header().Set("Content-Type", "text/html")
    w.Write(out)
}

func main() {
    http.HandleFunc("/", handler)
    fmt.Println("Server started on :8080")
    http.ListenAndServe(":8080", nil)
}
```

Upon every new request, the server above will execute a CGI script written in Bash. Name the
shell script as `cgi-script.sh` and place it in the same directory as the server's `main.go`
file. Here's how it looks:

```sh
#!/bin/bash

# Stop upon error
set -euo pipefail

# Get the name argument
name=$1

# Output HTML
echo "<html><body>Hello ${name}</body></html>"
```

The script just accepts `name` as an argument, sets the `Content-Type` header, injects the
dynamically passed value of `name`, and echos the out the HTML response. The server then
just relays it back to the client. To test this:

-   Run the server with `go run main.go`.
-   Set the permission of the CGI script:
    ```sh
    sudo chmod +x cgi-script.sh
    ```
-   Make a cURL request:
    ```sh
    curl -X POST http://localhost:8080 -d "name=Redowan"
    ```

This returns the following response:

```txt
<html><body>Hello Redowan</body></html>
```

## Why they didn't catch on

CGI scripts have fallen out of favor primarily due to concerns related to performance and
security. When a CGI script is executed, it initiates a new process for each request. While
this approach is straightforward, it becomes increasingly inefficient as web traffic volume
grows. However, it's worth noting that modern Linux kernels have made improvements in
process spawning, and solutions like FastCGI utilize persistent process pools to reduce the
overhead of creating new processes. Nevertheless, you still incur the VM startup cost for
each request when using interpreted languages like Python or Ruby.

Modern web servers and frameworks, such as Flask, Django, or Go servers, have addressed
these inefficiencies by maintaining persistent server processes. This, along with the
advantage of not having to bear the VM startup cost, has led people to opt for these
alternatives.

Another concern worth considering is the evident security issues associated with CGI
scripts. Even in our simple example, the Bash script accepts any value for the `name`
parameter and passes it directly to the response. This exposes a significant vulnerability
to injection attacks. While it's possible to manually sanitize the input before passing it
to the next step, it's reassuring to know that many of these security steps are
automatically handled for you by almost any modern web framework. Fin!

[^1]:
    [Apache Tutorial: Dynamic Content with CGI](https://httpd.apache.org/docs/2.4/howto/cgi.html)[^1]
