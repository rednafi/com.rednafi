---
title: Don't add extensions to shell executables
date: 2021-11-23
tags:
    - Shell
    - TIL
---

I was browsing through the source code of
[Tom Christie's](https://github.com/tomchristie) `typesystem` library and discovered
that the shell [scripts](https://github.com/encode/typesystem/tree/master/scripts) of
the project don't have any extensions attached to them. At first, I found it odd, and
then it all started to make sense.

> Executable scripts can be written in any language and the users don't need to care
> about that. Also, not gonna lie, it looks cleaner this way.

GitHub uses this [pattern](https://github.com/github/scripts-to-rule-them-all) successfully to normalize their scripts. According to the pattern, every project should
have a folder named `scripts` with a subset or superset of the following files:

* `script/bootstrap` – installs/updates all dependencies
* `script/setup` – sets up a project to be used for the first time
* `script/update` – updates a project to run at its current version
* `script/server` – starts app
* `script/test` – runs tests
* `script/cibuild` – invoked by continuous integration servers to run tests
* `script/console` – opens a console

## References

* [Typesystem](https://github.com/encode/typesystem)
* [Scripts to rule them all - GitHub Repo](https://github.com/github/scripts-to-rule-them-all)
* [Scripts to rule them all - GitHub Blog](https://github.blog/2015-06-30-scripts-to-rule-them-all/)
