---
title: Dotfile stewardship for the indolent
date: 2023-09-27
tags:
    - Shell
    - TIL
---

I'm one of those people who'll sit in front of a computer for hours, fiddling with
algorithms or debugging performance issues, yet won't spend 10 minutes to improve their
workflows. While I usually get away with this, every now and then, my inertia slithers back
to bite me. The latest episode was me realizing how tedious it is to move config files
across multiple devices when I was configuring a new MacBook Air and Mac Mini at the same
time.

I dislike customizing tools and tend to use the defaults as much as possible. However, over
the years, I've accumulated a few config files here and there, which were historically
backed up in a git repository and restored manually whenever necessary. MacOS's time machine
made sure that I didn't need to do it very often. So I never paid much attention to it.

But recently, I came across GNU stow[^1] and realized that people have been using it for
years to manage their configs. I tried it and found that it works perfectly for what I need.
It's a nifty little tool written in perl that allows you to store all of your config
files in a git repository and symlink them to the targeted directories. The tool is pretty
versatile and you can do a lot more than just dotfile management. But for this purpose, only
two commands will do. The workflow roughly goes like this:

```txt
┌─────────────────┐
│git repo [source]│
└┬────────────────┘
┌▽────────────────────────────────────────────────────────┐
│dotfiles [zsh/.zshrc, zsh/.zprofile, git/.gitconfig, ...]│
└┬────────────────────────────────────────────────────────┘
┌▽───────────────────────┐
│gnu stow creates symlink│
└┬───────────────────────┘
┌▽───────────────────────────┐
│home directory [destination]│
└┬───────────────────────────┘
┌▽────────────────────────────────────────────────────────────┐
│symlinked dotfiles [~/.zshrc, ~/.zprofile, ~/.gitconfig, ...]│
└─────────────────────────────────────────────────────────────┘
```

All of your config files will need to live in a git repo and their directory trees will have
to match the desired folder structure of the destination. That means, if you need to restore
a certain config file to `~/.config/app/.conf`, then in the source repo, the file needs to
live in the `pkg1/.config/app/.conf` directory. The source's top-level directory `pkg1` is
called a package and can be named anything. While invoking stow, we'll refer to a particular
dotfile by the package it lives within. Run:

```sh
stow -v -R -t ~ pkg1
```

Here:

* `-v (or --verbose)` makes stow run in verbose mode. When you use `-v`, stow will list the
symlinks it creates or updates, making it easier to see the changes it's making.

* `-R (or --restow)` tells stow to restow the packages. It's useful when you've already
stowed the packages previously, and want to reapply them. The `-R` flag ensures that stow
re-symlinks files, even if they already exist. This makes each run idempotent and you won't
have to worry about polluting your workspace with straggler links.

* `-t <target> (or --target=<target>)` specifies the target directory where stow should
create symlinks. The default target directory is the parent of `$pwd`. In the above command,
`-t ~` is used to set the home directory as the destination.

* `<pkg1>` is the package name you want to stow.

For a more concrete example, let's say, my source repo `~/canvas/dot` has two packages named
`git` and `zsh` where the former contains `.gitconfig` and the latter houses `.zshrc` and
`.zprofile` files:

```txt
# ~/canvas/dot

zsh
├── .zprofile
└── .zshrc
git
└── .gitconfig
```

To symlink both of them to the home directory, you'll need to run the following command from
the root of the source directory; `~/canvas/dot` in this case:

```sh
stow -v -R -t ~ zsh git
```

Then you can see the newly created symlinks in the home directory with this:

```sh
ls -lah ~ | grep '^l'
```

It prints:

```txt
lrwxr-xr-x  1 rednafi  staff  25 Sep 23 19:45 .gitconfig -> canvas/dot/git/.gitconfig
lrwxr-xr-x  1 rednafi  staff  24 Sep 23 19:52 .zprofile -> canvas/dot/zsh/.zprofile
lrwxr-xr-x  1 rednafi  staff  21 Sep 23 19:45 .zshrc -> canvas/dot/zsh/.zshrc
```

If you want to remove a config file, can unstow it with:

```sh
unstow -v -R -t ~ pkg1
```

or, manually remove the symlink with:

```sh
unlink ~/pkg1
```

One neat side effect of managing configs in this manner is that, since symlinks are pointers
to the original files living in the source repo, any changes made to the source files are
automatically reflected in the destination configs.

Here are my dotfiles[^2] and their management scripts in all their splendor!

[^1]: https://www.gnu.org/software/stow/
[^2]: https://github.com/rednafi/dot
