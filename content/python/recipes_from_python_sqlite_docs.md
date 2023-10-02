---
title: Recipes from Python SQLite docs
date: 2022-09-11
tags:
    - Database
    - Python
---

While going through the documentation of Python's `sqlite3`[^1] module, I noticed that it's
quite API-driven, where different parts of the module are explained in a prescriptive
manner. I, however, learn better from examples, recipes, and narratives. Although a few good
recipes already exist in the docs, I thought I'd also enlist some of the examples I tried
out while grokking them.

## Executing individual statements

To execute individual statements, you'll need to use the `cursor_obj.execute(statement)`
primitive.

```python
# src.py
import sqlite3

conn = sqlite3.connect(":memory:")
c = conn.cursor()

with conn:
    c.execute(
        """
    create table if not exists
        stat (id integer primary key, cat text, score real);
    """
    )

    c.execute("""insert into stat (cat, score) values ('a', 1.0);""")
    c.execute("""insert into stat (cat, score) values ('b', 2.0);""")
    result = c.execute("""select * from stat;""").fetchall()

    print(result)
```

```txt
[(1, 'a', 1.0), (2, 'b', 2.0)]
```

## Executing batch statements

You can bundle up multiple statements and execute them in a single go with the
`cursor_obj.executemany(template_statement, (data, ...))` API.

```python
# src.py
import sqlite3

conn = sqlite3.connect(":memory:")
c = conn.cursor()

with conn:
    c.execute(
        """
    create table if not exists
        stat (id integer primary key, cat text, score real);
    """
    )

    # Data needs to be passed as an iterable of tuples.
    data = (
        ("a", 1.0),
        ("b", 2.0),
        ("c", 3.0),
    )
    c.executemany(
        "insert into stat (cat, score) values (?, ?);", data
    )
    result = c.execute("""select * from stat;""").fetchall()

    print(result)
```

```txt
[(1, 'a', 1.0), (2, 'b', 2.0), (3, 'c', 3.0)]
```

## Applying user-defined callbacks

You can define and apply arbitrary Python callbacks to different data points in an SQLite
table. There are two types of callbacks that you can apply:

-   Scalar function: A scalar function returns one value per invocation; in most cases, you
    can think of this as returning one value per row.

-   Aggregate function: In contrast, an aggregate function returns one value per group of
    rows.

### Applying user-defined scalar functions

In the following example, I've created a table called `users` with two text type
columns—`username` and `password`. Here, we define a transformation scalar function named
`sha256` that applies sha256 hashing to all the elements of the `password` column. The
function is then registered via the `connection_obj.create_function(func_name, narg, func)`
API.

```python
# src.py
import sqlite3
import hashlib

conn = sqlite3.connect(":memory:")
c = conn.cursor()


def sha256(t: str) -> str:
    return hashlib.sha256(
        t.encode("utf-8"),
        usedforsecurity=True,
    ).hexdigest()


# Register the scalar function.
conn.create_function("sha256", 1, sha256)

with conn:
    c.execute(
        """
        create table if not exists users (
            username text,
            password text
        );
    """
    )
    c.execute(
        "insert into users values (?, sha256(?));",
        ("admin", "password"),
    )
    c.execute(
        "insert into users values (?, sha256(?));",
        ("user", "otherpass"),
    )

    result = c.execute("select * from users;").fetchall()
    print(result)
```

```txt
[
    (
        'admin',
        '5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8'),
    (
        'user',
        '0da86a02c6944c679c5a7f06418bfde6bddb445de708639a3131af3682b34108'
    )
]
```

### Applying user-defined aggregate functions

Aggregate functions are defined as classes and then registered with the
`connection_obj.create_aggregate(func_name, narg, aggregate_class)` API. In the example
below, I've created a table called `series` with a single integer type column `val`. To
define an aggregate function, we'll need to write a class with two methods—`step` and
`finalize` where `step` will return the value of an intermediary progression step and
`finalize` will return the final result. Below, you can see that the aggregate function
returns a single value in the output.

