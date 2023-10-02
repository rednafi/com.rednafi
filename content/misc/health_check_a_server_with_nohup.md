---
title: Health check a server with 'nohup $(cmd) &'
date: 2022-04-18
tags:
    - Shell
    - TIL
---

While working on a project with EdgeDB[^1] and FastAPI[^2], I wanted to perform health
checks against the FastAPI server in the GitHub CI. This would notify me about the working
state of the application. The idea is to:

* Run the server in the background.
* Run the commands against the server that'll denote that the app is in a working state.
* Perform cleanup.
* Exit with code 0 if the check is successful, else exit with code 1.

The following shell script demonstrates a similar workflow with a Python HTTP server. This
script:

* Runs a Python web server in the background.
* Makes an HTTP request against the server and checks if it returns HTTP 200 (OK).
    If the request fails or the server isn't ready then waits for a second and makes the
    request again, and keeps retrying for the next 20 times before giving up.
* Performs cleanups and kills the Python processes.
* Exit with code 0 if the request is successful, else exit with code 1.

```bash
#!/bin/bash

set -euo pipefail

# Run the Python server in the background.
nohup python3 -m http.server 5000 >> /dev/null &

# Give the server enough time to be ready before accepting requests.
c=20
while [[ $c != 0 ]]
do
    # Run the healthcheck.
    if [[ $(curl -I http://localhost:5000/ 2>&1) =~ "200 OK" ]]; then
        echo "Health check passed!"

        # ...do additional cleanups if required.
        pkill -9 -i python
        exit 0
    fi
    ((c--))
    echo "Server isn't ready. Retrying..." $c
    sleep 1
done

echo "Health check failed!"
# ...do additional cleanups if required.
pkill -9 -i python
exit 1
```

The `nohup` before the `python3 -m http.server 5000` makes sure that the `SIGHUP` signal
can't reach the server and shut down the process. The ampersand `&` after the command runs
the process in the background. Afterward, the script starts making requests to the
`http://localhost:5000/` URL in a loop. If the server returns HTTP 200, the health check is
considered successful. This will break the loop and the script will be terminated with
`exit 0` status. If the server doesn't return HTTP 200 or isn't ready yet, the script will
keep retrying 20 times with a 1 second interval between each subsequent request before
giving up. A failed health check will cause the script to terminate with `exit 1` status.

[^1]: [EdgeDB](https://www.edgedb.com/)
[^2]: [FastAPI](https://fastapi.tiangolo.com/)
[^3]: [What's the difference between nohup and ampersand](https://stackoverflow.com/questions/15595374/whats-the-difference-between-nohup-and-ampersand) [^3]
