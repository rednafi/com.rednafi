---
title: When to use 'git pull --rebase'
date: 2022-07-14
tags:
    - Git
    - TIL
---

Whenever your local branch diverges from the remote branch, you can't directly pull from
the remote branch and merge it into the local branch. This can happen when, for example:

* You checkout from the `main` branch to work on a feature in a branch named `alice`.
* When you're done, you merge `alice` into `main`.
* After that, if you try to pull the `main` branch from remote again and the content of
the `main` branch changes by this time, you'll encounter a merge error.

## Reproduce the issue

Create a new branch named `alice` from `main`. Run:

```bash
git checkout -b alice
```

From `alice` branch, add a line to a newly created file `foo.txt`:

```bash
echo "from branch alice" >> foo.txt
```

Add, commit, and push the branch:

```bash
git commit -am "From branch alice" && git push
```

From the GitHub UI, send a pull request against the `main` branch and merge it:

![image](https://user-images.githubusercontent.com/30027932/179367132-de85b59e-aa1c-477e-b3b2-89593f3a8b4c.png)

In your local machine, switch to `main` and try to pull the latest content merged from
the `alice` branch. You'll encounter the following error:

```
hint: You have divergent branches and need to specify how to reconcile them.
hint: You can do so by running one of the following commands sometime before
hint: your next pull:
hint:
hint:   git config pull.rebase false  # merge (the default strategy)
hint:   git config pull.rebase true   # rebase
hint:   git config pull.ff only       # fast-forward only
hint:
hint: You can replace "git config" with "git config --global" to set a default
hint: preference for all repositories. You can also pass --rebase, --no-rebase,
hint: or --ff-only on the command line to override the configured default per
hint: invocation.
fatal: Need to specify how to reconcile divergent branches.
```
This means that the history of your local `main` branch and the remote `main` branch
have diverged and they aren't reconciliable.

## Solution

From the `main` branch, you can run:

```bash
git pull --rebase
```

This will rebase your local `main` by adding your local commits on top of the remote
commits.

## References

* [When should I use git pull --rebase?](https://stackoverflow.com/questions/2472254/when-should-i-use-git-pull-rebase)
* [An example repo that reproduces the issue](https://github.com/rednafi/_pull-rebase)