```python
# src.py
import sqlite3
import hashlib

conn = sqlite3.connect(":memory:")
c = conn.cursor()


class Mult:
    def __init__(self):
        self._result = 1

    def step(self, value):
        self._result *= value

    def finalize(self):
        return self._result


# Register the aggregate class.
conn.create_aggregate("mult", 1, Mult)

with conn:
    c.execute(
        """
        create table if not exists series (
            val integer
        );
    """
    )
    c.execute("insert into series (val) values (?);", (2,))
    c.execute("insert into series (val) values (?);", (3,))

    result = c.execute("select mult(val) from series;").fetchall()
    print(result)
```

```txt
[(6,)]
```

## Printing traceback when a user-defined callback raises an error

By default, `sqlite3` will suppress the traceback of any error raised from an user-defined
function. However, you can turn on the traceback option as follows:

```python
# src
import sqlite3

sqlite3.enable_callback_tracebacks(True)

...
```

## Transforming types

Conventionally, Python `sqlite3` documentation uses the term _adaptation_ to refer to the
transformation that changes Python types to SQLite types and _conversion_ to refer to the
change in the reverse direction.

### Adapting Python types to SQLite types

To transform Python types to native SQLite types, you'll need to define a transformation
callback that'll carry out the task. Then the callback will need to be registered with the
`sqlite3.register_adapter(type, adapter_callback)` API.

Here, I've created an in-memory table called `colors` with a single text type column `name`
that refers to the name of the color. Then I register the `lambda color: color.value`
anonymous function that serializes an enum value to a text value. This allows me to pass an
enum member directly into the `cursor_obj.execute` method.

```python
# src.py
import enum
import sqlite3

conn = sqlite3.connect(":memory:")
c = conn.cursor()


class Color(enum.Enum):
    RED = "red"
    GREEN = "green"
    BLUE = "blue"


# Register an adapter to transform a Python type to an SQLite type.
sqlite3.register_adapter(Color, lambda color: color.value)

with conn:
    c.execute(
        """
        create table if not exists colors (
            name integer
        );
    """
    )
    c.execute("insert into colors (name) values (?);", (Color.RED,))
    c.execute("insert into colors (name) values (?);", (Color.GREEN,))

    result = c.execute("select name from colors;").fetchall()
    print(result)
```

```txt
[('red',), ('green',)]
```

### Converting SQLite types to Python types

Converting SQLite types to Python types works similarly to the previous section. Here, as
well, I've created the same `colors` table with a single `name` column as before. But this
time, I want to insert string values into the `name` column and get back native enum objects
from that field while performing a get query.

To do so, I've registered a converter function with the
`sqlite3.register_converter("sqlite_type_as_a_string", converter_callback)` API. Another
point to keep in mind is that you'll have to set `detect_type=sqlite3.PARSE_DECLTYPES` in
the `sqlite3.connection` method for the adaptation to work. Notice the output of the last
`select ...` statement and you'll see that we're getting enum objects in the returned list.

```python
# src.py
import enum
import sqlite3


class Color(enum.Enum):
    RED = "red"
    GREEN = "green"
    BLUE = "blue"


color_map = {v.value: v for v in Color.__members__.values()}

# Register a convert to convert text to Color enum
sqlite3.register_converter(
    "text",
    lambda v: color_map[v.decode("utf-8")],
)

conn = sqlite3.connect(
    ":memory:",
    detect_types=sqlite3.PARSE_DECLTYPES,  # Parse declaration types.
)

c = conn.cursor()

with conn:
    c.execute(
        """
        create table if not exists colors (
            name text
        );
    """
    )
    c.execute("insert into colors (name) values (?);", ("red",))
    c.execute("insert into colors (name) values (?);", ("green",))
    c.execute("insert into colors (name) values (?);", ("blue",))

    result = c.execute("select name from colors;").fetchall()
    print(result)
```

```txt
[
    (<Color.RED: 'red'>,),
    (<Color.GREEN: 'green'>,),
    (<Color.BLUE: 'blue'>,)
]
```

### Using the default adapters and converters

