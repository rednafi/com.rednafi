---
title: Compose multiple levels of fixtures in pytest
date: 2022-07-21
slug: compose-multiple-levels-of-pytest-fixtures
aliases:
    - /python/compose_multiple_levels_of_pytest_fixtures/
tags:
    - Python
    - Testing
    - TIL
---

While reading the second version of Brian Okken's pytest book[^1], I came across this neat
trick to compose multiple levels of fixtures. Suppose, you want to create a fixture that
returns some canned data from a database. Now, let's say that invoking the fixture multiple
times is expensive, and to avoid that you want to run it only once per test session.
However, you still want to clear all the database states after each test function runs.
Otherwise, a test might inadvertently get coupled with another test that runs before it via
the fixture's shared state. Let's demonstrate this:

```py
# test_src.py
import pytest


@pytest.fixture(scope="session")
def create_files(tmp_path_factory):
    """Fixture that creates files in the tmp_path/tmp directory,
    writes something stuff, then and returns the directory."""

    directory = tmp_path_factory.mktemp("tmp")

    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = directory / filename
        file.write_text("Hello, World!")
    yield directory


def test_read_default_content(create_files):
    directory = create_files

    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = directory / filename
        assert file.read_text() == "Hello, World!"


def test_read_custom_content(create_files):
    directory = create_files
    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = directory / filename
        file.write_text("Hello, Mars!")

    assert file.read_text() == "Hello, Mars!"
```

In the above snippet, we've created a session-scoped fixture called `create_files` that
creates three files in a temporary directory, writes some content to them, and then yields
the directory. Afterward, we write two tests where the first one tests the files' default
content and the second one writes some stuff to each of the file and then test their
content.

If we run this with pytest, both of the tests pass. However, if we change the order of the
tests where the `test_read_custom_content` runs before `test_read_default_content`, pytest
will raise an error:

```txt
test_src.py .F                                                 [100%]

==================== FAILURES ====================
____________________ test_read_default_content ____________________

create_files = PosixPath('/tmp/pytest-of-rednafi/pytest-33/tmp0')

    def test_read_default_content(create_files):
        directory = create_files

        for filename in ("foo.txt", "bar.txt", "baz.txt"):
            file = directory / filename
>           assert file.read_text() == "Hello, World!"
E           AssertionError: assert 'Hello, Mars!' == 'Hello, World!'
E             - Hello, World!
E             + Hello, Mars!

test_src.py:32: AssertionError
==================== short test summary info ====================
FAILED test_src.py::test_read_default_content
    - AssertionError: assert 'Hello, Mars!' == 'Hello, World!'
```

Our tests behave differently when the order of their execution changes. This is bad. You
should always make sure that running your tests randomly or reversely doesn't change the
outcome of the test run. You can use a plugin like pytest-reverse[^2] to change your test
execution order.

This happens because the data of the fixture `create_files` persists across multiple tests
since it's defined as a session-scoped fixture. Here, `test_read_custom_content` overwrites
the default contents of the files and when the other test runs after this one, it can't find
the default content and hence raises an `AssertionError`. To fix this, we'll need to make
sure that the fixture's state gets cleaned up after each test function executes.

One way to achieve this is by making the `create_files` fixture function-scoped; instead of
session-scoped. If you decorate `create_files` with `@pytest.fixture(scope="function")` and
then run the above snippet in a reverse manner, you'll see that the error doesn't occur this
time. However, making the fixture function-scoped means, the fixture will be executed once
before running each test function. This can be a deal breaker if the fixture has to perform
some time-consuming setups.

To solve this, we can keep the `create_files` fixture session-scoped and use another
function-scoped fixture to clean up its state. This way, before running each test function,
the function-scoped fixture will clean up the state of the session-scoped fixture. We can
write the previous example as follows:

```py
# test_src.py
import pytest


@pytest.fixture(scope="session")
def create_files(tmp_path_factory):
    """Fixture that creates files in the tmp_path/tmp directory,
    writes something stuff, then and returns the directory."""

    directory = tmp_path_factory.mktemp("tmp")

    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = directory / filename
        file.write_text("Hello, World!")
    yield directory


@pytest.fixture(scope="function")
def get_files(create_files):
    yield create_files
    # Clean up the files after each test function runs.
    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = create_files / filename
        file.write_text("Hello, World!")


def test_read_custom_content(get_files):
    directory = get_files
    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = directory / filename
        file.write_text("Hello, Mars!")

    assert file.read_text() == "Hello, Mars!"


def test_read_default_content(get_files):
    directory = get_files

    for filename in ("foo.txt", "bar.txt", "baz.txt"):
        file = directory / filename
        assert file.read_text() == "Hello, World!"
```

Notice that I've swapped the order of the tests just for demonstration purposes. Here, we've
defined another fixture called `get_files` which is function-scoped. Underneath, `get_files`
uses `create_files` to create the file contents and then cleans up the state after the yield
statement. We could refactor some of the clean-up code to make it DRY but I intentionally
kept it verbose for simplicity's sake.

In this case, the lighter `get_files` fixture gets executed before every test function runs
and keeps the state of the `create_files` clean. On the other hand, the `create_files`
fixture gets executed only once per test session. This time, if you run the tests, all the
tests should pass successfully. We have successfully composed two different levels of
fixture functions!

[^1]:
    [Python testing with pytest - Brian Okken](https://pragprog.com/titles/bopytest2/python-testing-with-pytest-second-edition/)

[^2]: [Pytest reverse](https://github.com/adamchainz/pytest-reverse)
