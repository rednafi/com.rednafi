---
title: Use curly braces while pasting shell commands
date: 2021-11-08
slug: use-curly-braces-while-pasting-shell-commands
aliases:
    - /misc/use_curly_braces_while_pasting_shell_commands/
tags:
    - Shell
    - TIL
---

Pasting shell commands can be a pain when they include hidden return `\n` characters. In
such a case, your shell will try to execute the command immediately. To prevent that, use
curly braces `{ <cmd> }` while pasting the command. Your command should look like the
following:

```sh
{ dig +short google.com }
```

Here, the spaces after the braces are significant.
