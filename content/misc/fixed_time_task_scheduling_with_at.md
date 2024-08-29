---
title: Fixed-time job scheduling with UNIX 'at' command
date: 2023-05-14
tags:
    - Shell
    - JavaScript
    - Networking
---

This weekend, I was working on a fun project that required a fixed-time job scheduler to run
a `curl` command at a future timestamp. I was aiming to find the simplest solution that
could just get the job done. I've also been exploring Google Bard[^1] recently and wanted to
see how it stacks up against other LLM tools like ChatGPT, BingChat, or Anthropic's Claude
in terms of resolving programming queries.

So, I asked Bard:

> _What's the simplest solution I could get away with to run a shell command at a future
> datetime?_

It introduced me to the UNIX `at` command that does exactly what I needed. Cron wouldn't be
a good fit for this particular use case, and I wasn't aware of the existence of `at` before.
So I started probing the model and wanted to document my findings for future reference.
Also, the final hacky solution that allowed me to schedule jobs remotely can be found at the
tail[^2] of this post.

## The insipid definition

The command `at` in UNIX is used to schedule one-time jobs or commands to be executed at a
specific time in the future. Internally, the system maintains a queue that adds a new entry
when a job is scheduled, and once it gets executed, the job is removed from the queue.

> **_NOTE:_** _By default, the jobs will be scheduled using the targeted machine's local
> timezone._

## Prerequisites

The command isn't included in GNU coreutils, so you might have to install it separately on
your machine.

### Debian-ish

On a Debian-flavored Linux machine, run:

```sh
apt install at
```

Then check the status of `atd` daemon. This daemon executes the scheduled jobs.

```sh
service atd status
```

```txt
* atd is running
```

If the service isn't running, then you can start the daemon with this command:

```sh
service atd start
```

```txt
 * Starting deferred execution scheduler atd    [OK]
```

### MacOS

On MacOS, scheduled jobs are carried out by `atrun` and it's disabled by default. I had to
fiddle around quite a bit to make it work on my MacBook Pro running MacOS Ventura. First,
you'll need to launch the daemon with the following command:

```sh
sudo launchctl load -w /System/Library/LaunchDaemons/com.apple.atrun.plist
```

This will start the `atrun` daemon. Or enable it for future bootups by modifying
`/System/Library/LaunchDaemons/com.apple.atrun.plist` to have:

```txt
...
<key>Enabled</key>
...

```

On modern MacOS like Ventura, unfortunately, this requires disabling SIP[^3]. Next, you'll
need to provide full disk access to `atrun`. To do so:

-   Open Spotlight and type in _Allow full disk access_.
-   On the left panel, click on _Allow applications to access all user files_.

![enable atrun on mac][image_1]

-   On the right panel, add `/usr/libexec/atrun` to the list of allowed apps. Press
    `cmd + shift + g` and type in the full path of `atrun`.

![add atrun to allow list][image_2]

You can learn more about making `atrun` work on MacOS here[^4]. Although I'm using MacOS for
development, In my particular case, making `at` work on MacOS wasn't the first priority
because I deployed the final solution to an Ubuntu container.

## A few examples

The following sections demonstrates some examples of scheduling commands to be executed in a
few different scenarios.

### Schedule at a specific time

To schedule a command to be executed at a specific time, use this command syntax:

```sh
at <time> <command>
```

For example, to schedule the command `ls -lah >> foo.txt` to be executed at `3:00 PM` local
time, you'd use the following command:

```sh
at 3pm
at> ls -lah >> foo.txt
at> <Ctrl-D>
```

Pressing `<Ctrl-D>` tells `at` that you have finished entering the command, and it should
schedule the job to run at the specified time. You'll see that at `3.00PM` local time, a
file named `foo.txt` containing the output of `ls -l` will be created.

### Schedule after a certain period of time

To schedule a command to run in a specific amount of time from now, use:

```sh
at now + <time> <command>
```

For example, to schedule `ps aux >> foo.txt` to run in 2 minutes from now, you'd use the
following command:

```sh
at now + 2 minutes
at> ps aux >> foo.txt
at> <Ctrl-D>
```

This will schedule the command to run in two minutes in the current local time.

### Schedule a script run

You can also run a script containing multiple commands at a specific time. To do this,
create a script file that houses the commands you want to run, and then use `at` to schedule
the script to be executed at the desired time.

For example, suppose you have a script file called `script.sh` that contains a `curl`
command which makes an API call and saves the output to a file. You can schedule it as such:

```sh
#!/usr/bin/env bash

curl -X GET https://httpbin.org/get >> foo.json
```

```sh
at -f script.sh now + 1 minute
```

The script will be executed in a minute from now. You can check the content of `foo.json` 1
minute later:

```json
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Host": "httpbin.org",
    "User-Agent": "curl/7.85.0",
    "X-Amzn-Trace-Id": "Root=1-646162a8-71a232d563e0c16a4a497acf"
  },
  "origin": "74.140.2.169",
  "url": "https://httpbin.org/get"
}
```

### Schedule in a non-interactive manner

What if you don't want to create a new script file and also don't want to schedule a command
interactively as shown before? You can `echo` the desired command and pipe it to `at` like
this:

```sh
echo "dig +short rednafi.com >> foo.txt" | at now + 1 minute
```

