---
title: Read a CSV file from s3 without saving it to the disk
date: 2022-06-26
tags:
    - Python
---

I frequently have to write ad-hoc scripts that download a CSV file from s3[^1], do some
processing on it, and then create or update objects in the production database using the
parsed information from the file. In Python, it's trivial to download any file from s3 via
boto3[^2], and then the file can be read with the `csv` module from the standard library.
However, these scripts are usually run from a separate script server and I prefer not to
clutter the server's disk with random CSV files. Loading the s3 file directly into memory
and reading its contents isn't difficult but the process has some subtleties. I do this
often enough to justify documenting the workflow here.

Along with boto3, we can leverage Python's `tempfile.NamedTemporaryFile`[^3] to directly
download the contents of the file to a temporary in-memory file. Afterward, we can do the
processing, create the objects in the DB, and delete the file once we're done. The
`NamedTemporaryFile` class can be used as a context manager and it'll delete the file
automatically when the `with` block ends.

This is quite straightforward with a simple gotcha. Here's how you'd usually download a file
from s3 and save that to a file-like object:

```py
# src.py
import boto3

s3 = boto3.client("s3")
with open("FILE_NAME", "wb") as f:
    s3.download_fileobj("BUCKET_NAME", "OBJECT_NAME", f)
```

Okay but the doc reminds us about this:

> The `download_fileobj` method accepts a writeable file-like object. The file object must
> be opened in binary mode, not text mode.

Opening the file in binary mode is an issue. The CSV reader needs the file to be opened in
text mode. This is not an issue when you download the file to disk since you can open the
file again in text mode to feed it to the CSV reader. However, we're trying to avoid saving
the file to disk and opening that again in text mode. So, you can't do this:

```py
# src.py
import boto3
import tempfile
import csv

s3 = boto3.client("s3")

with tempfile.NamedTemporaryFile("wb") as f:
    s3.download_fileobj("BUCKET_NAME", "OBJECT_NAME", f)

    # The csv file. This will raise an error since csv.DictReader
    # expects a file opened in text mode; not binary mode.
    csv_reader = csv.DictReader(f)
    for row in csv_reader:
        # ... do processing
        ...
```

The above snippet won't work because:

- The file-like object is opened in binary mode but the `csv.DictReader` expects the file
  pointer to be opened in text mode. So, it'll raise an error.

- Even if you fixed that, the CSV reader wouldn't be able to read anything since the file
  currently only allows writing in binary mode, not reading.

- Even if you fixed the second issue, the content of the CSV file would be empty. That's
  because after boto3 downloads and saves the file to the file object, it sets the file
  handle to the end of the file. So loading the content from there would result in an empty
  file. Here's how I fixed all three of these problems:

```py
# src.py
import boto3
import tempfile
import csv
import io

BUCKET_NAME = "foo-bucket"
OBJECT_NAME = "foo-file.csv"

s3 = boto3.client("s3")

# 'w+b' allows both reading from and writing to the file.
with tempfile.NamedTemporaryFile("w+b") as f:
    s3.download_fileobj(BUCKET_NAME, OBJECT_NAME, f)

    # This sets the file handle back to the beginning of the file.
    # Without this, the loaded file will show no content.
    f.seek(0)

    # Here, 'io.TextIOWrapper' is converting the binary content of
    # the file to be compatible with text content.
    csv_reader = csv.DictReader(
        io.TextIOWrapper(f, encoding="utf-8"),
    )

    # Now you're good to go.
    for row in csv_reader:
        # ... do processing
        ...
```

You can see that the snippet first opens a temporary file in `w+b` mode which allows both
binary read and write operations. Then it downloads the file from s3 and saves it to the
file-like object.

Once the download is finished, the file handle is placed at the bottom of the file. So,
we'll need to call `f.seek(0)` to place the handle at the beginning of the file; otherwise,
our read operation will yield no content. Also, since the currently opened file object only
allows binary read and write operations, we'll need to convert it to a text file object
before passing it to the CSV reader. The `io.TextIOWrapper` class does exactly that. Once
the file object is in text mode, we pass it to the CSV reader and do further processing.

[^1]: [AWS s3](https://aws.amazon.com/s3/)

[^2]: [boto3](https://boto3.amazonaws.com/v1/documentation/api/latest/index.html)

[^3]:
    [NamedTemporaryFile](https://docs.python.org/3/library/tempfile.html#tempfile.NamedTemporaryFile)

[^4]:
    [How to use Python csv.DictReader with a binary file?](https://stackoverflow.com/questions/51152023/how-to-use-python-csv-dictreader-with-a-binary-file-for-a-babel-custom-extract)
    [^4]
