---
title: Running Python linters with pre-commit hooks
date: 2020-04-06
tags:
    - Python
---

[Pre-commit hooks] can be a neat way to run automated ad-hoc _tasks_ before submitting a new
git commit. These tasks may include linting, trimming trailing whitespaces, running code
formatter before code reviews etc. Let's see how multiple Python linters and formatters can
be applied automatically before each commit to impose strict conformity on your codebase.

To keep my sanity, I only use three linters in all of my python projects:

- **[Isort]**: `Isort` is a Python utility to sort _imports_ alphabetically, and
  automatically separate them by sections and type. It parses specified files for global
  level import lines and puts them all at the top of the file grouped together by the type
  of import:
    - Future
    - Python Standard Library
    - Third Party
    - Current Python Project
    - Explicitly Local (. before import, as in: `from . import x`)
    - Custom Separate Sections (Defined by `forced_separate` list in the configuration file)
    - Custom Sections (Defined by `sections` list in configuration file)

        Inside each section, the imports are sorted alphabetically. This also automatically
        removes duplicate python imports, and wraps long from imports to the specified line
        length (defaults to 79).

- **[Black]**: `Black` is the uncompromising Python code formatter. It uses consistent rules
  to format your python code and makes sure that they look the same regardless of the
  project you're reading.

- **[Flake8]**: _Flake8_ is a wrapper around _PyFlakes_, _pycodestyle_, Ned Batchelder's
  [McCabe script]. The combination of these three linters makes sure that your code is
  compliant with [PEP-8] and free of some obvious code smells.

## Installing pre-commit

- Install using `pip`:

    ```sh
    pip install pre-commit
    ```

- Install via `curl`:

    ```sh
    curl https://pre-commit.com/install-local.py | python -
    ```

## Defining the pre-commit config file

Pre-commit configuration is a `.pre-commit-config.yaml` file where you define your hooks
(tasks) that you want to run before every commit. Once you have defined your hooks in the
config file, they will run automatically every time you say
`git commit -m "Commit message"`. The following example shows how _black_ and a few other
linters can be added as hooks to the config:

```yml
# .pre-commit-config.yaml

repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v2.3.0
    hooks:
    -   id: check-yaml
    -   id: end-of-file-fixer
    -   id: trailing-whitespace
-   repo: https://github.com/psf/black
    rev: 19.3b0
    hooks:
    -   id: black
```

## Installing the git hook scripts

Run:

```sh
pre-commit install
```

This will set up the git hook scripts and should show the following output in your terminal:

```txt
pre-commit installed at .git/hooks/pre-commit
```

Now you'll be able to implicitly or explicitly run the hooks before each commit.

## Running the hooks against all the files

By default, the hooks will run every time you say:

```sh
git commit -m "Commit message"
```

However, if you wish to run the hooks manually on every file, you can do so via:

```sh
pre-commit run --all-files
```

## Running the linters as pre-commit hooks

To run the above mentioned linters as pre-commit hooks, you need to add their respective
settings to the `.pre-commit-config.yaml` file. However, there're a few minor issues that
need to be taken care of.

- The default line length of `black` formatter is 88 (you should embrace that) but `flake8`
  caps the line at 79 characters. This raises conflict and can cause failures.

- _Flake8_ can be overly strict at times. You'll want to ignore basic errors like unused
  imports, spacing issues etc. However, since your IDE / editor also points out these issues
  anyway, you should solve them manually. You will need to configure _flake8_ to ignore some
  of these minor errors.

The following one is an example of how you can define your `.pre-commit-config.yaml` and
configure the individual hooks so that _isort_, _black_, _flake8_ linters can run without
any conflicts.

```yml
# .pre-commit-config.yaml

# isort
- repo: https://github.com/asottile/seed-isort-config
  rev: v1.9.3
  hooks:
  - id: seed-isort-config
- repo: https://github.com/pre-commit/mirrors-isort
  rev: v4.3.21
  hooks:
  - id: isort

# black
- repo: https://github.com/ambv/black
  rev: stable
  hooks:
    - id: black
      args: # arguments to configure black
        - --line-length=88
        - --include='\.pyi?$'

        # these folders wont be formatted by black
        - --exclude="""\.git |
          \.__pycache__|
          \.hg|
          \.mypy_cache|
          \.tox|
          \.venv|
          _build|
          buck-out|
          build|
          dist"""

      language_version: python3.6


# flake8
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v2.3.0
  hooks:
    - id: flake8
      args: # arguments to configure flake8
        # making isort line length compatible with black
        - "--max-line-length=88"
        - "--max-complexity=18"
        - "--select=B,C,E,F,W,T4,B9"

        # these are errors that will be ignored by flake8
        # check out their meaning here
        # https://flake8.pycqa.org/en/latest/user/error-codes.html
        - "--ignore=E203,E266,E501,W503,F403,F401,E402"
```

You can add the above lines to your configuration and run:

```sh
pre-commit run --all-files
```

This should apply the pre-commit hooks to your code base harmoniously. From now on, before
each commit, the hooks will make sure that your code complies with the rules imposed by the
linters.

<!-- Resources -->
<!-- prettier-ignore-start -->

[pre-commit hooks]:
    https://pre-commit.com/#introduction

[mccabe script]:
    https://github.com/PyCQA/mccabe

[pep-8]:
  https://www.python.org/dev/peps/pep-0008/

[isort]:
    https://github.com/timothycrosley/isort

[black]:
    https://github.com/psf/black

[flake8]:
    https://github.com/PyCQA/flake8

<!-- prettier-ignore-end -->
