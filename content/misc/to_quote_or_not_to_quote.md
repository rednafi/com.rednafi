---
title: To quote or not to quote
date: 2022-10-05
slug: to-quote-or-not-to-quote
aliases:
    - /misc/to_quote_or_not_to_quote/
tags:
    - Shell
---

My grug[^1] brain can never remember the correct semantics of quoting commands and variables
in a UNIX shell environment. Every time I work with a shell script or run some commands in a
Docker compose file, I've to look up how to quote things properly to stop my ivory tower
from crashing down. So, I thought I'd list out some of the most common rules that I usually
look up all the time.

I mostly work with bash; so that's what I'll focus on. However, the rules should be similar
for any POSIX compliant shell.

## Single quotes vs double quotes vs backticks

Use single quotes when you don't want your shell to expand variables. For example:

```sh
echo '$HOST'

```

This prints:

```txt
'$HOST'
```

In the previous snippet, the single quotes ensure that the value of the `HOST` variable
doesn't get expanded by the shell and instead the literal name of the variable is used. On
the contrary, your shell will evaulate the variable if you use double quotes here:

```sh
echo "$HOST"
```

```txt
xps
```

In this case, the command prints the name of my host machine. Lastly, a backtick pair is
used to open a subshell and run some command. The following command allows you to check out
to the `HEAD-1`th commit in Git:

```sh
git checkout `git rev-parse --short HEAD~1`
```

In the above command, first, the command within the backtick runs in a subshell and then
returns the result to the main shell. The `git checkout` part of the command in the main
shell then uses the output value of the `git rev-parse --short HEAD~1` sub-command to carry
out the intended action.

> While this works, `` `...` `` is the legacy[^2] syntax for command substitution, required
> by only the very oldest of non-POSIX-compatible Bourne shells. A better alternative is to
> use the `$(...)` syntax.

```sh
git checkout $(git rev-parse --short HEAD~1)
```

## When to quote variables

Quote if the variable can either be empty or contain any whitespace or special characters
like spaces, backslashs or wildcards. Not quoting strings with spaces often leads to the
shell breaking apart a single argument into many. Consider this command:

```sh
export x=some filename
echo $x
```

This will print:

```txt
some
```

Ideally, this should've returned `some filename`. You can fix this by quoting the value:

```sh
export x="some filename"
echo $x
```

```txt
some filename
```

In the shell environment, the value of a variable is delimited by space. So if the value of
your variable contains a space, it won't work correctly unless you quote it properly. This
can also happen while accepting a value from a user and assigning it to a variable. For
example:

```sh
read -p "Enter the name of a file: " file; cat $file
```

If the user provides a file name that contains a space or any special character like `*`,
`?` or `/`, the command above will behave unexpectedly. To ensure that the `cat` is applied
on a single file, wrap the `file` variable with double quotes.

```sh
read -p "Enter the name of a file: " file; cat "$file"
```

Instead of double quotes, if you wrap the variable with single quotes, the command will try
to apply `cat` on a file that's literally named `$file` which is most likely not what you
want.

[^1]: [Grug brained developer](https://grugbrain.dev/)

[^2]:
    [Why is $(...) preferred over `...` (backticks)?](http://mywiki.wooledge.org/BashFAQ/082)
