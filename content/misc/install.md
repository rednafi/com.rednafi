---
title: The *nix install command
date: 2024-07-28
tags:
    - Shell
    - TIL
---

TIL about the `install` command on \*nix systems. A quick GitHub search for the term brought
up a ton of matches[^1]. I'm surprised I just found out about it now.

Often, I need to:

-   Create a directory hierarchy
-   Copy a config or binary file to the new directory
-   Set permissions on the file

It usually looks like this:

```sh
# Create the directory hierarchy. The -p flag creates the parent directories
# if they don't exist
mkdir -p ~/.config/app

# Copy the current config to the newly created directory. Here, conf already
# exists in the current folder
cp conf ~/.config/app/conf

# Set the file permission
chmod 755 ~/.config/app/conf
```

Turns out, the `install` command in GNU coreutils[^2] can do all that in one line:

```sh
install -D -m 755 conf ~/.config/app/conf
```

You can check the file status with:

```sh
stat ~/.config/app/conf
```

On my machine, this prints:

```txt
 File: /Users/rednafi/.config/app
  Size: 0               Blocks: 0          IO Block: 4096   regular empty file
Device: 1,16    Inode: 16439606    Links: 1
Access: (0755/-rwxr-xr-x)  Uid: (  501/ rednafi)   Gid: (   20/   staff)
Access: 2024-07-28 20:51:42.793765043 +0200
Modify: 2024-07-28 20:51:42.793765043 +0200
Change: 2024-07-28 20:51:42.793907876 +0200
Birth: 2024-07-28 20:51:42.793765043 +0200
```

The `-D` flag directs `install` to create the destination directory if it doesn't exist, and
the `-m` flag sets file permissions. The result is the same as the three lines of commands
before.

It's common for Makefiles in C/C++ projects to install binaries like this:

```sh
install -D -m 744 app_bin /usr/local/bin/app_bin
```

This copies `app_bin` to `/usr/local/bin`, creates the parent directory if necessary, and
sets permissions so only the current user has read, write, and execute permissions, while
others have read-only access.

You can also set directory permissions:

```sh
install -d -m 600 foo/bar/bazz
```

This creates the directory hierarchy:

```sh
tree foo
```

Output:

```txt
foo
└── bar
    └── bazz

3 directories, 0 files
```

Then you can copy and set file permissions with another `install` command if needed.

You can also set user or group ownership while copying a file:

```sh
install -D -m 644 -o root -g root seed.db /var/lib/app/seed.db
```

This copies `seed.db` to the destination, creates the directory if necessary, and sets the
file ownership to the root user and group.

It's a neat tool that does one thing and does it right. There are a few other options you
can read about in the man pages, but I haven't needed anything beyond the above.

[^1]:
    [Search for "install -D" on GitHub](https://github.com/search?q=%22install+-D%22++language%3Ash+NOT+npm&type=code)

[^2]:
    [GNU install](https://www.gnu.org/software/coreutils/manual/html_node/install-invocation.html#install-invocation)
