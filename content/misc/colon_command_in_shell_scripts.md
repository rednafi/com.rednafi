---
title: Colon command in shell scripts
date: 2022-12-23
slug: colon-command-in-shell-scripts
aliases:
    - /misc/colon_command_in_shell_scripts/
tags:
    - Shell
    - TIL
---

The colon `:` command is a shell utility that represents a truthy value. It can be thought
of as an alias for the built-in `true` command. You can test it by opening a shell script
and typing a colon on the command line, like this:

```sh
:
```

If you then inspect the exit code by typing `$?` on the command line, you'll see a `0`
there, which is exactly what you'd see if you had used the true command.

```sh
: ; echo $?
```

The output will be:

```txt
0
```

I find the colon command useful when running a shell script with the `-x` flag, which prints
out the commands being executed by the interpreter. For example, consider the following
script:

```bash
#!/bin/bash
# script.sh

echo "section 1: print the first 2 lines of the current directory"
ls -lah | head -n 2

echo "section 2: print the size of the /usr/bin directory"
du -sh /usr/bin
```

Running this script with `bash -x script.sh` will print the following lines:

```txt
+ echo 'section 1: print the first 2 lines of the current directory'
section 1: print the first 2 lines of the current directory
+ ls -lah
+ head -n 2
total 120
drwxr-xr-x   26 rednafi  staff   832B Dec 23 13:35 .
+ echo 'section 2: print the size of the /usr/bin directory'
section 2: print the size of the /usr/bin directory
+ du -sh /usr/bin
 76M    /usr/bin
```

Notice that the above script prints out each command first (denoted by a preceding `+` sign)
and then its respective output. However, the `echo "section..."` commands in this script are
only used for debugging purposes, to enhance the readability of the output by providing
separation between different sections. Therefore, repeating these commands and their outputs
can be a little redundant. You can use the colon command to eliminate this repetition, as
follows:

```bash
#!/bin/bash

: "section 1: print the first 2 lines of the current directory"
ls -lah | head -n 2

: "section 2: print the size of the /usr/bin directory"
du -sh /usr/bin
```

Running this script with the -x flag will produce the following output:

```txt
+ : 'section 1: print the first 2 lines of the current directory'
+ ls -lah
+ head -n 2
total 120
drwxr-xr-x   26 rednafi  staff   832B Dec 23 13:35 .
+ : 'section 2: print the size of the /usr/bin directory'
+ du -sh /usr/bin
 76M    /usr/bin
```

If you look closely, you'll see that the debug commands and their outputs are no longer
getting repeated.

[^1]:
    [Why I use the colon command - @anthonywritescode](https://www.youtube.com/watch?v=onkNf1AKSgg)
    [^1]
