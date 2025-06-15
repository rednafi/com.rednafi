---
title: Save models with update_fields for better performance in Django
date: 2022-11-09
tags:
    - Python
    - Django
    - TIL
---

TIL that you can specify `update_fields` while saving a Django model to generate a leaner
underlying SQL query. This yields better performance while updating multiple objects in a
tight loop. To test that, I'm opening an IPython shell with
`python manage.py shell -i ipython` command and creating a few user objects with the
following lines:

```py
In [1]: from django.contrib.auth import User

In [2]: for i in range(1000):
   ...:     fname, lname = f'foo_{i}', f'bar_{i}'
   ...:     User.objects.create(
   ...:         first_name=fname, last_name=lname, username=f'{fname}-{lname}')
   ...:
```

Here's the underlying query Django generates when you're trying to save a single object:

```py
In [3]: from django.db import reset_queries, connections

In [4]: reset_queries()

In [5]: user_0 = User.objects.first()

In [6]: user_0.first_name = 'foo_updated'

In [7]: user_0.save()

In [8]: connection.queries

```

This will print:

```txt
[
    ...,
    {
        "sql": 'UPDATE "auth_user"
        SET "password" = \'\', "last_login" = NULL, "is_superuser" = 0,
        "username" = \'foo_0-bar_0\', "first_name" = \'foo_updated\',
        "last_name" = \'bar_0\', "email" = \'\', "is_staff" = 0,
        "is_active" = 1, "date_joined" = \'2022-11-09 22:27:39.291676\'
        WHERE "auth_user"."id" = 1002',
        "time": "0.009",
    },
]
```

If you inspect the query, you'll see that although we're only updating the `first_name`
field on the `user_0` object, Django is generating a query that updates all the underlying
fields on the object. The SQL query always passes the pre-existing values of the fields that
weren't touched. This might seem trivial, but what if the model consisted of 20 fields and
you need to call `save()` on it frequently? At a certain scale the database query that
updates all of your columns every time you call `save()` can start becoming expensive.

Specifying `update_fields` inside the `save()` method can make the query leaner. Consider
this:

```py
In[9]: reset_queries()

In[10]: user_0.first_name = "foo_updated_again"

In[11]: user_0.save(update_fields=["first_name"])

In[12]: connection.queries
```

This prints:

```txt
[
    {'sql': 'UPDATE "auth_user" SET "first_name" = \'changed_again\'
      WHERE "auth_user"."id" = 1002',
      'time': '0.008'
    }
]
```

You can see this time, Django generates a SQL that only updates the specific field we want
and doesn't send any redundant data over the wire. The following snippet quantifies the
performance gain while updating 1000 objects in a tight loop:

```py
# src.py
import os
import time

import django

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "mysite.settings")
django.setup()

from django.contrib.auth.models import User

# Create 1000 users.
for i in range(1000):
    User.objects.create_user(
        first_name=f"foo_{i}",
        last_name=f"bar_{i}",
        username=f"foo_{i}-bar_{i}",
    )

############### Update all users with '.save()' ###############
s1 = time.perf_counter()

for i, user in zip(range(1000), User.objects.all()):
    user.first_name = f"foo_updated_{i}"
    user.save()

e1 = time.perf_counter()
t1 = e1 - s1

print(f"User.save(): {t1:.2f}s")
###############################################################

###### Update all users with '.save(update_fields=[...])'######
s2 = time.perf_counter()

for i, user in zip(range(1000), User.objects.all()):
    user.first_name = f"foo_updated_again_{i}"
    user.save(update_fields=["first_name"])

e2 = time.perf_counter()
t2 = e2 - s2

print(f"User.save(update_fields=[...]): {t2:.2f}s")
###############################################################

print(
    f"User.save(update_fields=[...] is {t1 / t2:.2f}x faster than User.save()"
)
```

Running this script will print the following:

```txt
User.save(): 1.86s
User.save(update_fields=[...]): 1.77s
User.save(update_fields=[...] is 1.05x faster than User.save()
```

You can see that `User.save(updated_fields=[...])` is a tad bit faster than plain
`User.save`.

## Should you always use it?

Probably not. While the performance gain is measurable when you're updating multiple objects
in a loop, it's quite negligible if the object count is low. Also, this adds maintenance
overhead as any time you change the model, you'll have to remember to keep the
`Model.save(update_fields=[...])` in sync. If you forget to add a field to the
`update_fields`, Django will silently ignore the incoming data against that field and data
will be lost.

## References

[^1]:
    [Specifying which fields to save - Django docs](https://docs.djangoproject.com/en/4.1/ref/models/instances/#specifying-which-fields-to-save)
    [^1]

[^2]:
    [Save your Django models using update_fields for better performance - Reddit](https://www.reddit.com/r/django/comments/nynfab/save_your_django_models_using_update_fields_for/)
    [^2]
