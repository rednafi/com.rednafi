---
title: Unix-style pipelining with Python's subprocess module
date: 2023-07-14
tags:
    - Python
    - TIL
---

Python offers a ton of ways like `os.system` or `os.spawn*` to create new processes and run
arbitrary commands in your system. However, the documentation usually encourages you to use
the subprocess[^1] module for creating and managing child processes. The `subprocess` module
exposes a high-level `run()` function that provides a simple interface for running a
subprocess and waiting for it to complete. It accepts the command to run as a list of
strings, starts the subprocess, waits for it to finish, and then returns a
`CompletedProcess` object with information about the result. For example:

```python
import subprocess

# Here, result is an instance of CompletedProcess
result = subprocess.run(["ls", "-lah"], capture_output=True, encoding="utf-8")

# No exception means clean exit
result.check_returncode()
print(result.stdout)
```

This prints:

```txt
drwxr-xr-x   4 rednafi  staff   128B Jul  8 12:10 ..
-rw-r--r--@  1 rednafi  staff   250B Jul  8 12:10 .editorconfig
drwxr-xr-x@ 16 rednafi  staff   512B Jul 13 14:47 .git
drwxr-xr-x@  4 rednafi  staff   128B Jul  8 12:10 .github
...
```

This works great when you're carrying out simple and synchronous workflows, but it doesn't
offer enough flexibility when you need to fork multiple processes and want the processes
to run in parallel. I was working on a project where I wanted to glue a bunch of programs
together with Python and needed a way to run composite shell commands with pipes, e.g.
`echo 'foo\nbar' | grep 'foo'`. So I got curious to see how I could emulate that in Python.

Turns out you can do that easily with `subprocess.Popen`. This function allows for more
control over the subprocess. It starts the process and returns a `Popen` object immediately,
without waiting for the command to complete. This allows you to continue executing code
while the subprocess runs in parallel. `Popen` has methods like `poll()` to check if the
process has finished, `wait()` to wait for completion, and `communicate()` for interacting
with stdin/stdout/stderr. For example:

```python
import subprocess
import time

procs = []
for ip in ("1.1.1.1", "8.8.8.8"):
    print(f"Pinging {ip}...")

    proc = subprocess.Popen(["ping", "-c1", ip])
    procs.append(proc)

    print(f"Process {proc.pid} started")

# Do other stuff here while ping is running
print("Napping for a second...")
time.sleep(1)

# Wait for the processes to finish
for proc in procs:
    proc.communicate()
    print(f"Process {proc.pid} finished with code {proc.returncode}")
```

The above example shows how you can fire off subprocess tasks to run in parallel, let them
chug along in the background, do other stuff, and then collect the results at the end when
you need them. The goal here is to ping a couple of IP addresses in parallel using the
subprocess module. First, it creates an empty list to store the processes. Then it loops
through the IPs, printing a message and kicking off a ping for each one using `Popen()` so
they run asynchronously in the background. The `Popen` objects get appended to the
`procs` list.

After starting the pings, it simulates doing other work by sleeping for a second. Then it
loops through the processes again, waits for each one to finish with `communicate()`, and
prints out the process ID and return code for each ping. Running the script will give you
the following result (truncated for brevity):

```txt
Pinging 1.1.1.1...
Process 76242 started
Pinging 8.8.8.8...
Process 76243 started

Napping for a second...

64 bytes from 8.8.8.8: icmp_seq=0 ttl=56 time=26.305 ms
64 bytes from 1.1.1.1: icmp_seq=0 ttl=54 time=27.365 ms

--- 8.8.8.8 ping statistics ---
1 packets transmitted, 1 packets received, 0.0% packet loss

--- 1.1.1.1 ping statistics ---
round-trip min/avg/max/stddev = 26.305/26.305/26.305/0.000 ms

Process 76242 finished with code 0
Process 76243 finished with code 0
```

Now that we can run processes asynchronously and gather results, I'll demonstrate how I
emulated a composite UNIX command using that technique.

## Emulating UNIX pipes

Say you want to emulate the following shell command:

```sh
ps -ef | head -5
```

I'm running MacOS. So this returns:

```txt
UID PID PPID C STIME   TTY   TIME     CMD
0     1    0 0 Fri04PM  ??   23:45.79 /sbin/launchd
0   353    1 0 Fri04PM  ??   3:26.62  /usr/libexec/logd
0   354    1 0 Fri04PM  ??   0:00.09  /usr/libexec/smd
0   355    1 0 Fri04PM  ??   0:25.56  /usr/libexec/UserEventAgent (System)
```

