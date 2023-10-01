---
title: Manipulating text with query expressions in Django
date: 2023-01-07
tags:
    - Python
    - Django
---

I was working with a table that had a similar (simplified) structure like this:

```txt
|               uuid               |         file_path         |
|----------------------------------|---------------------------|
| b8658dfc3e80446c92f7303edf31dcbd | media/private/file_1.pdf  |
| 3d750874a9df47388569a23c559a4561 | media/private/file_2.csv  |
| d177b7f7d8b046768ab65857451a0354 | media/private/file_3.txt  |
| df45742175d7451dad59761f15653d9d | media/private/image_1.png |
| a542966fc193470dab84351c15523042 | media/private/image_2.jpg |
```

Let's say the above table is represented by the following Django model:

```python
from django.db import models


class FileCabinet(models.Model):
    uuid = models.UUIDField(
        primary_key=True, default=uuid.uuid4, editable=False
    )
    file_path = models.FileField(upload_to="files/")
```

I needed to extract the file names with their extensions from the `file_path` column and
create new paths by adding the prefix `dir/` before each file name. This would involve
stripping everything before the file name from a file path and adding the prefix, resulting
in a list of new file paths like this: `['dir/file_1.pdf', ..., 'dir/image_2.jpg']`.

Using Django ORM and some imperative Python code you could do the following:

```python
...

# This will give you a queryset with the file paths.
# e.g. <QuerySet ['media/private/file_1.pdf', ... ]>
file_paths = FileCabinet.objects.values_list("file_path", flat=True)

# Now the file names can be collected in a list via a listcomp.
# This will return: ["dir/file_1.pdf", ..., "dir/image_2.jpg"]
file_paths_new = [
    f"dir/{file_path.split('/')[-1]}" for file_path in file_paths
]

...
```

Here, we use the `FileCabinet` model to make a query and obtain the file paths. We then use
Python to split the file paths and extract the file names, and add the prefix `dir/` to
create the new paths. While this approach is relatively simple, it can be slow and
resource-intensive if the size of the working dataset is large. This is because the entire
working dataset is loaded into memory and the text manipulation is performed in Python.

To improve performance and efficiency, Django offers a declarative approach using
expressions. These expressions allow you to offload operations like this to the database,
which can be significantly faster and less resource-intensive than the imperative approach,
especially for larger querysets. Here's how you can achieve the same result in a declarative
manner:

```python
...

from django.db.models import F, Value
from django.db.models.functions import (
    Concat,
    Reverse,
    Right,
    StrIndex,
)


file_cabinet = polls_models.FileCabinet.objects.annotate(
    last_occur=StrIndex(Reverse(F("file_path")), Value("/")),
    file_name=Right(F("file_path"), F("last_occur") - 1),
    file_path_new=Concat(Value("dir/"), F("file_name")),
)

...
```

You can see the new file paths by inspecting the `file_cabinet` queryset as follows:

```python
file_paths_new = file_cabinet.values_list("file_path_new", flat=True)
```

This will give you the following queryset:

```txt
<QuerySet
    ['dir/file_1.pdf',
    'dir/file_2.csv',
    'dir/file_3.txt',
    'dir/image_1.png',
    'dir/image_2.jpg']
>
```

Now, let's step through the each of the ORM functionality that was levereged here:

The `annotate` function is being used to add additional information to each returned
`FileCabinet` object. This function allows you to specify additional fields that should be
calculated and included in the returned queryset. Inside the annotation method, we use `F`
objects to reference a model field within the query. They can be used to refer to a field's
value in the context of an update or filter, rather than referring to the actual field
itself.

Three fields are being added to the `FileCabinet` objects: `last_occur`, `file_name`, and
`file_path_new`.

`last_occur` is being calculated by using the `StrIndex` function. This function takes two
arguments: the string to search and the string to search for. In this case, the string being
searched is the `file_path` field, but it has been passed through the `Reverse` function to
reverse the string. This is done so that the `StrIndex` function starts searching from the
end of the string, rather than the beginning. The second argument to `StrIndex` is the
string to search for, which in this case is `/`. The `StrIndex` function returns the
position of the first occurrence of the search string in the main string.

`file_name` is being calculated by using the `Right` function. This function takes two
arguments: the string to extract from and the number of characters to extract. In this case,
the string being extracted from is the `file_path` field, and the number of characters to
extract is specified by the `last_occur` field. The `last_occur` field represents the
position of the last occurrence of `/` in the file_path field, so extracting the characters
from this position onwards gives us the file name with its extension. The `- 1` at the end
is used to remove the `/` character itself from the extracted string.

Finally, the `file_path_new` is constructed by using the `Concat` function. This function
takes a variable number of arguments and concatenates them together into a single string. In
this case, the `dir/` prefix is being concatenated with `file_name` field.

Perfection!


[^1]: [Do database work in the database rather than in Python](https://docs.djangoproject.com/en/4.1/topics/db/optimization/#do-database-work-in-the-database-rather-than-in-python) [^1]
[^2]: [Use StrIndex to get the last instance of a character for an annotation in Django](https://stackoverflow.com/questions/67030571/django-use-strindex-to-get-the-last-instance-of-a-character-for-an-annotation) [^2]
