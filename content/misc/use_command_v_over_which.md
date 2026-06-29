---
title: "Use 'command -v' over 'which' to find a program's executable"
slug: use-command-v-over-which
date: 2021-11-16
description: >-
    Replace which with command -v for POSIX-compliant executable lookup. Learn why command
    -v is the portable alternative for finding program paths.
tags:
    - Shell
    - Unix
    - TIL
    - CLI
images:
    - "https://blob.rednafi.com/misc/use-command-v-over-which/cover-4e06efeb2402.png"
aliases:
    - /misc/use_command_v_over_which/
discussions: []
mermaid: false
type_label: ""
atprotoPath: /misc/use-command-v-over-which/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mnl6jo3c6u2b"
---

One thing that came to me as news is that the command `which` - which is the de-facto tool
to find the path of an executable - is not POSIX compliant. The recent [Debian which hunt]
brought it to my attention. The POSIX-compliant way of finding an executable program is
`command -v`, which is usually built into most of the shells.

So, instead of doing this:

```sh
which python3.12
```

Do this:

```sh
command -v which python3.12
```

<!-- references -->
<!-- prettier-ignore-start -->

[debian which hunt]:
    https://lwn.net/Articles/874049/

<!-- prettier-ignore-end -->