The `sqlite3` module also employs some default adapters and converters that you can take
advantage of without defining and registering custom transformers. For example, SQLite
doesn't have any special types to represent a date or timestamp. However, Python `sqlite3`
allows you to annotate a column with a special type and it'll automatically convert the
values of the column to a compatible type of Python object while returning the result of a
get query.

Here, I've created a table called `timekeeper` with two columns—`d` and `dt` where `d`
expects a date and `dt` expects a timestamp. So, in the table creation DDL statement, we
annotate the columns with `date` and `timestamp` types respectively. We've also turned on
column type parsing by setting
`detect_types=sqlite3.PARSE_DECLTYPES | sqlite3.PARSE_COLNAMES` in the `sqlite3.connect`
method.

Finally, notice how we're inserting `datetime.date` and `datetime.datetime` objects directly
into the table. Also, this time, the final `select ...` statement looks a bit different.
We're specifying the expected type in the `select ...` statement and it's returning native
Python objects in the returned list.

```python
# src.py
import datetime
import sqlite3
import zoneinfo

conn = sqlite3.connect(
    ":memory:",
    detect_types=sqlite3.PARSE_DECLTYPES | sqlite3.PARSE_COLNAMES,
)

with conn:
    c = conn.cursor()
    c.execute(
        """
        create table if not exists
        timekeeper (id integer primary key, d date, dt timestamp);"""
    )
    tz = zoneinfo.ZoneInfo("America/New_York")
    dt = datetime.datetime.now(tz)
    d = dt.date()

    c.execute(
        "insert into timekeeper (d, dt) values (?, ?);",
        ((d, dt)),
    )
    result = c.execute(
        """
        select
            d as "d [date]", dt as "dt [timestamp]"
            from timekeeper;"""
    ).fetchall()
    print(result)
```

```txt
[
    (
        datetime.date(2022, 9, 5),
        datetime.datetime(2022, 9, 5, 14, 59, 52, 867917)
    )
]
```

## Implementing authorization control

Sometimes you need control over what operations are allowed to be run on an SQLite database
and what aren't. The `connection_obj.set_authorizer(auth_callback)` allows you to implement
authorization control. Here, `auth_callback` takes in 5 arguments. From the docs:

> The 1st argument to the callback signifies what kind of operation is to be authorized. The
> 2nd and 3rd arguments will be arguments or None depending on the 1st argument. The 4th
> argument is the name of the database (“main”, “temp”, etc.) if applicable. The 5th
> argument is the name of the inner-most trigger or view that is responsible for the access
> attempt or None if this access attempt is directly from input SQL code. Please consult the
> SQLite documentation about the possible values for the first argument and the meaning of
> the second and third arguments depending on the first one.

You can find the list of all the supported actions here[^2]. In the following example, I'm
disallowing create table, create index, drop table, and drop index actions. To deny an
action, the `auth_callback` will have to return `sqlite3.SQLITE_DENY` and that'll raise an
`sqlite3.DatabaseError` exception whenever a user tries to execute any of the restricted
actions. Returning `sqlite3.SQLITE_OK` from the callback ensures that unfiltered actions can
still pass through the guardrail without incurring any errors.

```python
# src.py
import sqlite3

conn = sqlite3.connect(":memory:")


def authorizer(action, arg1, arg2, dbname, trigger):
    # Print the params.
    print(action, arg1, arg2, dbname, trigger)

    # Disallow these actions.
    if action in (
        sqlite3.SQLITE_CREATE_TABLE,
        sqlite3.SQLITE_CREATE_INDEX,
        sqlite3.SQLITE_DROP_TABLE,
        sqlite3.SQLITE_DROP_INDEX,
    ):
        return sqlite3.SQLITE_DENY

    # Let everything else pass through.
    return sqlite3.SQLITE_OK


c = conn.cursor()

with conn:
    c.execute(
        """
        create table if not exists colors (
            name text
        );
    """
    )
    # After creating the table, let's make sure users can't perform
    # certain DDL operations.
    conn.set_authorizer(authorizer)

    c.execute("insert into colors (name) values (?);", ("red",))

    # This will fail because the authorizer will deny the operation.
    c.execute("drop table colors;")
```

