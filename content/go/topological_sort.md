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

Topological sorting is useful for arranging tasks so that each one follows its dependencies.
It's widely used in scheduling, build systems, dependency resolution, and database
migrations.

For example, consider these tasks:

- Task A must be completed before Tasks B and C.
- Tasks B and C must be completed before Task D.

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

The task order can be determined as:

1. A
2. B and C (in parallel, since both depend only on A)
3. D (which depends on both B and C)

This method ensures tasks are executed in the right sequence while respecting all
dependencies.

To resolve the above-mentioned case with `graphlib`, you'd do the following:

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

Since Python's stdlib already has `graphlib`, I thought I'd write a sloppy one in Go to
learn the mechanics of how it works.

## Writing a topological sorter in Go

The API will be similar to what we've seen in the `graphlib` example.

### Defining the graph structure

First, we need a graph structure to hold the tasks and their dependencies. We'll use an
adjacency list to represent the graph, and a map to track the in-degree of each node (how
many tasks it depends on).

```go
type Graph struct {
    vertices map[string][]string // Adjacency list for dependencies
    inDegree map[string]int      // Tracks the number of incoming edges
    queue    []string            // Queue of nodes ready to process
    active   int                 // Number of active tasks to process
}
```

Here:

- `vertices`: a list of tasks that each node points to (i.e., its dependents).
- `inDegree`: how many tasks must finish before each task can be processed.
- `queue`: tasks that can be processed because they have no unmet dependencies.
- `active`: how many tasks are currently ready for processing.

### Adding dependencies

Next, we'll define how one task depends on another. The `AddEdge` function sets up this
relationship, ensuring the **source task** knows it must finish before the **destination
task** can proceed.

```go
func (g *Graph) AddEdge(source, destination string) {
    g.vertices[source] = append(g.vertices[source], destination)
    g.inDegree[destination]++        // Increase destination's in-degree
    if _, exists := g.inDegree[source]; !exists {
        g.inDegree[source] = 0 // Ensure the source node is tracked
    }
}
```

- The **destination task** is added to the list of tasks that the **source task** points to,
  marking the dependency.
- The in-degree of the **destination task** is increased by 1 because it depends on the
  source task.
- If the **source task** is new, we initialize its in-degree to 0.

### Initializing and processing tasks in batches

Now we'll initialize the graph by identifying tasks that can be processed immediately—those
with an in-degree of 0 (i.e., they have no dependencies). We then process tasks batch by
batch.

```go
func (g *Graph) Prepare() {
    // Start by adding tasks with in-degree 0 to the queue
    for task, degree := range g.inDegree {
        if degree == 0 {
            g.queue = append(g.queue, task) // Ready to process
        }
    }
    g.active = len(g.queue) // Count how many are active
}
```

- This function finds tasks with an in-degree of 0 (no dependencies) and adds them to the
  processing queue.
- The active count keeps track of how many tasks are ready to run.

### Processing each batch of tasks

We use `GetReady` to retrieve the next batch of tasks that are ready for processing. These
are tasks with no unmet dependencies.

```go
func (g *Graph) GetReady() []string {
    batch := make([]string, len(g.queue)) // Create a batch from the queue
    copy(batch, g.queue)                  // Copy tasks to the batch
    g.queue = []string{}                  // Clear the queue after processing
    return batch                          // Return the ready batch
}
```

- `GetReady` pulls the current batch of tasks from the queue and clears it for the next
  batch.
- Tasks are returned in the order they are ready to be processed.

### Marking the processed tasks as done

Once a batch of tasks is completed, we mark them as done and reduce the in-degree of any
tasks that depend on them.

```go
func (g *Graph) Done(tasks ...string) {
    for _, task := range tasks { // For each completed task
        for _, dependent := range g.vertices[task] {
            g.inDegree[dependent]--          // Decrement dependent's in-degree
            if g.inDegree[dependent] == 0 {  // If ready, add to the queue
                g.queue = append(g.queue, dependent)
            }
        }
    }
    g.active = len(g.queue) // Update the active count
}
```

- For each completed task, we reduce the in-degree of any dependent tasks.
- If a dependent task's in-degree reaches 0, it's added to the queue and is now ready to be
  processed in the next batch.

