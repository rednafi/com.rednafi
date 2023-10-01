---
title: How not to run a script in Python
date: 2022-03-16
tags:
    - Python
---

When I first started working with Python, nothing stumped me more than how bizarre Python's
import system seemed to be. Often time, I wanted to run a module inside of a package with
the `python src/sub/module.py` command, and it'd throw an `ImportError` that didn't make any
sense. Consider this package structure:

```txt
src
├── __init__.py
├── a.py
└── sub
    ├── __init__.py
    └── b.py
```

Let's say you're importing module `a` in module `b`:

```python
# b.py
from src import a

...
```

Now, if you try to run module `b.py` with the following command, it'd throw an import error:

```sh
python src/sub/b.py
```

```txt
Traceback (most recent call last):
  File "/home/rednafi/canvas/personal/reflections/src/sub/b.py", line 2, in <module>
    from src import a
ModuleNotFoundError: No module named 'src'
```

What! But you can see the `src/a.py` module right there. Why can't Python access the module
here? Turns out Python puts the path of the module that you're trying to access to the top
of the `sys.path` stack. Let's print the `sys.path` before importing module `a` in the
`src/sub/b.py` file:

```python
# b.py
import sys

print(sys.path)
from src import a
```

Now running this module with `python src/sub/b.py` will print the following:

```txt
['/home/rednafi/canvas/personal/reflections/src/sub', '/usr/lib/python310.zip', '/usr/
lib/python3.10', '/usr/lib/python3.10/lib-dynload', '/home/rednafi/canvas/personal/
reflections/.venv/lib/python3.10/site-packages']

Traceback (most recent call last):
  File "/home/rednafi/canvas/personal/reflections/src/sub/b.py", line 5, in <module>
    from src import a
ModuleNotFoundError: No module named 'src'
```

From the first section of the above output, it's evident that Python looks for the imported
module in the `src/sub/` directory, not in the root directory from where the command is
being executed. That's why it can't find the `a.py` module because it exists in a directory
above the `sys.path`'s first entry. To solve this, you should run the module with the `-m`
switch as follows:

```sh
python -m src.sub.b
```

This will not raise the import error and return the following output:

```txt
['/home/rednafi/canvas/personal/reflections', '/usr/lib/python310.zip',
'/usr/lib/python3.10', '/usr/lib/python3.10/lib-dynload',
'/home/rednafi/canvas/personal/reflections/.venv/lib/python3.10/site-packages']
```

Here, the first entry denotes the root directory from where the script is being run
from. Voila, problem solved!

[^1]: [Don't run python my/script.py](https://www.youtube.com/watch?v=hgCVIa5qQhM) [^1]