```txt
18 colors None main None
22 BEGIN None None None
9 sqlite_master None main None
11 colors None main None
22 ROLLBACK None None None
Traceback (most recent call last):
  File "/home/rednafi/canvas/personal/reflections/src.py", line 45, in <module>
    c.execute("drop table colors;")
sqlite3.DatabaseError: not authorized
```

## Changing the representation of a row

The `sqlite3` module allows you to change the representation of a database row to your
liking. By default, the result of a query comes out as a list of tuples where each tuple
represents a single row. However, you can change the representation of database rows in such
a way that the result might come out as a list of dictionaries or a list of custom objects.

### Via an arbitrary container object as the row factory

You can attach a callback to the `connection_obj.row_factory` attribute to change how you
want to display the rows in a result list. The factory callback takes in two arguments—
`cursor` and `row` where `cursor` is a tuple containing some metadata related to a single
table record and `row` is the default representation of a single database row as a tuple.

In the following snippet, just like before, I'm creating the same `colors` table with two
columns—`name` and `hex`. Here, the `row_factory` function is the factory callback that
converts the default row representation from a tuple to a dictionary. We're then registering
the `row_factory` function with the `connection_obj.row_factory = row_factory` assignment
statement. Afterward, the `sqlite3` module calls this statement on each record and
transforms the representation of the rows. When you run the snippet, you'll see that the
result comes out as a list of dictionaries instead of a list of tuples.

```python
# src.py
import sqlite3


conn = sqlite3.connect(":memory:")


# Using a dictionary to represent a row.
def row_factory(cursor, row):
    # cursor.description:
    # (name, type_code, display_size,
    # internal_size, precision, scale, null_ok)
    # row: (value, value, ...)
    return {
        col[0]: row[idx]
        for idx, col in enumerate(
            cursor.description,
        )
    }


conn.row_factory = row_factory

c = conn.cursor()

with conn:
    c.execute(
        """
        create table if not exists colors (
            name text,
            hex text
        );
    """
    )

    c.execute(
        "insert into colors (name, hex) values (?, ?);",
        ("red", "#ff0000"),
    )
    c.execute(
        "insert into colors (name, hex) values (?, ?);",
        ("green", "#00ff00"),
    )
    c.execute(
        "insert into colors (name, hex) values (?, ?);",
        ("blue", "#0000ff"),
    )

    result = c.execute("select * from colors;").fetchall()
    print(result)
```

```txt
[
    {'name': 'red', 'hex': '#ff0000'},
    {'name': 'green', 'hex': '#00ff00'},
    {'name': 'blue', 'hex': '#0000ff'}
]
```

### Via a specialized Row object as the row factory

Instead of rolling with your own custom row factory, you can also take advantage of the
highly optimized `sqlite3.Row` object. From the docs:

> A `Row` instance serves as a highly optimized `row_factory` for Connection objects. It
> supports iteration, equality testing, len(), and mapping access by column name and index.
> Two row objects compare equal if have equal columns and equal members.

In the following example, I've reused the script from the previous section and just replaced
the custom row factory callback with `sqlite3.Row`. In the output, you'll see that the `Row`
object not only allows us to access the value of a column by `row[column_name]` syntax but
also let us convert the representation of the final result.

```python
# src.py
import sqlite3

conn = sqlite3.connect(":memory:")

# Registering a highly optimized 'Row' object as the
# default row_factory. Row is a map-like object that
# allows you to access column values by name.
conn.row_factory = sqlite3.Row

c = conn.cursor()

with conn:
    c.execute(
        """
        create table if not exists colors (
            name text,
            hex text
        );
    """
    )

    c.executemany(
        "insert into colors (name, hex) values (?, ?);",
        (
            ("red", "#ff0000"),
            ("green", "#00ff00"),
            ("blue", "#0000ff"),
        ),
    )

    result = c.execute("select * from colors;").fetchall()

    # Access the values of a row by column name.
    for row in result:
        print(row["name"], row["hex"])

    # Convert the result to a list of dicts.
    result_dict = [dict(row) for row in result]
    print(result_dict)
```

