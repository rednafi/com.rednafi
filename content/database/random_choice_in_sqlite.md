---
title: Pick random values from an array in SQL(ite)
date: 2022-09-02
slug: random-choice-in-sqlite
aliases:
    - /database/random_choice_in_sqlite/
tags:
    - Database
    - SQL
---

Python has a `random.choice` routine in the standard library that allows you to pick a
random value from an iterable. It works like this:

```py
# src.py
import random

# The seed ensures that you'll get the same random choice
# every time you run the script.
random.seed(90)

# This builds a list: ["choice_0", "choice_1", ..., "choice_9"]
lst = [f"choice_{i}" for i in range(10)]

print(random.choice(lst))
print(random.choice(lst))
```

This will print:

```txt
choice_3
choice_1
```

I was looking for a way to quickly hydrate a table with random data in an SQLite database.
To be able to do so, I needed to extract unpremeditated values from an array of predefined
elements. The issue is, that SQLite doesn't support array types or have a built-in function
to pick random values from an array. However, recently I came across this [trick from
Ricardo Ander-Egg's tweet], where he exploits SQLite's JSON support to parse an array. This
idea can be further extended to pluck random values from an array.

To extract values from any JSON object in SQLite, you can use the `json_extract` function.
Start a SQLite CLI session and run the following query:

```sql
select json_extract(
    '{"greetings": ["Hello", "Hola", "Ohe"]}', '$.greetings[2]'
)
```

This will give you an output as follows:

```txt
Ohe
```

The above query parses the JSON object inside the `json_extract` function and extracts the
last element from the `greetings` array. If you want to know more details about how you can
extract specific elements from JSON objects, head over to the SQLite [docs on this topic].

You can pick any value from a JSON array by its index:

```sql
select json_extract(
    '["Columbus", "Cincinnati", "Dayton", "Toledo"]', '$[2]'
)
```

```txt
Dayton
```

Now, how do we extract random elements from the above array? If we can generate a set of
random indices, those can be used to access values arbitrarily from the JSON array. These
random indices can be generated using SQLite's built-in `random()` function. The function
doesn't take any arguments and generates a large positive or negative arbitrary integer.
From this integer, a random index can be found by computing `abs(random()) modulo n` where
`abs(random())` denotes the absolute result of the random function and `n` represents the
length of the target array.

For example, if the length of the array is `4`, and `random()` produces the integer
`-123456789`, then the index will be `123456789 % 4 = 1` :

```sql
select abs(random()) % 4;
```

If you run this query multiple times, you'll see that it prints a value between `0` and `3`
in random order.

```txt
sqlite> select abs(random()) % 4;
0

sqlite> select abs(random()) % 4;
3

sqlite> select abs(random()) % 4;
2

sqlite> select abs(random()) % 4;
1
```

Similarly, if you compute `abs(random()) % 5`, it'll print a value between `0` to `4` and so
on. Armed with this knowledge, we can extract a random value from a JSON array like this:

```sql
select json_extract(
    '["Columbus", "Cincinnati", "Dayton", "Toledo"]',
    '$[' || cast(abs(random()) % 4 as text) || ']'
);
```

Running the above query will give you a single value from the JSON array in random order.
Execute the query multiple times to see it in action.

```txt
sqlite> select json_extract('["Columbus", ...
Toledo

sqlite> select json_extract('["Columbus", ...
Cincinnati

sqlite> select json_extract('["Columbus", ...
Columbus
```

Voila, we've successfully emulated Python's `random.choice` in SQL.

## Populating a table with random data

Populating a table with randomly distributed data is useful, especially when you need to
demonstrate a feature or flex your SQL fu. We can leverage the above pattern to populate a
simple table with 100 data points like this:

```sql
-- Create the 'stat' table with 'id', 'cat', and 'score' columns.
create table if not exists stat (
    id integer primary key, cat text, score real
);

-- Populate the 'stat' table with random data.
with recursive cte (x, y) as (
    select 'a', random() % 1000
    union all
    select json_extract ('["a", "b", "c"]',
        '$[' || cast(abs(random()) % 3 as text) || ']'),
        random() % 1000
    from cte
    limit 100)
    insert into stat (cat, score)
    select *
    from cte
    where not exists ( -- This block ensures that the query
            select *   -- can be run multiple times without
            from stat  -- any side effects. If you run this
            where stat.cat = cte.x -- multiple times, it'll
            or stat.score = cte.x); -- only insert the values once.

-- Inspect the populated table.
select * from stat;
```

If you run the above queries via the SQLite CLI, the final statement will reveal the `stat`
table with the randomly filled in data:

```txt
| id  | cat | score  |
|-----|-----|--------|
| 1   | a   | 390.0  |
| 2   | a   | 864.0  |
| 3   | b   | -856.0 |
| 4   | b   | -307.0 |
| 5   | c   | -405.0 |
| 6   | a   | -61.0  |
| 7   | a   | 794.0  |
| 8   | b   | -560.0 |
| 9   | a   | -355.0 |
| 10  | c   | 10.0   |
...

| 100 | c   | 420.0  |
```

<!--References -->
<!-- prettier-ignore-start -->

[trick from ricardo ander-egg's tweet]:
    https://twitter.com/ricardoanderegg/status/1564723221173342220?s=20&t=V4TtJsxqyH0IuheqhEvb4w

<!-- the json_extract() function in the sqlite docs -->
[on this topic]:
    https://www.sqlite.org/json1.html#jex

<!-- prettier-ignore-end -->
