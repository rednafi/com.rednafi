---
title: Installing Python on macOS with asdf
date: 2022-11-13
tags:
    - Python
    - TIL
---

I've just migrated from Ubuntu to macOS for work and am still in the process of setting up
the machine. I've been a lifelong Linux user and this is the first time I've picked up an OS
that's not just another flavor of Debian. Primarily, I work with Python, NodeJS, and a tiny
bit of Go. Previously, any time I had to install these language runtimes, I'd execute a
bespoke script that'd install:

-   Python via deadsnake[^1] ppa.
-   NodeJS via nvm[^2].
-   Go from the official binary source[^3].

Along with the hassle of having to manage three version managers, setting up multiple
versions of Python almost always felt like a chore. I've used pyenv[^4] before which kind of
feels like nvm and works quite well in practice. However, on Twitter, I came across this[^5]
reply by Adam Johnson which mentions that asdf[^6] can manage multiple runtimes of different
languages—one version manager to rule them all. Also, it's written in pure bash so there's
no external dependency required for the tool to work. Since I'm starting from scratch on a
new OS, I wanted to give this a tool to try. Spoiler alert, it works with zero drama. Here,
I'll quickly explain how to get up and running with multiple versions of Python and make
them work seamlessly.

## Prerequisites

For this to work, I'm assuming that you've got homebrew[^7] installed on your system.
Install asdf with the following command:

```sh
brew install asdf
```

Once asdf is installed, you'll need to install the Python plugin[^8]. Run this:

```sh
asdf plugin-add python
```

Also, you'll need to make sure that your system has these[^9] plugin-specific dependencies
in place.

## Bootstrapping Python

Once the prerequisites are fulfilled, you're ready to install the Python versions from the
source. Let's say you want to install Python 3.11. To do so, run:

```sh
asdf install python 3.11.0
```

This will install Python in the `/Users/$USER/.asdf/shims/python3.11` location. Just concat
the command to install multiple versions of Python:

```sh
asdf install 3.10.15 && asdf install 3.9.9
```

## Selecting a specific Python version

Once you've installed your desired Python versions with asdf, if you try to invoke global
Python with `python` or `python3` command, you'll encounter the following error:

```txt
No version is set for command python3
Consider adding one of the following versions in your config file at
python 3.8.15
python 3.11.0
python 3.10.8
```

To address this, you can run the next command to select the latest available version of
Python (here it's `3.11.0`) as the global default runtime:

```sh
asdf global python latest
```

Running this will add a `$HOME/.tool-versions` file with the following content:

```sh
python 3.11.0
```

You can also select other Python versions as the global runtime like this:

```sh
asdf global python <python-version>
```

In a project, if you want to use a specific Python version other than the global one, you
can run:

```sh
asdf local python <python-version>
```

This will add a `$PATH/.tool-versions` similar to the global file. Now you can just go ahead
and start using that specific version of Python. Running this command will create a virtual
environment using the locally specified Python runtime and start the interpreter inside
that:

```sh
python -m venv .venv && source .venv/bin/activate && python
```

## Removing a runtime

Running `asdf uninstall python <python-version>` will do the trick.

[^1]: [deadsnake](https://launchpad.net/~deadsnakes/+archive/ubuntu/ppa)

[^2]: [nvm](https://github.com/nvm-sh/nvm)

[^3]: [Download Go](https://go.dev/dl/)

[^4]: [pyenv](https://github.com/pyenv/pyenv)

[^5]:
    [Adam Johnson's tweet](https://twitter.com/AdamChainz/status/1591131543262867456?s=20&t=cl7NMLREat945aSICfk-9g)

[^6]: [asdf - manage multiple runtime versions with a single CLI tool](https://asdf-vm.com/)

[^7]: [homebrew](https://brew.sh/)

[^8]: [asdf Python plugin](https://github.com/asdf-community/asdf-python)

[^9]:
    [asdf plugin dependencies](https://asdf-vm.com/guide/getting-started.html#plugin-dependencies)
