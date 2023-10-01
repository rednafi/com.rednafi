---
title: Parametrized fixtures in Pytest
date: 2022-03-10
tags:
    - Python
    - Testing
---

While most of my Pytest fixtures don't react to the dynamically-passed values of function
parameters, there have been situations where I've definitely felt the need for that.
Consider this example:

```python
# test_src.py

import pytest


@pytest.fixture
def create_file(tmp_path):
    """Fixture to create a file in the tmp_path/tmp directory."""

    directory = tmp_path / "tmp"
    directory.mkdir()
    file = directory / "foo.md"  # The filename is hardcoded here!
    yield directory, file


def test_file_creation(create_file):
    """Check the fixture."""

    directory, file = create_file
    assert directory.name == "tmp"
    assert file.name == "foo.md"
```

Here, in the `create_file` fixture, I've created a file named `foo.md` in the `tmp` folder.
Notice that the name of the file `foo.md` is hardcoded inside the body of the fixture
function. The fixture yields the path of the directory and the created file.

Later on, the `test_file_creation` function just checks whether the fixture is working as
expected. This snippet will pass successfully if you execute it with the `pytest` command.

Now, if you needed to create three files—`foo.md`, `bar.md`, `baz.md`—how'd you do that in
the fixture? You could hardcode the names of the three files in the fixture as follows:

```python
# test_src.py

import pytest


@pytest.fixture
def create_files(tmp_path):
    """
    Fixture to create multiple files in the tmp_path/tmp
    directory.
    """

    directory = tmp_path / "tmp"
    directory.mkdir()

    # Notice the hardcoded file names. The fixture can only
    # create files with these names.
    filenames = ("foo.md", "bar.md", "baz.md")
    files = [directory / filename for filename in filenames]
    yield directory, files


def test_file_creation(create_files):
    """Check the fixture."""

    directory, files = create_files
    expected_filenames = ("foo.md", "bar.md", "baz.md")

    assert directory.name == "tmp"

    assert all(f.name for f in files if f.name in expected_filenames)
```

I had to change the name of the fixture from `create_file` to `create_files` because the
output signature of the fixture was changed to yield the directory path and a list of the
paths of the three newly created files.

While this works, it's cumbersome and inflexible. What if one of your tests needs one file
and another one demands two files to be created? How'd you tackle that?

It'd be much better if we could just pass the filename to the fixture as a parameter and the
fixture would then create the corresponding file in the temporary folder. Also, if we need
`n` files to be created, then we'll just have to execute the fixture `n` times. There's a
way to do so by leveraging fixture parameters and `@pytest.mark.parameterize` decorator.
This is how you can do it:

```python
# test_src.py

import pytest


@pytest.fixture
def create_file(tmp_path, filename):
    """Fixture to create a file in the tmp_path/tmp directory."""

    directory = tmp_path / "tmp"
    directory.mkdir()
    file = directory / filename  # The hardcoded filename is gone!
    yield directory, file


@pytest.mark.parametrize("filename", ("foo.md", "bar.md", "baz.md"))
def test_file_creation(create_file):
    """Check the fixture."""

    directory, file = create_file
    expected_filenames = ("foo.md", "bar.md", "baz.md")

    assert directory.name == "tmp"
    assert all(f for f in expected_filenames if file.name == f)
```

In this case, the fixture `create_file` takes an additional parameter called `filename` and
then yields the directory path and the file path; just as the first snippet. Later on, in
the `test_file_creation` function, the desired values of the `filename` parameter is
injected into the fixture via the `@pytest.mark.parametrize` decorator. In the above
snippet, Pytest runs the fixture 3 times and creates the desired files in 3 passes—just like
how a normal function call would behave.


[^1]: [Pass a parameter to a fixture function - Stackoverflow](https://stackoverflow.com/questions/18011902/pass-a-parameter-to-a-fixture-function) [^1]