We can also run multi-line commands in a single go by taking advantage of the heredoc
format:

```sh
at now + 1 minute <<EOF
dig +short rednafi.com >> foo.txt
EOF
```

In either case, 1 minute later, you'll see that a `foo.txt` file will be created in your
local directory with the following content:

```txt
185.199.111.153
185.199.108.153
185.199.109.153
185.199.110.153
```

This command above uses `at` to schedule the execution of a `dig` command for the domain
name `rednafi.com`. In this case, `dig` performs a DNS lookup, and the scheduled time is set
to be 1 minute from now in the current local time. The output of the command is then
appended to the file `foo.txt`. The `<<EOF` syntax is used for input redirection, which
allows the command to be specified in a heredoc format without requiring you to enter the
command in interactive mode as before.

### Schedule with UNIX timestamp

You can schedule jobs using a UNIX timestamp with the `-t` flag. The `at` command requires a
timestamp in the format `[[[mm]dd]HH]MM[[cc]yy][.ss]]`. Here's an example that uses the
`date` command to generate the current datetime, adds a 30-second offset to it, formats it
to the `at`'s expected format, and schedules a job.

On Linux, run:

```sh
at -t $(date -d "+30 seconds" +"%Y%m%d%H%M.%S") <<EOF
ping -c 5 rednafi.com >> foo.txt
EOF
```

On MacOS, run:

```sh
at -t $(date -v "+30S" +"%Y%m%d%H%M.%S") <<EOF
ping -c 5 rednafi.com >> foo.txt
EOF
```

### View and manage scheduled jobs

To view a list of scheduled jobs, use the following command:

```sh
atq
```

This will display a list of all the tasks that are currently scheduled.

```txt
36      Sun May 14 18:42:00 2023
37      Sun May 14 18:42:00 2023
```

To remove a scheduled task, use the following command:

```sh
atrm <job number>
```

The job number is the number assigned to the task when it was scheduled. You can find the
job number by running the `atq` command. If you need to clear all the pending jobs, use
this:

```sh
atrm $(atq | cut -f 1)
```

## A hacky way to schedule jobs remotely

This is a hacky and probably dangerous way to do remote job scheduling. However, the beauty
of side projects is that nobody's here to tell you what to do and it's a fun way to play
with hazmats.

I needed a way to quickly prop up a service that'd allow me to schedule webhook API calls at
a fixed point in time in the future. So I exposed a simple NodeJS server that'd allow me to
schedule an API call with `at` and execute the command at the desired datetime. Here's the
complete server:

```js
// server.js
import express, { json } from "express";
import { exec } from "child_process";

const app = express();
const port = 3000;
const authToken = "some-token";

app.use(json());

app.post("/run-command", (req, res) => {
  const { command } = req.body;

  if (!command) {
    return res.status(400).json({ error: "Command not provided." });
  }

  const authHeader = req.headers.authorization;
  if (!authHeader || authHeader !== `Bearer ${authToken}`) {
    return res.status(401).json({ error: "Unauthorized." });
  }

  exec(command, (error, stdout, stderr) => {
    if (error) {
      return res
        .status(500)
        .json({ msg: "Command execution failed.", error: stderr });
    }
    res.json({ msg: "Command execution successful.", output: stdout });
  });
});

app.listen(port, () => {
  console.log(`Server listening on port ${port}`);
});
```

This endpoint takes in a shell command and just runs it on the server; bad idea, right? But
the endpoint is secured behind a Bearer token and I'm the only one who's going to use this.
Security by obscurity!

Before running the server, you'll need to install `express` and once you've done it, you can
start the server with the following command:

```sh
node server.js
```

Now, from a different console panel, you can schedule a remote task as follows:

```sh
curl -X POST -H "Authorization: Bearer some-token" \
  -H "Content-Type: application/json" \
  --data "{\"command\":\"echo 'ping -c 5 rednafi.com >> foo.txt' | at now +1min\"}" \
  http://localhost:3000/run-command
```

This will return:

```json
{"msg":"Command execution successful.","output":""}
```

In my case, I needed to POST a payload at a certain time in the future:

```sh
curl -X POST -H "Authorization: Bearer some-token" \
     -H "Content-Type: application/json" \
     --data "{\"command\":\"echo \\\"curl -X POST \
     https://webhook.site/d667acd3-477f-453c-9375-0dcbb51703bd \
     -H 'Content-Type: application/json' --data \
     '{\\\"hello\\\": \\\"world\\\"}'\\\" | at now +1min\"}" \
     http://localhost:3000/run-command
```

[^1]: [Bard](https://bard.google.com/)
[^2]: [A hacky way to schedule jobs remotely](#a-hacky-way-to-schedule-jobs-remotely)
[^3]:
    [System Integrity Protection (SIP)](https://developer.apple.com/documentation/security/disabling_and_enabling_system_integrity_protection)

[^4]: [Making "at" work on macOS](https://unix.stackexchange.com/a/478840/383934)
[^5]: ["at" command in Linux](https://linuxize.com/post/at-command-in-linux/) [^5]

[image_1]:
    https://github.com/rednafi/rednafi.com/assets/30027932/a6d775a6-b547-4ad4-80b2-e517288cc697
[image_2]:
    https://github.com/rednafi/rednafi.com/assets/30027932/e1e61f38-e35f-40df-a9bf-31ae651283f0
