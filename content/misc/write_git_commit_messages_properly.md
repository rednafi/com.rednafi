---
title: Write git commit messages properly
date: 2021-11-11
tags:
    - Git
---

Writing consistent commit messages helps you to weave a coherent story with your git
history. Recently, I've started paying attention to my commit messages. Before this, my
commit messages in this repository used to look like this:

```sh
git log --oneline -5
```

```txt
d058a23 (HEAD -> master) bash strict mode
a62e59b Updating functool partials til.
532b21a Added functool partials til
ec9191c added unfinished indexing script
18e41c8 Bash tils
```

With all the misuse of letter casings and punctuations, clearly, the message formatting
is all over the place. To tame this mayhem, I've adopted these 7 rules of writing great
commit messages:

## The seven rules of writing consistent git commit messages

1. Separate subject from body with a blank line
2. Limit the subject line to 50 characters (I often break this when there's no message
body)
3. Capitalize the subject line
4. Do not end the subject line with a period
5. Use the imperative mood in the subject line
6. Wrap the body at 72 characters
7. Use the body to explain what and why vs. how

Now, after rebasing, currently, the commit messages in this repo look like this:

```sh
git log --oneline -5
```

```sh
d058a23 (HEAD -> master) Employ bash strict mode
a62e59b Update functool partials til
532b21a Add functool partials til
ec9191c Add unfinished indexing script
18e41c8 Update bash tils
```

## Reference

* [How to write a git commit message](https://chris.beams.io/posts/git-commit/)
