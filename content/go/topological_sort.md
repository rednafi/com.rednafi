---
title: Topological sort
date: 2024-10-13
tags:
    - Go
mermaid: true
---

I was fiddling with `graphlib` in the Python stdlib and found it quite nifty. It processes a
**Directed Acyclic Graph (DAG)**, where tasks (nodes) are connected by directed edges
(dependencies), and returns the correct execution order. The "acyclic" part ensures no
circular dependencies.

Topological sorting is essential for arranging tasks so that each one follows its
dependencies. It's widely used in scheduling, build systems, dependency resolution, and
database migrations.

For example, consider these tasks:

-   Task A must be completed before Tasks B and C.
-   Tasks B and C must be completed before Task D.

This can be represented as:

<!-- prettier-ignore-start -->

{{< mermaid >}}
graph TD
    A --> B
    A --> C
    B --> D
    C --> D
{{</ mermaid >}}

<!-- prettier-ignore-end -->

Here, **A** can start right away, **B** and **C** follow after **A**, and **D** is last,
depending on both **B** and **C**.

Using `graphlib`'s `TopologicalSorter`, this task order can be determined as:

1. A
2. B and C (in parallel, since both depend only on A)
3. D (which depends on both B and C)

This method ensures tasks are executed in the right sequence while respecting all
dependencies.

To resolve the above mentioned case with `graphlib`, you'd do the following:

```python
from graphlib import TopologicalSorter

# Define the graph
graph = {
    "A": [],  # A has no dependency
    "B": ["A"],  # B depends on A
    "C": ["A"],  # C depends on A
    "D": ["B", "C"],  # D depends on B and C
}

# Create a TopologicalSorter instance
sorter = TopologicalSorter(graph)

# Get the tasks in the correct order
sorter.prepare()

# Resolve the tasks in batch mode
while sorter.is_active():
    batch = tuple(sorter.get_ready())
    print("Executing:", batch)
    sorter.done(*batch)
```

Running this will print the following:

```txt
Executing: ('A',)
Executing: ('B', 'C')
Executing: ('D',)
```

Since Python's stdlib already has `graphlib`, I thought I'd write a sloppy one in Go to learn the mechanics of how it works.

## Writing a topological sorter in Go

The API will be similar to what we've seen in Python. First,
