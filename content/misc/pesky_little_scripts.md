---
title: Pesky little scripts
date: 2023-10-29
slug: pesky-little-scripts
aliases:
    - /misc/pesky_little_scripts/
tags:
    - Shell
description: >-
  Organize custom scripts with comma-prefixed naming. Improve tab completion and
  eliminate clutter by prefixing script names with special characters.
---

I like writing custom scripts to automate stuff or fix repetitive headaches. Most of them
are shell scripts, and a few of them are written in Python. Over the years, I've accumulated
quite a few of them. I use Git and [GNU stow] to manage them across different machines, and
the [dotfile workflow] is quite effective. However, as the list of scripts grows larger,
invoking them becomes a pain because the tab completion results get cluttered with other
system commands. Plus, often I even forget the initials of a script's name and stare at my
terminal while the blinking cursor facepalms at my stupidity.

I was watching this [amazing talk by Brandon Rhodes] that proposes quite an elegant solution
to this problem. It goes like this:

> _All your scripts should start with a character as a prefix that doesn't have any special
> meaning in the shell environment. Another requirement is that no other system command
> should start with your chosen character._

That way, when you type the prefix character and hit tab, only your custom scripts should
appear and nothing else. This works with your aliases too!

The dilemma here is picking the right character that meets both of the requirements.
Luckily, Brandon did the research for us. Turns out, the shell environment uses pretty much
all the characters on the keyboard as special characters other than these 6:

```txt
@ _ + - : ,
```

Among them, the first 5 requires pressing the Shift key, which is inconvenient. But the
plain old comma `,` is right there. You can start your script or alias names with a comma
`,` and it'll be golden.

My tab completion looks like this:

```txt
rednafi@air:~/canvas/rednafi.com
$ ,
,brclr        ,clear-cache  ,docker-prune-containers  ,redis
,brpre        ,docker-nuke  ,docker-prune-images      ,www
```

All my aliases start with `,` too so that they also appear in the list with the custom
scripts. Fin!

<!-- references -->
<!-- prettier-ignore-start -->

[gnu stow]:
    https://www.gnu.org/software/stow/

[dotfile workflow]:
    /misc/dotfile-stewardship-for-the-indolent/

[amazing talk by Brandon Rhodes]:
    https://www.youtube.com/watch?v=pybtvFFRYFs

<!-- prettier-ignore-end -->
