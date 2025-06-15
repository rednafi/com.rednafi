---
title: Stream process a CSV file in Python
date: 2022-07-01
tags:
    - Python
    - Networking
---

A common bottleneck for processing large data files is—memory. Downloading the file and
loading the entire content is surely the easiest way to go. However, it's likely that you'll
quickly hit OOM errors. Often time, whenever I have to deal with large data files that need
to be downloaded and processed, I prefer to stream the content line by line and use multiple
processes to consume them concurrently.

For example, say, you have a CSV file containing millions of rows with the following
structure:

```txt
---------------------------------------
|        a        |           b       |
---------------------------------------
0.902210680227088 | 0.236522024407207 |
0.424413804319515 | 0.400788559643378 |
0.601611774624256 | 0.4992389256938   |
0.332269908707654 | 0.72328094652184  |
---------------------------------------
```

Here, let's say you need to download the file from some source and run some other heavy
tasks that depends on the data from the file. To avoid downloading the file to the disk, you
can stream and read the content line by line directly from the network. While doing so, you
may want to trigger multiple other tasks that can run independent of the primary process.

At my workplace, I often have to create objects in a relational database using the
information in a CSV file. The idea here is to consume the information in the CSV file
directly from the network and create the objects in the database. This database object
creation task can be offloaded to a separate process outside of the main process that's
streaming the file contents.

Since we're streaming the content from the network line by line, there should be zero disk
usage and minimal memory footprint. Also, to speed up the consumption, we'll fork multiple
OS processes. To put in concisely, we'll need to perform the following steps:

- Stream a single row from the target CSV file.
- Write the content of the row in an in-memory string buffer.
- Parse the file buffer with `csv.DictReader`.
- Collect the dict the contains the information of the parsed row.
- Yield the dict.
- Flush the buffer.
- Another process will collect the yielded dict and consume that outside of the main
  process.
- And continue the loop for the next row.

The following snippet implements the workflow mentioned above:

```py
# src.py
from __future__ import annotations

import csv
import io
import multiprocessing as mp
import time
from operator import itemgetter
from typing import Iterator, Mapping

import httpx


def stream_csv(url: str) -> Iterator[Mapping[str, str | int]]:
    """Return an iterator that yields a dict representing a single
    row of a CSV file.

    Args:
        url (str): URL that holds the CSV file

    Yields:
        Iterator[dict[str, str]]: Returns a generator that yields a
        dict.
    """
    with httpx.Client() as client:
        # Make a streaming HTTP request.
        with client.stream("GET", url, follow_redirects=True) as r:
            # Create instance of an in-memory file. We save the row
            # of the incoming CSV file here.
            f = io.StringIO()

            # The content of the source CSV file is iterated
            # line by line.
            lines = r.iter_lines()

            # Ignore the header row. This is the first row.
            next(lines)

            # Enumerate allows us to attach a line number to each row.
            # We start from two since header is the first line.
            for lineno, line in enumerate(lines, 2):
                # Write one line to the in-memory file.
                f.write(line)

                # Seek sends the file handle to the top of the file.
                f.seek(0)

                # We initiate a CSV reader to read and parse each line
                # of the CSV file
                reader = csv.DictReader(f, fieldnames=("a", "b"))

                # Since we know that there's only one row in the reader
                # we just call 'next' on it to get the parsed dict.
                # The row dict looks like this:
                # {'a': '0.902210680227088', 'b': '0.236522024407207'}
                row = next(reader)

                # Add a line number to the dict. It makes the dict looks
                # like this:
                # {
                #   'a': '0.902210680227088',
                #   'b': '0.236522024407207',
                #   'lineno': 2
                # }
                row["lineno"] = lineno  # type: ignore

                # Yield the row. This allows us to call the function
                # in a lazy manner.
                yield row

                # The file handle needs to be set to the top before
                # cleaning up the buffer.
                f.seek(0)

                # Clean up the buffer.
                f.flush()


def process_row(row: Mapping[str, str | int]) -> None:
    """Consume a single row and do some work.

    Args:
        row (dict[str, str]): Represents a single parsed row of
        a CSV file.
    """

    a, b = itemgetter("a", "b")(row)
    float_a, float_b = float(a), float(b)

    # Do some processing.
    print(
        f"Processed row {row['lineno']}:"
        f"a={float_a:.15f}, b={float_b:.15f}",
    )

    # Mimick some other heavy processing.
    time.sleep(2)


if __name__ == "__main__":
    # fmt: off
    csv_url = (
        "https://github.com/rednafi/reflections/files" \
        "/9006167/foo.csv",
    )

    with mp.Pool(4) as pool:
        for res in pool.imap(process_row, stream_csv(csv_url)):
            pass
```

The first function `stream_csv` accepts a URL that points to a CSV file. In this case, the
URL used here points to a real CSV file hosted on GitHub. HTTPx[^1] allows you to make a
streaming[^2] GET request and iterate through the contents of the file without fully
downloading it to the disk.

Inside the `client.stream` block, we've created an in-memory file instance with
`io.StringIO`. This allows us to write the streamed content of the source CSV file to the
in-memory file. Then we pull one row from the source file, write it to the in-memory buffer,
and pass the in-memory file buffer over to the `csv.DictReader` class.

The `DictReader` class will parse the content of the row and emit a `reader` object. Running
`next` on the `reader` iterator returns a dictionary with the parsed content of the row. The
parsed content for the first row of the example CSV looks like this:

```py
{
    "a": "0.902210680227088",
    "b": "0.236522024407207",
    "lineno": 1,
}
```

Next, the `process_row` function takes in the data of a single row as a dict like the one
above and does some processing on that. For demonstration, currently, it just prints the
values of the rows and then sleeps for two seconds.

Finally, in the `__main__` block, we fire up four processes to apply the `process_row`
function to the output of the `stream_csv` function. Running the script will print the
following output:

```txt
Processed row 2:a=0.902210680227088, b=0.236522024407207
Processed row 3:a=0.424413804319515, b=0.400788559643378
Processed row 4:a=0.601611774624256, b=0.499238925693800
Processed row 5:a=0.332269908707654, b=0.723280946521840 # Sleep 2 sec
Processed row 6:a=0.024648655864128, b=0.585924680177486
Processed row 7:a=0.116178678991780, b=0.027524894156040
Processed row 8:a=0.313182023389972, b=0.373896338507016
Processed row 9:a=0.252893754537173, b=0.809821115129037 # Sleep 2 sec
Processed row 10:a=0.770407022765901, b=0.021249180774146
...
...
...
```

Since we're forking 4 processes, the script will print four items, and then it'll pause
roughly for 2 seconds before moving on. If we were using a single process, the script would
wait for 2 seconds after printing every row. By increasing the number of processes, you can
speed up the consumption rate. Also, if the consumer tasks are lightweight, you can open
multiple threads to consume them.

[^1]: [HTTPx](https://www.python-httpx.org/)

[^2]: [Streaming responses](https://www.python-httpx.org/quickstart/#streaming-responses)