The `ps -ef` command outputs a full list of running processes, then the pipe symbol sends
that output as input to the `head -5` command, which reads the first 5 lines from that input
and prints just those, essentially slicing off the top 5 processes. We can emulate this in
Python as follows:

```python
import subprocess

# Run 'ps -ef' and pipe the output to 'head -n 5'
ps_cmd = subprocess.Popen(["ps", "-ef"], stdout=subprocess.PIPE)

# Run 'head -n 5' and pipe the output of 'ps -ef' to it
head_cmd = subprocess.Popen(
    ["head", "-n", "5"],
    stdin=ps_cmd.stdout,
    stdout=subprocess.PIPE,
    encoding="utf-8",
)

stdout, stderr = head_cmd.communicate()
print(stdout)
```

This snippet uses the `subprocess.Popen` to run shell commands and pipe the outputs between
them. First, `ps_cmd` executes `ps -ef` and sends the full output to the `subprocess.PIPE`
buffer. Next, `head_cmd` runs `head -n 5`. The stdin of `head_cmd` is set to the stdout of
`ps_cmd`. This pipes the stdout from `ps_cmd` as input to `head_cmd`. Finally,
`head_cmd.communicate()` runs the composite command and waits for the whole thing to finish.
The final output of this snippet is the same as the `ps -ef | head -5` command.

Here's another example where we'll emulate the `sha256sum < <(echo 'foo')` command. On the
left side, `sha256sum` computes the SHA-256 cryptographic hash of an input. The construct
`<(echo 'foo')` creates a temporary file descriptor containing the output 'foo' from `echo`,
which is then redirected via `<` as standard input to `sha256sum`. Together this computes
and prints the SHA-256 hash of the input string without needing an actual file. In this
particular case, we want to compute the hash of 3 different inputs in parallel by spawning
three separate processes.

```python
import subprocess
import os


def calculate_hash(plaintext: bytes) -> subprocess.Popen:
    # Create a new process for each pass of hashing
    proc = subprocess.Popen(
        ["sha256sum"], stdin=subprocess.PIPE, stdout=subprocess.PIPE
    )

    # Send the plaintext to the stdin of the child process
    proc.stdin.write(plaintext)

    # Ensure that the child gets input
    proc.stdin.flush()
    return proc


procs = []
for _ in range(3):
    proc = calculate_hash(os.urandom(10))
    procs.append(proc)

for proc in procs:
    stdout, _ = proc.communicate()
    print(stdout.decode("utf8").strip())
```

Running this snippet will display the 3 hashes:

```txt
3db9d86f16a60907f261f97f6b9b3dce97416056dc65b9608921ee80c71885a3  -
1ee3de4990a3bca56454d6b3fb94cba1275c8c2f19a8ce6dca5cb2779b5152a7  -
f9aa6903c454c70f037328fa1504bf66700e6fdb20407fe1830223e3acec2028  -
```

First, we define a function called `calculate_hash` that accepts a bytes plaintext input and
returns a `subprocess.Popen` object. This function will spawn a new child process running
the `sha256sum` command. The stdin and stdout of the child process are configured as
`subprocess.PIPE` using the `Popen` constructor. This enables data to be piped between the
parent and child processes. Inside `calculate_hash`, the plaintext input is written to the
stdin pipe of the child process using `proc.stdin.write()`. This pipes the data into the
child's standard input stream. Next, `proc.stdin.flush()` method is called to ensure the
child process actually receives the input.

The main logic begins by initializing an empty list called `procs`. Then a loop runs 3
times, each time generating a random 10-byte string using `os.urandom`. This string is
passed to `calculate_hash`, which spawns a new `sha256sum` child process, pipes the random
data to it, and returns the `Popen` object representing the child. Each `Popen` is appended
to the procs list, so now there are 3 child processes running in parallel.

Finally, the `procs` list is iterated through and `proc.communicate()` is called on each
`Popen` instance to read back the stdout pipe from the child. This contains the output of
`sha256sum`, which is the hash of the random input string. The hash is then decoded,
stripped, and printed to the console.


[^1]: [subprocess](https://docs.python.org/3/library/subprocess)
[^2]: [Effective Python - Item 52 - Brett Slatkin](https://effectivepython.com/) [^2]
