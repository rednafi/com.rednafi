---
title: Background jobs and inherited file descriptors
date: 2026-03-28
slug: background-jobs-inherited-fd
tags:
    - Shell
    - Unix
description: >-
  Why & backgrounds execution but doesn't stop output from flooding your terminal.
---

I keep a `brew update && brew upgrade && brew cleanup` alias around. Every now and then I
wrap it in a subshell and put an `&` on the end, expecting it to go to the background and
come back when it's done:

```sh
(brew update && brew upgrade && brew cleanup) &
```

But download progress, upgrade logs, and cleanup messages keep printing to the terminal
while I'm trying to do other things. `(sleep 5) &` works the way I'd expect: it vanishes,
and the shell prints `[1] + done` when it finishes.

---

When a shell forks a background job, the child process inherits the parent's [file
descriptors]. File descriptors 0, 1, and 2 (stdin, stdout, stderr) all point at the same
terminal the parent is using. `&` tells the shell to run the command without waiting for it
to finish. It says nothing about where output goes.

`sleep` never writes to fd 1 or fd 2, so there's nothing to see and backgrounding feels
clean. Any command that does write to those descriptors prints to the terminal, because
that's where they still point.

This script makes it visible. It writes to both stdout and stderr once a second for five
seconds:

```sh
#!/bin/sh

for i in $(seq 1 5); do
    echo "stdout: working on step $i"
    echo "stderr: step $i details" >&2
    sleep 1
done
echo "done"
```

Save it as `noisy.sh` and background it with `sh noisy.sh &`. Output keeps printing over
whatever you're doing at the prompt.

The fix is to redirect stdout and stderr before backgrounding. `&>` is shorthand for
`>/path 2>&1`, and it points both descriptors somewhere other than the terminal:

```sh
sh noisy.sh &>/dev/null &
```

Now the job runs silently, and the shell prints `[1] + done` when it finishes. The GIF below
shows both runs back to back:

![noisy.sh backgrounded with and without redirection][img_1]

If you want to keep the output for later, redirect to a file instead of `/dev/null`:

```sh
sh noisy.sh &>/tmp/noisy.log &
```

Going back to the brew command from earlier:

```sh
# discard output
(brew update && brew upgrade && brew cleanup) &>/dev/null &

# or keep it for later
(brew update && brew upgrade && brew cleanup) &>/tmp/brew.log &
```

---

The same inheritance applies to fd 0 (stdin), but the kernel won't let a background process
read from the terminal. If it tries, the kernel sends `SIGTTIN` and the job gets
[suspended]:

```
[1]  + suspended (tty input)  some-command
```

It sits there until you bring it back with `fg`. Backgrounding something that might prompt
for a password or a `y/n` confirmation can stall this way. The command isn't stuck. It's
waiting for terminal input it's not allowed to read.

If the command won't need input, redirecting output is enough. If it might prompt, handle
the prompts first or don't background it.

<!-- references -->
<!-- prettier-ignore-start -->

[img_1]:
    https://blob.rednafi.com/static/images/background-jobs-inherited-fd/bg_noisy.gif

[file descriptors]:
    https://pubs.opengroup.org/onlinepubs/9699919799/functions/fork.html

[suspended]:
    https://www.gnu.org/software/bash/manual/html_node/Job-Control-Basics.html

<!-- prettier-ignore-end -->
