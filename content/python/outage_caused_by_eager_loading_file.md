---
title: Dissecting an outage caused by eager-loading file content
date: 2022-10-14
tags:
    - Python
    - Incident Post-mortem
---

Python makes it freakishly easy to load the whole content of any file into memory and
process it afterward. This is one of the first things that's taught to people who are new to
the language. While the following snippet might be frowned upon by many, it's definitely not
uncommon:

```python
# src.py

with open("foo.csv", "r") as f:
    # Load the whole content of the file as a string in memory and return it.
    f_content = f.read()

    # ...do your processing here.
    ...
```

Adopting this pattern as the default way of handling files isn't the most terrible thing in
the world for sure. Also, this is often the preferred way of dealing with image files or
blobs. However, overzealously loading file content is only okay as long as the file size is
smaller than the volatile memory of the working system.

_Moreover, you'll need to be extra careful if you're accepting files from users and running
further procedures on the content of those files. Indiscriminantly loading up the full
content into memory can be dangerous as it can cause OOM errors and crash the working
process if the system runs out of memory while processing a large file. This simple overlook
was the root cause of a major production incident at my workplace today._

The affected part of our primary Django monolith asks the users to upload a CSV file to a
panel, runs some procedures on the content of the file, and displays the transformed rows in
a paginated HTML table. Since the application is primarily used by authenticated users and
we knew the expected file size, there wasn't any guardrail that'd prevent someone from
uploading a humongous file and crashing down the whole system. To make things worse, the
associated background function in the Django view was buffering the entire file into memory
before starting to process the rows. Buffering the entire file surely makes the process a
little faster but at the cost of higher memory usage.

Although we were using background processes to avoid chugging files in the main server
process, that didn't help when the users suddendly started to send large CSV files in
parallel. The workers were hitting OOM errors and getting restarted by the process manager.
In our particular case, we didn't have much reason to buffer the whole file before
processing. Apparently, the naive way scaled up pretty gracefully and we didn't pay much
attention since no one was uploading file that our server instances couln't handle. We were
storing the incoming file in a `models.FileField` type attribute of a Django model. When a
user uploads a CSV file, we'd:

-   Open the file in binary mode via the `open(filepath, "rb")` callable.
-   Buffer the whole file in memory and transform the binary content into a unicode string.
-   Pass the stringified file-like object to `csv.DictReader` to load that as a CSV file.
-   Apply transformation on the rows line by line and render the HTML table.

This is how the code looks:

```python
# src.py

import csv
import io

# Django mandates us to open the file in binary mode.
with model_instance.file.open(mode="rb") as f:
    reader = csv.DictReader(
        io.StringIO(f.read().decode(errors="ignore", encoding="utf-8")),
    )

    with row in reader:
        # ... data processing goes here.
```

The `csv.DictReader` callable only accepts a file-like object that's been opened in text
mode. However, Django's `FileField` type doesn't make any assumptions about the file
content. It mandates us to open the file in binary mode and then decode it if necessary. So,
we open the file in binary mode with `model_instance.file.open(mode="rb")` which returns an
`io.BufferedReader` type file object. This file-like object can't be passed directly to the
`csv.DictReader` because a byte stream doesn't have the concept of EOL and the CSV reader
need that to know where a row ends. As a consequence, the `csv.DictReader` expects a
file-like object opened in text mode where the rows are explicitly delineated by
platform-specific EOLs like `\n` or `\n\r`.

To solve this, we load the content of the file in memory with `f.read()` and decode it by
calling `.decode()` on the result of the preceding operation. Then we create an in-memory
text file-like buffer by passing the decoded string to `io.StringIO`. Now the CSV reader can
consume this transformed file-like object and build dictionaries of rows off of that.
Unfortunately, this stringified file buffer stays alive in the memory throughout the entire
lifetime of the processor function. Imagine 100s of large CSV files getting thrown at the
workers that execute the above code snippet. You see, at this point, overwhelming the
background workers doesn't seem too difficult.

When our workers started to degrade in production and the alerts went bonkers, we began
investigating the problem. After pinpointing the issue, we immediately responded to it by
vertically scaling up the machines. The surface area of this issue was quite large and we
didn't want to hotfix it in fear of triggering inadvertent regressions. Once we were out of
the woods, we started patching the culprit.

The solution to this is quite simple—convert the binary file-like object into a text
file-like object without buffering everything in memory and then pass the file to the CSV
reader. We were already processing the CSV rows in a lazy manner and just removing
`f.read()` fixed the overzealous buffering issue. The corrected code snippet looks like
this:

```python
# src.py

import csv
import io

# Django mandates us to open the file in binary mode.
with model_instance.file.open(mode="rb") as f:
    reader = csv.DictReader(
        io.TextIOWrapper(f, errors="ignore", encoding="utf-8"),
    )

    with row in reader:
        # ... data processing goes here.

```

Here, `io.TextIOWrapper` wraps the binary file-like object in a way that makes it behave as
if it were opened in text mode. In fact when you open a file in text mode, the native
implementation of `open` returns a file-like object wrapped in `io.TextIOWrapper`. You can
find more details about the implementation[^1] of `open` in PEP-3116[^2].

The `csv.DictReader` callable can consume this transformed file-like object without any
further modifications. Since we aren't calling `f.read()` anymore, no overzealous content
buffering is going on here and we can lazily ask for new rows from the `reader` object as we
sequentially process them.

[^1]: [open](https://peps.python.org/pep-3116/#the-open-built-in-function)

[^2]: [New I/O - PEP-3116](https://peps.python.org/pep-3116/)

[^3]:
    [How to use python csv.DictReader with a binary file?](https://stackoverflow.com/questions/51152023/how-to-use-python-csv-dictreader-with-a-binary-file-for-a-babel-custom-extract)
    [^3]
