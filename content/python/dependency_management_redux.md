---
title: Python dependency management redux
date: 2023-06-27
tags:
    - Python
---

One major drawback of Python's huge ecosystem is the significant variances in workflows
among people trying to accomplish different things. This holds true for dependency
management as well. Depending on what you're doing with Python—whether it's building
reusable libraries, writing web apps, or diving into data science and machine learning—your
workflow can look completely different from someone else's. That being said, my usual
approach to any development process is to pick a method and give it a shot to see if it
works for my specific needs. Once a process works, I usually automate it and rarely revisit
it unless something breaks.

Also, I actively try to abstain from picking up tools that haven't stood the test of time.
If the workflow laid out here doesn't work for you and something else does, that's
fantastic! I just wanted to document a more modern approach to the dependency management
workflow that has reliably worked for me over the years. Plus, I don't want to be the person
who still uses distutils[^1] in their package management workflow and gets reprehended by
`pip` for doing so.

## Defining the scope

Since the dependency management story in Python is a huge mess for whatever reason, to avoid
getting yelled at by the most diligent gatekeepers of the internet, I'd like to clarify the
scope of this piece. I mainly write web applications in Python and dabble in data science
and machine learning every now and then. So yeah, I'm well aware of how great conda[^2] is
when you need to deal with libraries with C dependencies. However, that's not typically my
day-to-day focus. Here, I'll primarily delve into how I manage dependencies when developing
large-scale web apps and reusable libraries.

In applications, I manage my dependencies with pip[^3] and pip-tools[^4], and for libraries,
my preferred build backend is hatch[^5]. PEP-621[^6] attempts to standardize the process of
storing project metadata in a `pyproject.toml` file, and I absolutely love the fact that
now, I'll mostly be able to define all my configurations and dependencies in a single file.
This made me want to rethink how I wanted to manage the dependencies without sailing against
the current recommended standard while also not getting swallowed into the vortex of
conflicting opinions in this space.

## In applications

Whether I'm working on a large Django monolith or exposing a microservice via FastAPI or
Flask, while packaging an application, I want to be able to:

-   Store all project metadata, linter configs, and top-level dependencies in a
    `pyproject.toml` file following the PEP-621[^6] conventions.
-   Separate the top-level application and development dependencies.
-   Generate `requirements.txt` and `requirements-dev.txt` files from the requirements
    specified in the TOML file, where the top-level and their transient dependencies will be
    pinned to specific versions.
-   Use vanilla `pip` to build the application hermetically from the locked dependencies
    specified in the `requirements*.txt` files.

The goal is to simply be able to run the following command to install all the pinned
dependencies in a reproducible manner:

```sh
pip install -r requirements.txt -r requirements-dev.txt
```

pip-tools allows me to do exactly that. Suppose, you have an app where you're defining the
top-level dependencies in a canonical `pyproject.toml` file like this:

```toml
[project]
requires-python = ">=3.8"
name = "foo-app"
version = "0.1.0"
dependencies = [
    "fastapi==0.97.0",
    "uvicorn==0.22.0",
]

[project.optional-dependencies]
dev = [
    "black>=23.3.0",
    "mypy>=1.2.0",
    "pip-tools>=6.13.0",
    "pytest>=7.3.2",
    "pytest-cov>=4.1.0",
    "ruff>=0.0.272"
]

# Even for an application, specifying a build backend is required.
# Otherwise, pip-compile command will give you an obscure error.
[tool.setuptools.packages.find]
where = ["app"]  # ["."] by default
```

Here, following PEP-621 conventions, we've specified the app and dev dependencies in the
`project.dependencies` and `project.optional-dependencies.dev` sections respectively. Now in
a virtual environment, install pip-tools and run the following commands:

```sh
# This will pin the app and dev deps to requirements*.txt files and
# generate hashes for hermetic builds

# Pin the app deps along with their build hashes
pip-compile -o requirements.txt pyproject.toml \
    --generate-hashes --strip-extras

# Use the app deps as a constraint while pinning the dev deps so that the
# dev deps don't install anything that conflicts with the app deps
echo "--constraint $(PWD)/requirements.txt" \
    | pip-compile --generate-hashes --output-file requirements-dev.txt \
    --extra dev - pyproject.toml
```

Running the commands will create two lock files `requirements.txt` and
`requirements-dev.txt` where all the pinned top-level and transient dependencies will be
listed out. The contents of the `requirements.txt` file looks like this (truncated):

```txt
#
# This file is autogenerated by pip-compile with Python 3.11
# by the following command:
#
#    pip-compile --generate-hashes --output-file=requirements.txt
# --strip-extras pyproject.toml
#
anyio==3.7.0 \
    --hash=sha256:275d9973793619a5374e1c89a4f4ad3f4b0a5510a2b5b939444bee8f4c4d37ce \
    --hash=sha256:eddca883c4175f14df8aedce21054bfca3adb70ffe76a9f607aef9d7fa2ea7f0
    # via starlette
click==8.1.3 \
    --hash=sha256:7682dc8afb30297001674575ea00d1814d808d6a36af415a82bd481d37ba7b8e \
    --hash=sha256:bb4d8133cb15a609f44e8213d9b391b0809795062913b383c62be0ee95b1db48
    # via uvicorn
...
```

Similarly, the content of `requirements-dev.txt` file goes as follows (truncated):