```txt
red #ff0000
green #00ff00
blue #0000ff
[
    {'name': 'red', 'hex': '#ff0000'},
    {'name': 'green', 'hex': '#00ff00'},
    {'name': 'blue', 'hex': '#0000ff'}
]
```

### Via text factory

If you need to apply a common transformation callback to multiple text columns, the
`sqlite3` module has a shortcut to do so. You can certainly write an ordinary row factory
that'll only transform the text columns but the `connection_obj.text_factory` attribute
enables you to do that in a more elegant fashion. You can set
`connection_obj.text_factory = row_factory` and that'll selectively apply the `row_factory`
callback only to the text columns. In the following example, I'm applying an anonymous
function to the text columns to translate the color names to English.

```python
# src.py
import sqlite3


conn = sqlite3.connect(":memory:")

c = conn.cursor()

# Apply factory only to text fields.
color_map = {"το κόκκινο": "red", "সবুজ": "green"}

# Translate all the text fields.
conn.text_factory = lambda x: color_map.get(x.decode("utf-8"), x)

with conn:
    c.execute("create table if not exists colors (name text);")

    c.execute(
        "insert into colors (name) values (?);", ("το κόκκινο",)
    )
    c.execute("insert into colors (name) values (?);", ("সবুজ",))

    result = c.execute("select * from colors;").fetchall()
    print(result)
```

```txt
[('red',), ('green',)]
```

## Creating custom collation

Collation defines how the string values in a text column are compared. It also dictates how
the data in the column will be ordered when you perform any kind of sort operation. A
collation callback can be registered with the
`connection_obj.create_collation(name, collation_callback)` syntax where the `name` denotes
the name of the collation rule and the `collation_callback` determines how the string
comparison should be done. The callback accepts two string values as arguments and returns:

-   1 if the first is ordered higher than the second
-   -1 if the first is ordered lower than the second
-   0 if they are ordered equal

Then you can use the collation rules with an order by clause as follows:

```sql
select * from table_name order by column_name collate collation_name
```

Here's a full example of a collation callback in action:

```python
# src.py
import sqlite3

conn = sqlite3.connect(":memory:")

c = conn.cursor()


def reverse_collate(a, b):
    return 1 if a < b else -1 if a > b else 0


# Register the collation function.
conn.create_collation("reverse", reverse_collate)

with conn:
    c.execute("create table if not exists colors (name text);")

    c.executemany(
        "insert into colors (name) values (?);",
        (("το κόκκινο",), ("সবুজ",)),
    )

    result = c.execute(
        """select * from colors
            order by name collate reverse;"""
    ).fetchall()
    print(result)
```

```txt
[('সবুজ',), ('το κόκκινο',)]
```

## Registering trace callbacks to introspect running SQL statements

During debugging, I often find it helpful to be able to trace all the SQL statements running
under a certain connection. This becomes even more useful in a multiprocessing environment
where each process opens a new connection to the DB and runs its own sets of SQL queries. We
can leverage the `connection_obj.set_trace_callback` method to trace the statements. The
`set_trace_callback` method accepts a callable that takes a single argument and `sqlite3`
module passes the currently running statement to the callback every time it invokes it.
Notice how the output prints all the statements executed by SQLite behind the scene. This
also reveals that `cursor_obj.executemany` wraps up multiple statements in a transaction and
runs them in an atomic manner.

```python
# src.py
import sqlite3

conn = sqlite3.connect(":memory:")

c = conn.cursor()


# Print all the statements executed.
def introspect(s):
    print(s)


# Register the trace function.
conn.set_trace_callback(introspect)

with conn:
    c.execute("create table if not exists colors (name text);")

    c.executemany(
        "insert into colors (name) values (?);",
        (("red",), ("green",), ("blue",)),
    )

    result = c.execute("""select * from colors""").fetchall()
    print(result)
```

