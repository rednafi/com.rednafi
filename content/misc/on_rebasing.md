---
title: I kind of like rebasing
date: 2024-06-18
tags:
    - git
---

People tend to get pretty passionate about Git workflows on different online forums. Some
like to rebase, while others prefer to keep the disorganized records. Some dislike the extra
merge commit, while others love to preserve all the historical artifacts. There's merit to
both sides of the discussion. That being said, I kind of like rebasing because I'm a messy
committer who:

-   Usually doesn't care for keeping atomic commits[^1].
-   Creates a lot of short commits with messages like "fix" or "wip".
-   Likes to clean up the untidy commits before sending the branch for peer review.
-   Prefers a linear history over a forked one so that `git log --oneline --graph` tells a
    nice story.

Git rebase allows me to squash my disordered commits into a neat little one, which bundles
all the changes with passing tests and documentation. Sure, a similar result can be emulated
using `git merge --squash feat_branch` or GitHub's squash-merge feature, but to me, rebasing
feels cleaner. Plus, over time, I've subconsciously picked up the tricks to work my way
around rebase-related gotchas.

Julia Evans explores the pros and cons of rebasing in detail here[^2]. Also, squashing
commits is just one of the many things that you can do with the rebase command. Here, I just
wanted to document my daily rebasing workflow where I mostly rename, squash, or fixup
commits.

## A few assumptions

Broadly speaking, there are two common types of rebasing: rebasing a feature branch onto the
main branch and interactive rebasing on the feature branch itself. The workflow assumes a
usual web service development cadence where:

-   You'll be working on a feature branch that's forked off of a main branch.
-   The main branch is protected, and you can't directly push your changes to it.
-   Once you're done with your feature work, you'll need to create a pull request against
    the main branch.
-   After your PR is reviewed and merged onto the main branch, CI automatically deploys it
    to some staging environment.

I'm aware this approach doesn't work for some niches in software development, but it's the
one I'm most familiar with, so I'll go with it.

## Rebasing a feature branch onto the main branch

Let's say I want to start working on a new feature. Here's how I usually go about it:

1. Pull in the latest `main` with `git pull`.
2. Fork off a new branch via `git switch -c feat_branch`.
3. Do the work in `feat_branch`, and before sending the PR, do interactive rebasing if
   necessary, and then rebase the `feat_branch` onto the latest changes of `main` with:

    ```sh
    git pull --rebase origin main
    ```

4. Push the changes to the remote repository with `git push origin HEAD` and send a PR
   against `main` for review.

    Here, `...origin HEAD` instructs git to push the current branch that HEAD is pointing
    to.

The 3rd step is where I often do interactive rebasing before sending the PR to make my work
presentable. The next section will explain that in detail.

Occasionally, the 4th step doesn't go as expected, and merge conflicts occur when I run
`git rebase main` from `feat_branch`. In those cases, I use my editor (VSCode) to fix the
conflict, add the changes with `git add .`, and run `git rebase --continue`. This completes
the rebase operation, and we're ready to push it to the remote.

## Rebasing interactively on the feature branch

This is an extension of the 3rd step of the previous section. Sometimes, while working on a
feature, I quickly make many messy commits and push them to the remote branch. This happens
quite frequently when I'm prototyping on a feature or updating something regarding GitHub
Actions. In these cases, I tend to make quick changes, commit with a message like "fix" or
"ci" and push to remote to see if the CI is passing. However, once I'm done, the commit log
on that branch looks like this:

```sh
git log main..@ --oneline --graph
```

This command instructs git to show only the commits that exist on `feat_branch` but not on
`main`. I learned recently that in git's context, `@` indicates the current branch. Neat,
this means I won't need to remember the branch name or do a `git branch` and then copy the
name of the current branch. Running the command returns:

```txt
* 148934c (HEAD -> feat_branch) ci
* e0f6152 ci
* 8f4dc4c ci
* bf33bf7 ci
* 2e3dce6 ci
```

I'm not too proud of the state of this `feat_branch` and prefer to tidy things up before
making a PR against `main`. One common thing I do is squash all these commits into one and
then add a proper commit message. Interactive rebasing allows me to do that. Let's say you
want to interactively rebase the 5 commits listed above and squash them. To do so, you can
run the following command from the `feat_branch`:

```sh
git rebase -i HEAD~5
```

This will open a file named `git-rebase-todo` in your default git editor (set via git
config) that looks like this:

```txt
pick 763e178 ci # empty
pick 4b10faf ci # empty
pick 7f7ce20 ci # empty
pick 88fc529 ci # empty
pick 8bc19b6 ci # empty

# Rebase a2e45d3..8bc19b6 onto a2e45d3 (5 commands)
#
# Commands:
# p, pick <commit> = use commit
# r, reword <commit> = use commit, but edit the commit message
# e, edit <commit> = use commit, but stop for amending
# s, squash <commit> = use commit, but meld into previous commit
# f, fixup [-C | -c] <commit> = like "squash" but keep only the previous
#                    commit's log message, unless -C is used, in which case
#                    keep only this commit's message; -c is same as -C but
#                    opens the editor
# x, exec <command> = run command (the rest of the line) using shell
# b, break = stop here (continue rebase later with 'git rebase --continue')
# d, drop <commit> = remove commit
# l, label <label> = label current HEAD with a name
# t, reset <label> = reset HEAD to a label
# m, merge [-C <commit> | -c <commit>] <label> [# <oneline>]
#         create a merge commit using the original merge commit's
#         message (or the oneline, if no original merge commit was
#         specified); use -c <commit> to reword the commit message
# u, update-ref <ref> = track a placeholder for the <ref> to be updated
#                       to this position in the new commits. The <ref> is
#                       updated at the end of the rebase
#
# These lines can be re-ordered; they are executed from top to bottom.
#
# If you remove a line here THAT COMMIT WILL BE LOST.
#
# However, if you remove everything, the rebase will be aborted.
#
```

Notice that the file has quite a bit of instructions that are commented out. You can perform
actions like pick, reword, edit, fixup, etc. I usually use squash and edit the
`git-rebase-todo` file like this:

```txt
pick 763e178 ci # empty
s 4b10faf ci # empty  # <- s=squash means melding this commit into the previous one
s 7f7ce20 ci # empty
s 88fc529 ci # empty
s 8bc19b6 ci # empty

# ... rest of the file remains untouched
```

Now, if you close the previous file, git will automatically open another file like the
following:

```txt
# This is a combination of 5 commits.
# This is the 1st commit message:

ci

# This is the commit message #2:

ci

# This is the commit message #3:

ci

# This is the commit message #4:

ci

# This is the commit message #5:

ci
```

After the first comment, you can put in the message for all the combined commits:

```txt
# This is a combination of 5 commits.

Add pip caching to the CI      # <- message for the combined commits

# ... you can remove rest of the content
```

If you close this file, you'll see a message on your console indicating that the rebase has
been successful:

```txt
[detached HEAD 28f5084] Add pip caching to the CI
 Date: Wed Jun 19 22:42:07 2024 +0200
Successfully rebased and updated refs/heads/feat_branch.
```

Now running `git log` will show that the messy commit has been squashed into one.

```sh
git log main..@ --oneline --graph
```

This displays:

```txt
* 28f5084 (HEAD -> feat_branch) Add pip caching to the CI
```

This is just one of the many things you can do during interactive rebasing. While I do this
most commonly, sometimes I also drop unnecessary commits to tidy up things and group
multiple commits instead of just squashing everything into one commit. All of these actions
can be done in a similar manner to squashing commits as mentioned above.

Sometimes, I don't know how many commits I'll need to interactively rebase. In those cases,
I can get the number of all the new commits on a feature branch by counting the entries in
`git log` as follows:

```sh
git log main..@ --oneline | wc -l
```

Then you can use the number from the output of the previous command to rebase `n` number of
commits:

```sh
git rebase -i HEAD~n
```

Another thing you can do is split a single commit into multiple commits. This is quite a bit
more involved and I rarely do it during interactive rebasing.

One last thing I learned recently is that you can run your tests or any arbitrary command
during interactive rebasing. To do so, start your rebase session with `--exec cmd` as
follows:

```sh
git rebase -i --exec "echo hello" HEAD~5
```

In the `git-rebase-todo` file this time, you'll see that the command is run after each
commit as follows:

```txt
pick dffb3c1 ci # empty
exec echo hello
pick 4d2fa08 ci # empty
exec echo hello
pick 2b35e4f ci # empty
exec echo hello
pick 6de7a52 ci # empty
exec echo hello

# ...
```

You can edit this file to run the exec command after any commit you want to. The commands
will run once you save and close this file. This is a neat way to run your test suite and
make sure they pass in the intermediate commits.

Fin!

[^1]:
    [Atomic commits](https://suchdevblog.com/lessons/AtomicGitCommits.html#why-should-you-write-atomic-git-commits)

[^2]:
    [Git rebase: what can go wrong?](https://jvns.ca/blog/2023/11/06/rebasing-what-can-go-wrong-/)