```txt
#
# This file is autogenerated by pip-compile with Python 3.11
# by the following command:
#
#    pip-compile --extra=dev --generate-hashes --output-file=requirements-dev.txt
#    - pyproject.toml
#
anyio==3.7.0 \
    --hash=sha256:275d9973793619a5374e1c89a4f4ad3f4b0a5510a2b5b939444bee8f4c4d37ce \
    --hash=sha256:eddca883c4175f14df8aedce21054bfca3adb70ffe76a9f607aef9d7fa2ea7f0
    # via
    #   -r -
    #   starlette
black==23.3.0 \
    --hash=sha256:064101748afa12ad2291c2b91c960be28b817c0c7eaa35bec09cc63aa56493c5 \
    --hash=sha256:0945e13506be58bf7db93ee5853243eb368ace1c08a24c65ce108986eac65915 \
...
```

Once the lock files are generated, you're free to build the application in however way you
see fit and the build process doesn't even need to be aware of the existence of `pip-tools`.
In the simplest case, you can just run `pip install` to build the application. Check out
this working example[^7] that uses the workflow explained in this section.

## In libraries

While packaging libraries, I pretty much want the same things mentioned in the application
section. However, the story of dependency management in reusable libraries is a bit more
hairy. Currently, there's no standard around a lock file and I'm not aware of a way to build
artifacts from a plain `requirements.txt` file. For this purpose, my preferred build backend
is hatch[^5]. Mostly because it follows the latest standards formalized by the associated
PEPs. From the FAQ section of the hatch docs:

> _Q: What is the risk of lock-in?_
>
> _A: Not much! Other than the plugin system, everything uses Python's established standards
> by default. Project metadata is based entirely on PEP-621[^6]/PEP-631[^8], the build
> system is compatible with PEP-517[^9]/PEP-660[^10], versioning uses the scheme specified
> by PEP-440[^11], dependencies are defined with PEP-508[^12] strings, and environments use
> virtualenv._

However, it doesn't support lock files yet:

> _The only caveat is that currently there is no support for re-creating an environment
> given a set of dependencies in a reproducible manner. Although a standard lock file format
> may be far off since PEP-665[^13] was rejected, resolving capabilities are coming to pip.
> When that is stabilized, Hatch will add locking functionality and dedicated documentation
> for managing applications._

In my experience, I haven't faced many issues regarding the lack of support for lock files
while building reusable libraries. Your mileage may vary.

Now let's say we're trying to package up a CLI that has the following source structure:

```txt
src
├── __init__.py
└── cli.py
```

The content of `cli.py` looks like this:

```python
import click


@click.command()
@click.version_option()
def cli() -> None:
    """Simple cli command to show the version of the package"""
    click.echo("Hello from foo-cli!")


if __name__ == "__main__":
    cli()
```

The corresponding `pyproject.toml` file looks as follows:

```toml
[project]
requires-python = ">=3.8"
name = "foo-cli"
dependencies = [
    "click>=8.1.3",
]
version = "0.0.1"

[project.optional-dependencies]
dev = [
    "hatch>=1.7.0",
    "black>=23.3.0",
    "mypy>=1.2.0",
    "pip-tools>=6.13.0",
    "pytest>=7.3.2",
    "pytest-cov>=4.1.0",
    "ruff>=0.0.272"
]

[project.scripts]
foo-cli = "src:cli.cli"

[build-system]
requires = ["hatchling >= 1.7.0"]
build-backend = "hatchling.build"

# We're using setuptools as the build backend
[tool.setuptools.packages.find]
where = ["src"]  # ["."] by default
```

Now install `hatch` in your virtualenv and run the following command to create the build
artifacts:

```sh
hatch build src
```

This will create the build artifacts in the `src` directory:

```txt
src
├── __init__.py
├── cli.py
├── foo_cli-0.0.1-py3-none-any.whl
└── foo_cli-0.0.1.tar.gz
```

You can now install the local wheel file to test the build:

```sh
pip install foo_cli-0.0.1-py3-none-any.whl
```

Once you've installed the CLI locally, you can test it by running `foo-cli` from your
console:

```sh
foo-cli
```

This returns:

```txt
Hello from foo-cli!
```

You can also build and install the CLI with:

```sh
pip install ".[dev]"
```

Hatch also provides a `hatch publish` command to upload the package to PyPI. For a complete
reference, check out how I shipped another CLI[^14] following this workflow.

[^1]: [distutils](https://docs.python.org/3.11/distutils/)

[^2]: [conda](https://docs.conda.io/en/latest/)

[^3]: [pip](https://pip.pypa.io/en/stable/)

[^4]: [pip-tools](https://pip-tools.readthedocs.io/en/latest/)

[^5]: [hatch](https://hatch.pypa.io/latest/)

[^6]: [PEP-621](https://peps.python.org/pep-0621/)

[^7]:
    [Example application - fastapi-nano](https://github.com/rednafi/fastapi-nano/blob/master/pyproject.toml)

[^8]: [PEP-631](https://peps.python.org/pep-0631/)

[^9]: [PEP-517](https://peps.python.org/pep-0517/)

[^10]: [PEP-660](https://peps.python.org/pep-0660/)

[^11]: [PEP-440](https://peps.python.org/pep-0440/)

[^12]: [PEP-508](https://peps.python.org/pep-0508/)

[^13]: [PEP-665](https://peps.python.org/pep-0665/)

[^14]:
    [Example library - rubric](https://github.com/rednafi/rubric/blob/main/pyproject.toml)

[^15]:
    [Using pyproject.toml in your Django project - Peter Baumgartner](https://lincolnloop.com/insights/using-pyprojecttoml-in-your-django-project/)
    [^15]

[^16]:
    [TIL: pip-tools Supports pyproject.toml - Hynek Schlawack](https://hynek.me/til/pip-tools-and-pyproject-toml/)
    [^16]