```txt
create table if not exists colors (name text);
BEGIN
insert into colors (name) values (?);
insert into colors (name) values (?);
insert into colors (name) values (?);
select * from colors
[('red',), ('green',), ('blue',)]
COMMIT
```

## Backing up a database

There are a few ways you can back up your database file via Python `sqlite3`.

### Dumping the database iteratively

The following snippet creates a table, inserts some data into it, and then, iteratively
fetches the database content via the `connection_obj.iterdump()` API. Afterward, the
returned content is written to another database file using the `file.write` primitive.

For demonstration purposes, I'm using an in-memory DB and backing that up in another
`NamedTemporaryFile`. This will work the same way with an on-disk DB and on-disk backup file
as well. One advantage of this approach is that your data is not loaded into memory at once,
rather it's streamed iteratively from the main DB to the backup DB.

```python
# src.py
import sqlite3
import tempfile
from contextlib import ExitStack

conn = sqlite3.connect(":memory:")

with ExitStack() as stack:
    conn = stack.enter_context(conn)
    dst_file = stack.enter_context(tempfile.NamedTemporaryFile())

    c = conn.cursor()
    c.execute("create table if not exists colors (name text);")
    c.executemany(
        "insert into colors (name) values (?);",
        (("red",), ("green",), ("blue",)),
    )

    for line in conn.iterdump():
        dst_file.write(line.encode("utf-8") + b"\n")

    dst_file.seek(0)
    print(dst_file.read().decode("utf-8"))
```

```txt
BEGIN TRANSACTION;
CREATE TABLE colors (name text);
INSERT INTO "colors" VALUES('red');
INSERT INTO "colors" VALUES('green');
INSERT INTO "colors" VALUES('blue');
COMMIT;
```

### Copying an on-disk database to another

This example shows another approach that you can adopt to create a second copy of your
on-disk DB. First, it connects to the source DB and then creates another connection to an
empty backup DB. Afterward, the source data is backed up to the destination DB with the
`connection_obj_source.backup(connection_obj_destination)` API.

The `.backup` method takes in three values—a connection object that points to the
destination DB, the number of pages to copy in a single pass, and a callback to introspect
the progress. You can set the value of the `progress` parameter to `-1` if you want to load
the entire source database into memory and copy everything to the destination in a single
pass. Also, in this example, the `progress` hook just prints the progress of the copied
pages.

```python
# src.py
import sqlite3
from contextlib import ExitStack

conn_src = sqlite3.connect("src.db")
conn_dst = sqlite3.connect("dst.db")


# Hook that indicates backup progress.
def progress(status, remaining, total):
    print(f"Copied {total-remaining} of {total} pages...")


with ExitStack() as stack:
    conn_src = stack.enter_context(conn_src)
    conn_dst = stack.enter_context(conn_dst)

    cursor_src = conn_src.cursor()

    cursor_src.execute(
        """
        create table if not exists colors (
            name text,
            hex text
        );
    """
    )

    cursor_src.executemany(
        "insert into colors (name, hex) values (?, ?);",
        (
            ("red", "#ff0000"),
            ("green", "#00ff00"),
            ("blue", "#0000ff"),
        ),
    )

    # Must commit before backup.
    conn_src.commit()

    # Copy a to b. The 'pages' parameter determines how many DB pages
    # to copy in a single iteration. Set to -1 to load everything into
    # memory at once and do it in a single iteration.
    conn_src.backup(conn_dst, pages=1, progress=progress)

    # Ensure that the backup is complete.
    result = conn_dst.execute(
        "select count(*) from colors;"
    ).fetchone()
    print(f"Number of rows in dst: {result[0]}")
```

```txt
Copied 1 of 2 pages...
Copied 2 of 2 pages...
Number of rows in dst: 3
```

### Loading an on-disk database into the memory

The `connection_obj.backup` API also lets you load your existing database into memory. This
is helpful when the DB you're working with is small and you want to leverage the extra
performance benefits that come with an in-memory DB. The workflow is almost exactly the same
as before and the only difference is that the destination connection object points to an
in-memory DB instead of an on-disk one.

