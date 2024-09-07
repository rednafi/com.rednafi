---
title: No really, Python's pathlib is great
date: 2020-04-13
tags:
    - Python
---

When I first encountered Python's `pathlib` module for path manipulation, I brushed it aside
assuming it to be just an OOP way of doing what `os.path` already does quite well. The
official doc also dubs it as the `Object-oriented filesystem paths`. However, back in 2019
when ticket[^1] confirmed that Django was replacing `os.path` with `pathlib`, I got curious.

The `os.path` module has always been the de facto standard for working with paths in Python.
But the API can feel massive as it performs a plethora of other loosely coupled system
related jobs. I've to look things up constantly even to perform some of the most basic tasks
like joining multiple paths, listing all the files in a folder having a particular
extension, opening multiple files in a directory etc. The `pathlib` module can do nearly
everything that `os.path` offers and comes with some additional cherries on top.

## Problem with Python's path handling

Traditionally, Python has represented file paths as regular text strings. So far, using
paths as strings with `os.path` module has been adequate although a bit cumbersome. However,
paths are not actually strings and this has necessitated the usage of multiple modules to
provide disparate functionalities that are scattered all around the standard library,
including libraries like `os`, `glob`, and `shutil`. The following code uses three modules
just to copy multiple python files from current directory to another directory called `src`:

```python
from glob import glob
import os
import shutil

for fname in glob("*.py"):
    new_path = os.path.join("src", fname)
    shutil.copy(fname, new_path)
```

The above pattern can get complicated fairly quickly and you have to know or look for
specific modules and methods in a large search space to perform your path manipulations.
Let's have a look at a few more examples of performing the same tasks using `os.path` and
`pathlib` modules.

## Joining & creating new paths

Say you want to achieve the following goals:

-   There is a file named `file.txt` in your current directory and you want to create the
    path for another file named `file_another.txt` in the same directory.
-   Then you want to save the absolute path of `file_another.txt` in a new variable.

Let's see how you'd usually do this via the `os` module.

```python
from os.path import abspath, dirname, join

file_path = abspath("./file.txt")
base_dir = dirname(file_path)
file_another_path = join(base_dir, "file_another.txt")
```

The variables `file_path`, `base_dir`, `file_another_path` look like this on my machine:

```python
print("file_path:", file_path)
print("base_dir:", base_dir)
print("file_another_path:", file_another_path)
```

```txt
>>> file_path: /home/rednafi/code/demo/file.txt
>>> base_dir: /home/rednafi/code/demo
>>> file_another_path: /home/rednafi/code/demo/file_another.txt
```

You can use the usual string methods to transform the paths but generally, that's not a good
idea. So, instead of joining two paths with `+` like regular strings, you should use
`os.path.join()` to join the components of a path. This is because different operating
systems do not define paths in the same way. Windows uses `"\"` while Mac and \*nix based
OSes use `"/"` as a separator. Joining with `os.path.join()` ensures correct path separator
on the corresponding operating system. Pathlib module uses `"/"` operator overloading and
make this a little less painful.

```python
from pathlib import Path

file_path = Path("file.txt").resolve()
base_dir = file_path.parent
file_another_path = base_dir / "another_file.txt"

print("file_path:", file_path)
print("base_dir:", base_dir)
print("file_another_path:", file_another_path)
```

```txt
>>> file_path: /home/rednafi/code/demo/file.txt
>>> base_dir: /home/rednafi/code/demo
>>> file_another_path: /home/rednafi/code/demo/file_another.txt
```

The `resolve` method finds out the absolute path of the file. From there you can use the
`parent` method to find out the base directory and add the `another_file.txt` accordingly.

## Making directories & renaming files

Here’s a piece of code that:

-   Tries to make a `src/stuff/` directory when it already exists.
-   Renames a file in the `src` directory called `.config` to `.stuffconfig`.

```python
import os
import os.path

os.makedirs(os.path.join("src", "stuff"), exist_ok=True)
os.rename("src/.config", "src/.stuffconfig")
```

Here is the same thing done using the `pathlib` module:

```python
from pathlib import Path

Path("src/stuff").mkdir(parents=True, exist_ok=True)
Path("src/.config").rename("src/.stuffconfig")
```

```txt
>>> PosixPath('src/.stuffconfig')
```

Notice the output where the renamed file path is printed. It's not a simple string, rather a
`PosixPath` object that indicates the type of host system (Linux in this case). You can
almost always use stringified path values and the Path objects interchangeably.