### Running the full topological sort

Finally, we'll implement the `TopologicalSortBatch` function, which processes all tasks in
batches until none are left.

```go
func TopologicalSortBatch(graph *Graph) {
    graph.Prepare() // Prepare the graph by loading the initial batch
    for graph.IsActive() { // While tasks remain to be processed
        batch := graph.GetReady()       // Get the next batch
        fmt.Println("Next batch:", batch) // Process the batch
        graph.Done(batch...)             // Mark the batch as done
    }
}
```

- `Prepare` loads the first set of tasks that can be processed.
- `IsActive` checks if there are any tasks left to process.
- `GetReady` retrieves the next batch of tasks to process.
- `Done` marks tasks as finished, allowing dependent tasks to be processed next.

### Using the sorter

You can use the API as follows:

```go
g := NewGraph()

// Define task dependencies
g.AddEdge("A", "B")      // B depends on A
g.AddEdge("A", "C")      // C depends on A
g.AddEdge("B", "D")      // D depends on B
g.AddEdge("C", "D")      // D depends on C

// Perform topological sort in batches
TopologicalSortBatch(g)
```

This will return:

```txt
Next batch: [A]
Next batch: [B C]
Next batch: [D]
```

Here, A needs to run first. B and C can run in parallel after A finishes, and only then can
D run.

## Complete example

Here's the full implementation, heavily annotated for clarity:

```go
package main

import "fmt"

type Graph struct {
    vertices map[string][]string // Task dependencies
    inDegree map[string]int      // Number of unmet dependencies
    queue    []string            // Ready tasks
    active   int                 // Active task count
}

func NewGraph() *Graph {
    return &Graph{
        vertices: make(map[string][]string),
        inDegree: make(map[string]int),
        queue:    []string{},
        active:   0,
    }
}

func (g *Graph) AddEdge(source, destination string) {
    // Add the destination task to the source's dependency list
    g.vertices[source] = append(g.vertices[source], destination)
    // Increment the in-degree of the destination task
    g.inDegree[destination]++
    // Ensure the source task is tracked with in-degree 0 if new
    if _, exists := g.inDegree[source]; !exists {
        g.inDegree[source] = 0
    }
}

func (g *Graph) Prepare() {
    // Load tasks with no unmet dependencies (in-degree 0)
    for task, degree := range g.inDegree {
        if degree == 0 {
            g.queue = append(g.queue, task)
        }
    }
    g.active = len(g.queue) // Set active task count
}

func (g *Graph) IsActive() bool {
    return g.active > 0 // Check if there are active tasks left
}

func (g *Graph) GetReady() []string {
    batch := make([]string, len(g.queue)) // Create batch of ready tasks
    copy(batch, g.queue)                  // Copy tasks to the batch
    g.queue = []string{}                  // Clear queue after processing
    return batch                          // Return ready tasks
}

func (g *Graph) Done(tasks ...string) {
    // For each completed task, decrement in-degree of its dependents
    for _, task := range tasks {
        for _, dependent := range g.vertices[task] {
            g.inDegree[dependent]--
            // If dependent has no unmet dependencies, add to queue
            if g.inDegree[dependent] == 0 {
                g.queue = append(g.queue, dependent)
            }
        }
    }
    g.active = len(g.queue) // Update active task count
}

func TopologicalSortBatch(graph *Graph) {
    graph.Prepare()        // Prepare initial batch of tasks
    for graph.IsActive() { // Process tasks while there are active ones
        batch := graph.GetReady()         // Get the next batch
        fmt.Println("Next batch:", batch) // Output batch
        graph.Done(batch...)              // Mark tasks in the batch as done
    }
}

// Usage
func main() {
    g := NewGraph()

    // Define task dependencies
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")
    g.AddEdge("B", "D")
    g.AddEdge("C", "D")

    // Perform topological sort in batches
    TopologicalSortBatch(g)
}
```

You can use this to make [custom task orchestrators].

Fin!

<!-- References -->
<!-- prettier-ignore-start -->

<!-- python graphlib | my favourite python library -->
[custom task orchestrators]:
    https://philuvarov.io/python-top-sort/

<!-- References -->
<!-- prettier-ignore-end -->