```python
# src.py
import sqlite3

conn_src = sqlite3.connect("src.db")
conn_dst = sqlite3.connect(":memory:")

with conn_src:
    cursor_src = conn_src.cursor()

    cursor_src.execute(
        """
        create table if not exists colors (
            name text,
            hex text
        );
    """
    )

    cursor_src.executemany(
        "insert into colors (name, hex) values (?, ?);",
        (
            ("red", "#ff0000"),
            ("green", "#00ff00"),
            ("blue", "#0000ff"),
        ),
    )

    # Must commit before backup.
    conn_src.commit()

    # Copy a to memory.
    conn_src.backup(conn_dst)

    # Ensure that the backup is complete.
    result = conn_dst.execute(
        "select count(*) from colors;"
    ).fetchone()
    print(f"Number of rows in dst: {result[0]}")
```

```txt
Number of rows in dst: 3
```

### Copying an in-memory database to an on-disk file

You can also dump your in-memory DB into the disk. Just point the source connection object
to the in-memory DB and the destination connection to the on-disk DB file.

```python
# src.py
import sqlite3

conn_src = sqlite3.connect(":memory:")
conn_dst = sqlite3.connect("dst.db")

with conn_src:
    cursor_src = conn_src.cursor()

    cursor_src.execute(
        """
        create table if not exists colors (
            name text,
            hex text
        );
    """
    )

    cursor_src.executemany(
        "insert into colors (name, hex) values (?, ?);",
        (
            ("red", "#ff0000"),
            ("green", "#00ff00"),
            ("blue", "#0000ff"),
        ),
    )

    # Must commit before backup.
    conn_src.commit()

    # Copy a to an on-disk file.
    conn_src.backup(conn_dst)

    # Ensure that the backup is complete.
    result = conn_dst.execute(
        "select count(*) from colors;"
    ).fetchone()
    print(f"Number of rows in dst: {result[0]}")
```

```txt
Number of rows in dst: 3
```

## Implementing a full text search engine

This is not exactly a feature that's specific to the `sqlite3` API. However, I wanted to
showcase how effortless it is to leverage SQLite's native features via the Python API. The
following example creates a virtual table and implements a full-text search engine that
allows us to fuzzy search the colors in the `colors` table by their names or hex values.

```python
# src.py
import sqlite3
import uuid

conn = sqlite3.connect(":memory:")

# Get the search result as a Python dict.
conn.row_factory = lambda cursor, row: {
    col[0]: row[idx] for idx, col in enumerate(cursor.description)
}

with conn:
    c = conn.cursor()

    # Unindexed ensures that uuid field doesn't appear in the ft5 index.
    c.execute(
        """
        create virtual table
        if not exists colors using fts5(uuid unindexed, name, hex);"""
    )
    get_uuid = lambda: str(uuid.uuid4())  # noqa: E731

    color_data = (
        (get_uuid(), "red", "#ff0000"),
        (get_uuid(), "green", "#00ff00"),
        (get_uuid(), "blue", "#0000ff"),
        (get_uuid(), "yellow", "#ffff00"),
        (get_uuid(), "cyan", "#00ffff"),
        (get_uuid(), "magenta", "#ff00ff"),
        (get_uuid(), "black", "#000000"),
        (get_uuid(), "white", "#ffffff"),
    )
    c.executemany("insert into colors values (?, ?, ?);", color_data)
    result = c.execute(
        """select * from colors where name match
            'cyan OR red NOT magenta';"""
    ).fetchall()
    print(result)
```

```txt
[
    {
        'uuid': 'c5f6e5ea-124b-44fe-afad-69aad565541e',
        'name': 'red',
        'hex': '#ff0000'

    },
    {
        'uuid': '0dedbb75-6e1d-4a7f-85f7-f0ed0ae4d162',
        'name': 'cyan',
        'hex': '#00ffff'
    }
]
```

[^1]: [sqlite3](https://docs.python.org/3/library/sqlite3.html)
[^2]: [SQLite actions](https://www.sqlite.org/c3ref/constlist.html)
