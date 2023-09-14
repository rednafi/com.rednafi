---
title: Use 'command -v' over 'which' to find a program's executable
date: 2021-11-16
tags:
    - Shell
    - TIL
---

One thing that came to me as news is that the command `which`—which is the de-facto tool
to find the path of an executable—is not POSIX compliant. The recent Debian [debacle] around
`which` brought it to my attention. The POSIX-compliant way of finding an executable program
is `command -v`, which is usually built into most of the shells.

So, instead of doing this:

```sh
which python3.10
```

Do this:

```sh
command -v which python3.12
```

## References

* [Debian's which hunt][debacle]
* [TIL: which is not POSIX]


[debacle]: https://lwn.net/Articles/874049/
[til: which is not posix]: https://hynek.me/til/which-not-posix/
