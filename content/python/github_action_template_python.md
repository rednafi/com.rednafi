---
title: Github action template for Python based projects
date: 2022-03-02
tags:
    - Python
    - GitHub
---

Five traits that almost all the GitHub Action workflows in my Python projects share are:

-   If a new workflow is triggered while the previous one is running, the first one will get
    canceled.
-   The CI is triggered every day at UTC 1.
-   Tests and the lint-checkers are run on Ubuntu and MacOS against multiple Python
    versions.
-   Pip dependencies are cached.
-   Dependencies, including the Actions dependencies are automatically updated via
    dependabot[^1].

I use pip-tools[^2] for managing dependencies in applications and setuptools[^3] `setup.py`
combo for managing dependencies in libraries. Here's an annotated version of the template
action syntax:

```yml
# .github/workflows/ci.yml

name: CI

on:
  # Triggers when something is pushed to the 'main' branch.
  push:
    branches:
      - master

  # Triggers when a pull request is sent against the 'main' branch.
  pull_request:
    branches:
      - master

  # Triggers everyday at 1 UTC.
  schedule:
    - cron: "0 1 * * *"

# Cancel any running workflow if the CI gets triggered again.
concurrency:
  group: ${{ github.head_ref || github.run_id }}
  cancel-in-progress: true

jobs:
  run-tests:
    # Tests are run on multiple Python versions.
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        # Multiple OSs.
        os: [ubuntu-latest, macos-latest]

        # Multiple Python versions.
        python-version: ["3.10", "3.11", "3.12"]
        include:
          - os: ubuntu-latest
            path: ~/.cache/pip # Cache location on Ubuntu
          - os: macos-latest
            path: ~/Library/Caches/pip # Cache location on MacOS

    steps:
      # Checkout to the codebase.
      - uses: actions/checkout@v3

      # Sets up Python.
      - uses: actions/setup-python@v3
        with:
          python-version: ${{ matrix.python-version }}

      # Cache pip dependencies via 'cache' actions.
      - uses: actions/cache@v2
        with:
          path: ${{ matrix.path }}
          key: "${{ runner.os }}-pip-${{ hashFiles('**/requirements.txt') }}\
            -${{ hashFiles('**/requirements-dev.txt') }}"
          restore-keys: |
            ${{ runner.os }}-pip-

      # Dev and app dependencies are kept in separate files.
      - name: Install the Dependencies
        run: |
          pip install --upgrade pip
          pip install -r requirements.txt
          pip install -r requirements-dev.txt

      # Run black, isort, flake8, etc.
      - name: Check Linter
        run: |
          echo "Checking black formatting..."
          python3 -m black --check .
          echo "Checking isort formatting..."
          python3 -m isort --check .
          echo "Checking flake8 formatting..."
          python3 -m flake8 .

      # Run the tests via pytest.
      - name: Run the tests
        run: |
          pytest -v -s
```

The dependabot config looks as follows:

```yml
# .github/dependabot.yml

version: 2
updates:
  - package-ecosystem: "pip" # See documentation for possible values.
    directory: "/" # Location of package manifests.
    schedule:
      interval: "daily"

  # Maintain dependencies for GitHub Actions.
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "daily"
```

[^1]: [Dependabot](https://github.com/dependabot)

[^2]: [pip-tools](https://github.com/jazzband/pip-tools)

[^3]: [setuptools](https://github.com/pypa/setuptools)

[^4]:
    [An active version of the above workflow](https://github.com/rednafi/stress-test-locust/blob/master/.github/workflows/build_test.yml)
    [^4]