## Listing specific types of files in a directory

Let's say you want to recursively visit nested directories and list `.py` files in a
directory called source. The directory looks like this:

```txt
src/
├── stuff
│   ├── __init__.py
│   └── submodule.py
├── .stuffconfig
├── somefiles.tar.gz
└── module.py
```

Usually, `glob` module is used to resolve this kind of situation:

```python
from glob import glob

top_level_py_files = glob("src/*.py")
all_py_files = glob("src/**/*.py", recursive=True)

print(top_level_py_files)
print(all_py_files)
```

```txt
>>> ['src/module.py']
>>> ['src/module.py', 'src/stuff/__init__.py', 'src/stuff/submodule.py']
```

The above approach works perfectly. However, if you don't want to use another module just
for a single job, `pathlib` has embedded `glob` and `rglob` methods. You can entirely ignore
glob and achieve the same result in the following way:

```python
from pathlib import Path

top_level_py_files = Path("src").glob("*.py")
all_py_files = Path("src").rglob("*.py")

print(list(top_level_py_files))
print(list(all_py_files))
```

This will also print the same as before:

```txt
>>> [PosixPath('src/module.py')]
>>> [PosixPath('src/module.py'),
    PosixPath('src/stuff/__init__.py'),
    PosixPath('src/stuff/submodule.py')]
```

By default, both `Path.glob` and `Path.rglob` returns a generator object. Calling `list` on
them gives you the desired result. Notice how `rglob` method can discover the desired files
without you having to mention the directory structure with wildcards explicitly. Pretty
neat, huh?

## Opening multiple files & reading their contents

Now let's open the `.py` files and read their contents that you recursively discovered in
the previous example:

```python
from glob import glob

contents = []
for fname in glob("src/**/*.py", recursive=True):
    with open(fname, "r") as f:
        contents.append(f.read())

print(contents)
```

```txt
>>> ['from contextlib ...']
```

The `pathlib` implementation is almost identical as above:

```python
from pathlib import Path

contents = []
for fname in Path("src").rglob("*.py"):
    with open(fname, "r") as f:
        contents.append(f.read())

print(contents)
```

```txt
>>> ['from contextlib import ...']
```

You can also cook up a more robust implementation with generator comprehension and context
manager:

```python
from contextlib import ExitStack
from pathlib import Path

# ExitStack ensures all files are properly closed after o/p
with ExitStack() as stack:
    streams = (
        stack.enter_context(open(fname, "r"))
        for fname in Path("src").rglob("*.py")
    )
    contents = [f.read() for f in streams]

print(contents)
```

```txt
>>> ['from contextlib import ...']
```

## Anatomy of the pathlib module

Primarily, `pathlib` has two high-level components, `pure path` and `concrete path`. Pure
paths are absolute `Path` objects that can be instantiated regardless of the host operating
system. On the other hand, to instantiate a concrete path, you need to be on the specific
type of host expected by the class. These two high level components are made out of six
individual classes internally coupled by inheritance. They are:

1. PurePath (Useful when you want to work with windows path on a Linux machine)
2. PurePosixPath (Subclass of `PurePath`)
3. PureWindowsPath (Subclass of `PurePath`)
4. Path (Concrete path object, most of the time, you'll be dealing with this one)
5. PosixPath (Concrete posix path, subclass of `Path`)
6. WindowsPath (Concrete windows path, subclass of `Path`)

This UML diagram from the official docs does a better job at explaining the internal
relationships between the component classes.

![pathlib path hierarchy][image_1]

Unless you are doing cross platform path manipulation, most of the time you'll be working
with the concrete `Path` object. So I'll focus on the methods and properties of `Path` class
only.

### Operators

Instead of using `os.path.join` you can use `/` operator to create child paths.

```python
from pathlib import Path

base_dir = Path("src")
child_dir = base_dir / "stuff"
file_path = child_dir / "__init__.py"

print(file_path)
```

```txt
>>> PosixPath('src/stuff/__init__.py')
```

### Attributes & methods

The following tree shows an inexhaustive list of attributes and methods that are associated
with `Path` object. I have cherry picked some of the attributes and methods that I use most
of the time while doing path manipulation. Head over to the official docs for a more
detailed list. We'll linearly traverse through the tree and provide necessary examples to
grasp their usage.

```txt
Path
│
├── Attributes
│       ├── parts
│       ├── parent & parents
│       ├── name
│       ├── suffix & suffixes
│       └── stem
│
│
└── Methods
        ├── joinpath(*other)
        ├── cwd()
        ├── home()
        ├── exists()
        ├── expanduser()
        ├── glob()
        ├── rglob(pattern)
        ├── is_dir()
        ├── is_file()
        ├── is_absolute()
        ├── iterdir()
        ├── mkdir(mode=0o777, parents=False, exist_ok=False)
        ├── open(mode='r', buffering=-1, encoding=None, errors=None, newline=None)
        ├── rename(target)
        ├── replace(target)
        ├── resolve(strict=False)
        └── rmdir()
```

Let's dive into their usage one by one. For all the examples, We'll be using the previously
seen directory structure.

```txt
src/
├── stuff
│   ├── __init__.py
│   └── submodule.py
├── .stuffconfig
├── somefile.tar.gz
└── module.py
```

#### Path.parts

Returns a tuple containing individual components of a path.

```python
from pathlib import Path

file_path = Path("src/stuff/__init__.py")
file_path.parts
```

```txt
>>> ('src', 'stuff', '__init__.py')
```

#### Path.parents & Path.parent

`Path.parents` returns an immutable sequence containing the all logical ancestors of the
path. While `Path.parent` returns the immediate predecessor of the path.

```python
file_path = Path("src/stuff/__init__.py")

for parent in file_path.parents:
    print(parent)
```

```txt
>>> src/stuff
... src
... .
```

```python
file_path.parent
```

```txt
>>> PosixPath('src/stuff')
```

#### Path.name

Returns the last component of a path as string. Usually used to extract file name from a
path.

```python
from pathlib import Path

file_path = Path("src/module.py")
file_path.name
```

```txt
>>> 'module.py'
```

#### Path.suffixes & Path.suffix

`Path.suffixes` returns a list of extensions of the final component. `Path.suffix` only
returns the last extension.

```python
from pathlib import Path

file_path = Path("src/stuff/somefile.tar.gz")
file_path.suffixes
```

```txt
>>> ['.tar', '.gz']
```

```python
file_path.suffix
```

```txt
>>>'.gz'
```

#### Path.stem

Returns the final path component without the suffix.

```python
from pathlib import Path

file_path = Path("src/stuff/somefile.tar.gz")
file_path.stem
```

```txt
>>> 'somefile.tar'
```

#### Path.is_absolute

Checks if a path is absolute or not. Return boolean value.

```python
from pathlib import Path

file_path = Path("src/stuff/somefile.tar.gz")
file_path.is_absolute()
```

```txt
>>> False
```

#### Path.joinpath(\*other)

This method is used to combine multiple components into a complete path. This can be used as
an alternative to `"/"` operator for joining path components.

```python
from pathlib import Path

file_path = Path("src").joinpath("stuff").joinpath("__init__.py")
file_path
```

```txt
>>> PosixPath('src/stuff/__init__.py')
```

#### Path.cwd()

Returns the current working directory.

```python
from pathlib import Path

file_path = Path("src/stuff/somefile.tar.gz")
file_path.cwd()
```

```txt
>>> PosixPath('/home/rednafi/code/demo')
```

#### Path.home()

Returns home directory.

```python
from pathlib import Path

Path.home()
```

```txt
>>> PosixPath('/home/rednafi')
```

#### Path.exists()

Checks if a path exists or not. Returns boolean value.

```python
from pathlib import Path

file_path = Path("src/stuff/thisisabsent.py")
file_path.exists()
```

```txt
>>> False
```

#### Path.expanduser()

Returns a new path with expanded `~` symbol.

```python
from pathlib import Path

file_path = Path("~/code/demo/src/stuff/somefile.tar.gz")
file_path.expanduser()
```

```txt
>>> PosixPath('/home/rednafi/code/demo/src/stuff/somefile.tar.gz')
```

#### Path.glob()

Globs and yields all file paths matching a specific pattern. Let's discover all the files in
`src/stuff/` directory that have `.py` extension.

```python
from pathlib import Path

dir_path = Path("src/stuff/")
file_paths = dir_path.glob("*.py")

print(list(file_paths))
```

```txt
>>> [PosixPath('src/stuff/__init__.py'), PosixPath('src/stuff/submodule.py')]
```

#### Path.rglob(pattern)

This is like `Path.glob` method but matches the file pattern recursively.

```python
from pathlib import Path

dir_path = Path("src")
file_paths = dir_path.rglob("*.py")

print(list(file_paths))
```

```txt
>>> [PosixPath('src/module.py'),
    PosixPath('src/stuff/__init__.py'),
    PosixPath('src/stuff/submodule.py')]
```

#### Path.is_dir()

Checks if a path points to a directory or not. Returns boolean value.

```python
from pathlib import Path

dir_path = Path("src/stuff/")
dir_path.is_dir()
```

```txt
>>> True
```

#### Path.is_file()

Checks if a path points to a file. Returns boolean value.

```python
from pathlib import Path

dir_path = Path("src/stuff/")
dir_path.is_file()
```

```txt
>>> False
```

#### Path.is_absolute()

Checks if a path is absolute or relative. Returns boolean value.

```python
from pathlib import Path

dir_path = Path("src/stuff/")
dir_path.is_absolute()
```

```txt
>>> False
```

#### Path.iterdir()

When the path points to a directory, this yields the content path objects.

```python
from pathlib import Path

base_path = Path("src")
contents = [content for content in base_path.iterdir()]

print(contents)
```

```txt
>>> [PosixPath('src/stuff'),
     PosixPath('src/module.py'),
     PosixPath('src/.stuffconfig')]
```

#### Path.mkdir(mode=0o777, parents=False, exist_ok=False)

Creates a new directory at this given path.

**Parameters:**

-   **mode:**(_str_) Posix permissions (mimicking the POSIX mkdir -p command)

-   **parents:**(_boolean_) If parents is `True`, any missing parents of this path are
    created as needed. Otherwise, if the parent is absent, `FileNotFoundError` is raised.

-   **exist_ok:** (_boolean_) If `False`, FileExistsError is raised if the target directory
    already exists. If `True`, FileExistsError is ignored.

```python
from pathlib import Path

dir_path = Path("src/other/side")
dir_path.mkdir(parents=True)
```

#### Path.open(mode='r', buffering=-1, encoding=None, errors=None, newline=None)

This is same as the built in `open` function.

```python
from pathlib import Path

with Path("src/module.py") as f:
    contents = open(f, "r")
    for line in contents:
        print(line)
```

```txt
>>> from contextlib import contextmanager
... from time import time
... ...
```

#### Path.rename(target)

Renames this file or directory to the given target and returns a new Path instance pointing
to target. This will raise `FileNotFoundError` if the file is not found.

```python
from pathlib import Path

file_path = Path("src/stuff/submodule.py")
file_path.rename(file_path.parent / "anothermodule.py")
```

```txt
>>> PosixPath('src/stuff/anothermodule.py')
```

#### Path.replace(target)

Replaces a file or directory to the given target. Returns the new path instance.

```python
from pathlib import Path

file_path = Path("src/stuff/anothermodule.py")
file_path.replace(file_path.parent / "Dockerfile")
```

```txt
>>> PosixPath('src/stuff/Dockerfile')
```

#### Path.resolve(strict=False)

Make the path absolute, resolving any symlinks. A new path object is returned. If strict is
`True` and the path doesn't exist, `FileNotFoundError` will be raised.

```python
from pathlib import Path

file_path = Path("src/./stuff/Dockerfile")
file_path.resolve()
```

```txt
>>> PosixPath('/home/rednafi/code/demo/src/stuff/Dockerfile')
```

#### Path.rmdir()

Removes a path pointing to a file or directory. The directory must be empty, otherwise,
`OSError` is raised.

```python
from pathlib import Path

file_path = Path("src/stuff")
file_path.rmdir()
```

## So, should you use it?

Pathlib was introduced in python 3.4. However, if you are working with python 3.5 or
earlier, in some special cases, you might have to convert `pathlib.Path` objects to regular
strings. But since python 3.6, `Path` objects work almost everywhere you are using
stringified paths. Also, the `Path` object nicely abstracts away the complexity that arises
while working with paths in different operating systems.

The ability to manipulate paths in an OO way and not having to rummage through the massive
`os` or `shutil` module can make path manipulation a lot less painful.

[^1]: [Replace os.path with pathlib](https://code.djangoproject.com/ticket/29983)

[^2]:
    [pathlib - Object-oriented filesystem paths](https://docs.python.org/3/library/pathlib.html)
    [^2]

[^3]:
    [Python 3's pathlib Module: Taming the File System](https://realpython.com/python-pathlib/)
    [^3]

[^4]:
    [Why you should be using pathlib](https://treyhunner.com/2018/12/why-you-should-be-using-pathlib/#The_os_module_is_crowded)
    [^4]

[image_1]: https://blob.rednafi.com/static/images/pathlib/img_1.png
