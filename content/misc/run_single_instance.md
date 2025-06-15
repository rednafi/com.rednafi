---
title: Running only a single instance of a process
date: 2024-12-31
tags:
    - Shell
    - Python
    - Go
---

I've been having a ton of fun fiddling with Tailscale[^1] over the past few days. While
setting it up on a server, I came across this shell script[^2] that configures the `ufw`
firewall on Linux to ensure direct communication across different nodes in my tailnet. It
has the following block of code that I found interesting (added comments for clarity):

```sh
#!/usr/bin/env bash

# Define the path for the PID file, using the script's name to ensure uniqueness
PIDFILE="/tmp/$(basename "${BASH_SOURCE[0]%.*}.pid")"

# Open file descriptor 200 for the PID file
exec 200>"${PIDFILE}"

# Try to acquire a non-blocking lock; exit if the script is already running
flock -n 200 \
    || {
        echo "${BASH_SOURCE[0]} script is already running. Aborting..."; exit 1;
    }

# Store the current process ID (PID) in the lock file for reference
PID=$$
echo "${PID}" 1>&200

# Do work (in the original script, real work happens here)
sleep 999
```

Here, `flock` is a Linux command that ensures only one instance of the script runs at a time
by locking a specified file (e.g., `PIDFILE`) through a file descriptor (e.g., `200`). If
another process already holds the lock, the script either waits or exits immediately. Above,
it bails with an error message and exit code 1.

If you try running two instances of this script, the second one will exit with this message:

```txt
<script-name> script is already running. Aborting...
```

On most Linux distros, `flock` comes along with the coreutils. If not, it's easy to install
with your preferred package manager.

## A more portable version

On macOS, the file locking mechanism is different, and `flock` doesn't work there. To make
your script portable, you can use `mkdir` in the following manner to achieve a similar
result:

```sh
#!/usr/bin/env bash

LOCKDIR="/tmp/$(basename "${BASH_SOURCE[0]%.*}.lock")"

# Try to create the lock directory
mkdir "${LOCKDIR}" 2>/dev/null || {
  echo "Another instance is running. Aborting..."
  exit 1
}

# Set up cleanup for the lock directory
trap "rmdir \"${LOCKDIR}\"" EXIT

# Main script logic
echo "Acquired lock, doing important stuff..."
# ... your script logic ...
sleep 999
```

This works because `mkdir` is atomic. It creates the lock directory (`LOCKDIR`) in `/tmp` or
fails if the directory already exists. This acts as a marker for the running instance. If
successful, the script sets up a `trap` to remove the directory on exit and continues to the
main logic. If `mkdir` fails, it means another instance of the process is running, and the
script exits with a message.

This is almost as effective as the `flock` version. Since I rarely write scripts for
non-Linux environments, either option is fine!

## With Python

Oftentimes, I opt for Python when I need to write larger scripts. The same can be achieved
in Python like this:

```py
import fcntl
import os
import sys
import time

# Use the script name to generate a unique lock file
LOCKFILE = f"/tmp/{os.path.basename(__file__)}.lock"


def work() -> None:
    time.sleep(999)


if __name__ == "__main__":
    try:
        # Open a file and acquire an exclusive lock
        with open(LOCKFILE, "w") as lockfile:
            fcntl.flock(lockfile, fcntl.LOCK_EX | fcntl.LOCK_NB)
            print("Acquired lock, running script...")

            # Main script logic here
            work()

    except BlockingIOError:
        print("Another instance is running. Exiting.")
        sys.exit(1)
```

The script uses `fcntl.flock` to prevent multiple instances from running. It creates a lock
file (`LOCKFILE`) in the `/tmp` directory, named after the scripts filename. When the script
starts, it opens the file in write mode and tries to lock it with `fcntl.flock` using an
exclusive lock (`LOCK_EX`). The `LOCK_NB` flag makes the operation non-blocking. If another
process holds the lock, the script exits with a message.

> _This approach works on both Linux and macOS, as both support `fcntl` for file-based
> locks. The lock is automatically released when the file is closed, either at the end of
> the script or the `with` block._

## With Go

I was curious about doing it in Go. It's quite similar to Python:

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "syscall"
    "time"
)

// Use the script name (basename) to generate a unique lock file
var lockfile = fmt.Sprintf("/tmp/%s.lock", filepath.Base(os.Args[0]))

func work() {
    time.Sleep(999 * time.Second)
}

func main() {
    // Open the lock file
    file, err := os.OpenFile(lockfile, os.O_CREATE|os.O_RDWR, 0644)
    if err != nil {
        fmt.Println("Failed to open lock file:", err)
        os.Exit(1)
    }
    defer file.Close()

    // Try to acquire an exclusive lock
    err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
    if err != nil {
        fmt.Println("Another instance is running. Exiting.")
        os.Exit(1)
    }

    // Release the lock on exit
    defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

    fmt.Println("Acquired lock, running script...")

    // Main script logic
    work()
}
```

Like the Python example, this uses `syscall.Flock` to prevent multiple script instances. It
creates a lock file based on the script's name using `filepath.Base(os.Args[0])` and stores
it in `/tmp`. The script tries to acquire an exclusive, non-blocking lock
(`LOCK_EX | LOCK_NB`). If unavailable, it exits with a message. The lock is automatically
released when the file is closed in the `defer` block.

Underneath, Go makes sure that `syscall.Flock` works on both macOS and Linux.

[^1]: [Tailscale](https://tailscale.com/)

[^2]:
    [Update tailscale ufw rules](https://github.com/AT3K/Tailscale-Firewall-Setup/blob/main/update_tailscale_ufw_rules.sh)
