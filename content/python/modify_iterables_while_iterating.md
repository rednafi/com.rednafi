---
title: Modify iterables while iterating in Python
date: 2022-03-04
tags:
    - Python
---

If you try to mutate a sequence while traversing through it, Python usually doesn't
complain. For example:

```py
# src.py

l = [3, 4, 56, 7, 10, 9, 6, 5]

for i in l:
    if not i % 2 == 0:
        continue
    l.remove(i)

print(l)
```

The above snippet iterates through a list of numbers and modifies the list `l` in-place to
remove any even number. However, running the script prints out this:

```txt
[3, 56, 7, 9, 5]
```

Wait a minute! The output doesn't look correct. The final list still contains `56` which is
an even number. Why did it get skipped? Printing the members of the list while the for-loop
advances reveal what's happening inside:

```txt
3
4
7
10
6
[3, 56, 7, 9, 5]
```

From the output, it seems like the for-loop doesn't even visit all the elements of the
sequence. However, trying to emulate what happens inside the for-loop with `iter` and `next`
makes the situation clearer. Notice the following example. I'm using `ipython` shell to
explore:

```py
In [1]: l = [3, 4, 56, 7, 10, 9, 6, 5]

In [2]: # Make the list an iterator.

In [3]: it = iter(l)

In [4]: # Emulate for-loop by applying 'next()' function on 'it'.

In [5]: next(it)
Out[5]: 3

In [6]: next(it)
Out[6]: 4

In [7]: # Remove a value that's already been visited by the iterator.

In [8]: l.remove(3)

In [9]: next(it)
Out[9]: 7

In [10]: # Notice how the iterator skipped 56. Remove another.

In [11]: l.remove(4)

In [12]: next(it)
Out[12]: 9
```

The REPL experiment reveals that:

> Whenever you remove an element of an iterable that's already been visited by the iterator,
> in the next iteration, the iterator will jump right by 1 element. This can make the
> iterator skip a value. The opposite is also true if you prepend some value to a sequence
> after the iterator has started iterating. In that case, in the next iteration, the
> iterator will jump left by 1 element and may visit the same value again.

Here's what happens when you prepend values after the iteration has started:

```py
In[1]: l = [3, 4, 56, 7, 10, 9, 6, 5]

In[2]: it = iter(l)

In[3]: next(it)
Out[3]: 3

In[4]: next(it)
Out[4]: 4

In[5]: l.insert(0, 44)

In[6]: next(it)
Out[6]: 4
```

Notice how the element `4` is being visited twice after prepending a value to the list `l`.

## Solution

To solve this, you'll have to make sure the target elements don't get removed after the
iterator has already visited them. You can iterate in the reverse order and remove elements
maintaining the original order. The first snippet can be rewritten as follows:

```py
# src.py

l = [3, 4, 56, 7, 10, 9, 6, 5]

# Here, 'reversed' returns a lazy iterator, so it's performant!
for i in reversed(l):
    print(i)
    if not i % 2 == 0:
        continue
    l.remove(i)

print(l)
```

Running the script prints:

```txt
5
6
9
10
7
56
4
3
[3, 7, 9, 5]
```

Notice, how the iterator now visits all the elements and the final list contains the odd
elements as expected.

Another way you can solve this is—by copying the list `l` before iterating. But this can be
expensive if `l` is large:

```py
# src.py
l = [3, 4, 56, 7, 10, 9, 6, 5]

# Here 'l.copy()' creates a shallow copy of 'l'. It's
# less performant than 'reversed(l)'.
for i in l.copy():
    print(i)
    if not i % 2 == 0:
        continue
    l.remove(i)

print(l)
```

This time, the order of the iteration and element removal is the same, but that isn't a
problem since these two operations occur on two different lists. Running the snippet
produces the following output:

```txt
3
4
56
7
10
9
6
5
[3, 7, 9, 5]
```

## What about dictionaries

Dictionaries don't even allow you to change their sizes while iterating. The following
snippet raises a `RuntimeError`:

```py
# src.py

# {0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8, 9: 9}
d = {k: k for k in range(10)}

for k, v in d.items():
    if not v % 2 == 0:
        continue
    d.pop(k)
```

```txt
Traceback (most recent call last):
  File "/home/rednafi/canvas/personal/reflections/src.py", line 4, in <module>
    for k,v in d.items():
RuntimeError: dictionary changed size during iteration
```

You can solve this by making a copy of the keys of the dictionary and iterating through it
while removing the elements from the dictionary. Here's one way to do it:

```py
# src.py

# {0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8, 9: 9}
d = {k: k for k in range(10)}

# This creates a copy of all the keys of 'd'.
# At least we arent't creating a new copy of the
# entire dict and tuple creation is quite fast.
for k in tuple(d.keys()):
    if not d[k] % 2 == 0:
        continue
    d.pop(k)

print(d)
```

Running the snippet prints:

```txt
{1: 1, 3: 3, 5: 5, 7: 7, 9: 9}
```

Voila, the key-value pairs of the even numbers have been removed successfully!

[^1]:
    [How to modify a list while iterating - Anthony Sottile](https://www.youtube.com/watch?v=JXis-BKRDFY)
    [^1]
