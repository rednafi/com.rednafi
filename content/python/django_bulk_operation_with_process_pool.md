---
title: Bulk operations in Django with process pool
date: 2022-06-27
tags:
    - Python
    - Django
---

I've rarely been able to take advantage of Django's `bulk_create / bulk_update` APIs in
production applications; especially in the cases where I need to create or update multiple
complex objects with a script. Often time, these complex objects trigger a chain of signals
or need non-trivial setups before any operations can be performed on each of them.

The issue is, `bulk_create / bulk_update` doesn't trigger these signals or expose any hooks
to run any setup code. The Django doc mentions these caveates[^1] in detail. Here are a few
of them:

-   The model's `save()` method will not be called, and the `pre_save` and `post_save`
    signals will not be sent.
-   It does not work with child models in a multi-table inheritance scenario.
-   If the model's primary key is an `AutoField`, the primary key attribute can only be
    retrieved on certain databases (currently PostgreSQL, MariaDB 10.5+, and SQLite 3.35+).
    On other databases, it will not be set.
-   It does not work with many-to-many relationships.
-   It casts `objs` to a list, which fully evaluates objs if it's a generator. Here, `obj`
    is the iterable that passes the information necessary to create the database objects in
    a single go.

To solve this, I wanted to take advantage of Python's `concurrent.futures` module. It
exposes a similar API for both thread-based and process-based concurrency. The snippet below
creates ten thousand user objects in the database and runs some setup code before creating
each object.

```python
# script.py

from __future__ import annotations

import os
from typing import Iterable

import django

os.environ["DJANGO_SETTINGS_MODULE"] = "mysite.settings"
django.setup()

from concurrent.futures import ProcessPoolExecutor

from django.contrib.auth.models import User
from tqdm import tqdm

MAX_WORKERS = 4


def create_user_setup() -> None:
    # ... Run some heavy weight setup code here.
    pass


def create_user(username: str, email: str) -> None:
    # ... Call complex setup code here. This allows the
    # setup code to run concurrently.
    create_user_setup()
    User.objects.create(username=username, email=email)


def bulk_create_users(users: Iterable[dict[str, str]]) -> None:
    # A container for the pending future objects.
    futures = []

    # With PostgreSQL, Psycopg2 often complains about closed cursors
    # and this fixes that.
    django.db.connections.close_all()

    with ProcessPoolExecutor(max_workers=MAX_WORKERS) as executor:
        for user in users:
            future = executor.submit(create_user, **user)
            futures.append(future)

        # Wait for all the futures to complete and give the
        # user a visual feedback with tqdm progressbar.
        for future in tqdm(futures):
            future.result()

        print("done!")


if __name__ == "__main__":
    users = (
        {
            "username": f"{i}",
            "email": f"{i}@{i}.com",
        }
        for i in range(10_000)
    )

    bulk_create_users(users=users)
```

Here, the `create_user_setup` function runs some complex setup code before the creation of
each user object. We wrap the user creation process in a function named `create_user` and
call the setup code in that. This allows us to run the complex setup code concurrently. The
magic happens in the `bulk_create_users` function. It takes in an iterable containing the
information to create the users and runs the `create_user` functions concurrently.

The `ProcessPoolExecutor` forks 4 processes and starts consuming the iterable. We use the
`executor.submit` method for maximum flexibility. This allows us to further process the
returned value from the `create_user` function (in this case it's `None`). Running this
snippet will also show a progress bar as the processes start chewing through the work.

You can also try experimenting with `ThreadPoolExecutor`, `executor.map`, and `chunksize`. I
didn't choose `executor.map` because it's tricky to show the progress bar with `map`. Also,
I encountered some `psycopg2` errors in a PostgreSQL database whenever I switched to the
`ThreadPoolExecutor`. Another gotcha is that `psycopg` can complain about closed cursors and
closing the database connection before running each process is a way to avoid that. Notice
that the script above runs `django.db.connections.close_all()` before entering into the
`ProcessPoolExecutor` context manager.

This appoach will run the `pre_save` and `post_save` signals which allows me to take
advantage of these hooks without losing the ability of being able to perform concurrent row
operations.

## Breadcrumbs

Example shown here performs a trivial task of creating 10k user objects. In cases like this,
you might find that a simple `for-loop` might be faster. Always run at least a rudimentary
benchmark before adding concurrency to your workflow.

Also, this approach primarily targets ad-hoc scripts and tasks. I don't recommend forking
multiple processes in your views or forms since Python processes aren't cheap.

[^1]:
    [Caveats of bulk_create](https://docs.djangoproject.com/en/dev/ref/models/querysets/#bulk-create)

[^2]: [concurrent.futures](https://docs.python.org/3/library/concurrent.futures.html) [^2]
