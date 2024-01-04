---
title: Faster bulk_update in Django
date: 2022-11-30
tags:
    - Python
    - Django
    - Database
---

Django has a `Model.objects.bulk_update` method that allows you to update multiple objects
in a single pass. While this method is a great way to speed up the update process,
oftentimes it's not fast enough. Recently, at my workplace, I found myself writing a script
to update half a million user records and it was taking quite a bit of time to mutate them
even after leveraging bulk update. So I wanted to see if I could use `multiprocessing` with
`.bulk_update` to quicken the process even more. Turns out, yep I can!

Here's a script that creates 100k users in a PostgreSQL database and updates their usernames
via vanilla `.bulk_update`. Notice how we're timing the update duration:

```python
# app_name/vanilla_bulk_update.py
import os

import django

# This allows us to run this module as a script inside a Django app.
os.environ.setdefault("DJANGO_SETTINGS_MODULE", "mysite.settings")
django.setup()

import time

from django.contrib.auth.models import User

# Delete the previous users, if there's any.
User.objects.all().delete()

# Create 100k users.
users = User.objects.bulk_create(
    (User(username=f"user_{i}") for i in range(100_000)),
)

# Start time.
s1 = time.perf_counter()

# Update all the users' usernames to use upper case.
for user in users:
    user.username = user.username.upper()

# Save all the users. The batch_size determines how many records will
# be saved at once.
User.objects.bulk_update(users, ["username"], batch_size=1_000)

# End time.
e1 = time.perf_counter()

# Print the time taken.
print(f"Time taken to update 100k users: {e1 - s1} seconds.")

# Print a few usernames to see that the script has changed them as expected.
print("Updated usernames:")
print("===================")
for username in User.objects.values_list("username", flat=True)[:5]:
    print(username)
```

This can be executed as a script like this:

```sh
python -m app_name.vanilla_bulk_update
```

It'll return:

```txt
Time taken to update 100k users: 9.220380916005524 seconds.
Updated usernames:
===================
USER_99840
USER_99841
USER_99842
USER_99843
USER_99844
```

A little over 9 seconds isn't too bad for 100k users but we can do better. Here's how I've
updated the above script to make it 4x faster:

```python
# app_name/multiprocessing_bulk_update.py
import os

import django

# This allows us to run this module as a script inside a Django app.
os.environ.setdefault("DJANGO_SETTINGS_MODULE", "mysite.settings")
django.setup()

import multiprocessing as mp
import time

from django.contrib.auth.models import User

MAX_WORKERS = 2 * mp.cpu_count() - 1
CHUNK_SIZE = 1_000


def main():
    # Delete the previous users, if there's any.
    User.objects.all().delete()

    # Create 100k users.
    users = User.objects.bulk_create(
        (User(username=f"user_{i}") for i in range(100_000))
    )

    # Start time.
    s1 = time.perf_counter()

    # Mutate the usernames to use upper case.
    for user in users:
        user.username = user.username.upper()

    # Split the users into chunks for each process to work on. This returns
    # [[USER_0, USER_1, USER_2, ...], [USER_3, USER_4, USER_5, ...], ...]
    user_chunks = (
        users[i : i + CHUNK_SIZE] for i in range(0, len(users), CHUNK_SIZE)
    )

    # Close the connection before forking.
    django.db.connections.close_all()

    # Create a pool of processes and run the update_users function on
    # each chunk.
    with mp.Pool(MAX_WORKERS) as pool:
        pool.map(update_users, user_chunks, chunksize=10)

    # End time.
    e1 = time.perf_counter()

    # Print the time taken.
    print(
        "Time taken to update 100k users with multiprocessing: "
        f"{e1 - s1} seconds."
    )

    # Print a few usernames to see that the script has changed them
    # as expected.
    print("Updated usernames:")
    print("===================")
    for username in User.objects.values_list("username", flat=True)[:5]:
        print(username)


def update_users(user_chunk):
    # The batch_size determines how many records will be saved at once.
    User.objects.bulk_update(user_chunk, ["username"], batch_size=CHUNK_SIZE)


if __name__ == "__main__":
    main()
```

This script divides the updated user list into a list of multiple user chunks and assigns
that to the `user_chunks` variable. The `update_users` function takes a single user chunk
and runs `.bulk_update` on that. Then we fork a bunch of processes and run the
`update_users` function over the `user_chunks` via `multiprocessing.Pool.map`. Each process
consumes `10` chunks of users in a single go—determined by the `chunksize` parameter of the
`pool.map` function. Running the updated script will give you similar output as before but
with a much smaller runtime:

```sh
python -m app_name.multiprocessing_bulk_update
```

This will print the following:

```txt
Time taken to update 100k users with multiprocessing: 2.2682724999976926 seconds.
Updated usernames:
===================
USER_960
USER_961
USER_962
USER_963
USER_964
```

Whoa! This updated the records in under 2.5 seconds. Quite a bit of performance gain there.

> This won't work if you're using SQLite database as your backend since SQLite doesn't
> support concurrent writes from multiple processes. Trying to run the second script with
> SQLite backend will incur a database error.

[^1]:
    [Django bulk_update](https://docs.djangoproject.com/en/dev/ref/models/querysets/#bulk-update)
    [^1]

[^2]:
    [Using a pool of forked workers](https://docs.python.org/3/library/multiprocessing.html#using-a-pool-of-workers)
    [^2]
