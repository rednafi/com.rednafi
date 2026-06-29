---
title: "Use curly braces while pasting shell commands"
slug: use-curly-braces-while-pasting-shell-commands
date: 2021-11-08
description: >-
    Prevent shell commands from executing immediately when pasting. Use curly braces to
    safely paste multi-line commands with hidden newline characters.
tags:
    - Shell
    - Unix
    - TIL
images:
    - "https://blob.rednafi.com/misc/use-curly-braces-while-pasting-shell-commands/cover-107ac7bd4df0.png"
aliases:
    - /misc/use_curly_braces_while_pasting_shell_commands/
discussions: []
mermaid: false
type_label: ""
atprotoPath: /misc/use-curly-braces-while-pasting-shell-commands/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mnl6jo5bqd2s"
---

Pasting shell commands can be a pain when they include hidden return `\n` characters. In
such a case, your shell will try to execute the command immediately. To prevent that, use
curly braces `{ <cmd> }` while pasting the command. Your command should look like the
following:

```sh
{ dig +short google.com }
```

Here, the spaces after the braces are significant.
