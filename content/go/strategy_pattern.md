---
title: Strategy pattern in Go
date: 2024-02-16
tags:
    - Go
    - TIL
---

These days, I don't build hierarchical types through inheritance even when writing languages
that support it. Type composition has replaced almost all of my use cases where I would've
reached for inheritance before.

I've written[^1] about how to escape the _template pattern_[^2] hellscape and replace that
with _strategy pattern_[^3] in Python before. While by default, Go saves you from shooting
yourself in the foot by disallowing inheritance, it wasn't obvious to me how I could apply
the strategy pattern to make things more composable and testable.

Also, often the Go community exhibits a knee-jerk reaction to the word "pattern," even when
it has nothing to do with OO. However, I feel it's important to use a specific term while
explaining a concept, and I'd rather not attempt to relabel a concept when an established
term already exists for it.

Just a quick recap: the strategy pattern is a design approach where you can choose from a
set of methods to solve a problem, each method wrapped in its own class. This way, you can
swap out these methods easily without messing with the rest of your code, making it simple
to adjust behaviors on the fly.

Let's say you're writing a `display` service that prints a message as either plain text or
JSON formats. Imperatively you could do this:

```rb
# main.rb
require 'json'

def display(message, format)
  if format == :text
    puts message
  elsif format == :json
    json_output = { message: message }.to_json
    puts json_output
  else
    puts "Unknown format"
  end
end

# Usage
display("Hello, World!", :text) # Prints "Hello, World!"
display("Hello, World!", :json)  # Prints "{"message":"Hello, World!"}"
```

While this is a trivial example, you can see that adding more formats means we'll need to
extend the conditionals in the `display` function, and this gets out of hand pretty quickly
in many real-life situations where you might have a non-trivial amount of cases.

However, the biggest reason why the imperative solution isn't ideal is because of how
difficult it is to test. Imagine each of the conditionals triggers some expensive side
effects when the corresponding block runs. How'd you test `display` then in an isolated
manner without mocking the whole universe?

Strategy pattern tells us that each conditional can be converted into a class with one
method. We call these classes strategies. Then, we initialize these strategy classes at
runtime and explicitly pass the instances to the `display` function. The function knows how
to use the strategy instances and executes a specific strategy to print a message in a
particular format based on a certain condition.

Here's how you could rewrite the previous example. In the first phase, we'll wrap each
formatter in a separate class:

```rb
# main.rb
require 'json'

# Formatter Interface
class MessageFormatter
  def output(message)
    raise NotImplementedError, "This method should be overridden"
  end
end

# Concrete Formatter for Text
class TextFormatter < MessageFormatter
  def output(message)
    message
  end
end

# Concrete Formatter for JSON
class JsonFormatter < MessageFormatter
  def output(message)
    { message: message }.to_json
  end
end
```

Here, the `TextFormatter` and `JsonFormatter` classes implement the `MessageFormatter`
interface. This interface requires the downstream classes to implement the `output` method.
The `output` methods of the respective formatters know how to format and print the messages.

The `display` function can then just accept a `message` and a `formatter` instance and run
`formatter.output(message)` while remaining completely ignorant of what each formatter does.
This enables polymorphism.

```rb
# main.rb

# Display Function with direct unknown format handling
def display(message, formatter)
  unless formatter.is_a?(MessageFormatter)
    puts "Unsupported format"
    return
  end
  output = formatter.output(message)
  puts output
end
```

Finally, in runtime, you can instantiate the strategy classes and pass them explicitly to
the `display` function as necessary:

```rb
# main.rb

require_relative 'formatter'

text_formatter = TextFormatter.new
json_formatter = JsonFormatter.new

display("Hello, World!", text_formatter) # Prints "Hello, World!"
display("Hello, World!", json_formatter) # Prints "{"message":"Hello, World!"}"
```

Now whenever you need to test the `display` function, you can just create a fake formatter
and pass that as an argument. The `display` function will happily accept any formatter as
long as the strategy class satisfies the `MessageFormatter` interface.

Same thing can be achived in a more functional[^4] manner as well.

But Ruby is still primarily an OO language and it has classes. How'd you model the same
solution in a language like Go where there's no concept of a class or explicit interface
implementation? This wasn't clear to me from the get-go until I started playing with the
language a little more and digging through OSS codebase with GitHub code search.

Turns out, in Go, you can do the same thing using interfaces and structs, and with even
fewer lines of code. Here's how:

```go
// main.go

// Formatter interface defines a method for outputting messages
type Formatter interface {
    Output() string
}

// TextFormatter for plain text messages
type TextFormatter struct {
    Message string
}

func (t *TextFormatter) Output() string {
    return t.Message
}

// JSONFormatter for JSON-encoded messages
type JSONFormatter struct {
    Message string
}

func (j *JSONFormatter) Output() string {
    jsonData, _ := json.Marshal(map[string]string{"message": j.Message})
    return string(jsonData)
}
```

Above, we're defining a `Formatter` interface that contains only a single method `Output()`.
Then we define the two concrete strategy types `TextFormatter` and `JSONFormatter` and
implement the `Output()` method on both of them to implicitly satisfy the `Formatter`
interface. The `Output` method on the concrete types knows how to print the message in
plaintext or JSON format respectively.

The `Display` function intakes an object of any type that implements the `Formatter`
interface and calls `Output` on it; just like the Ruby example. Notice that we aren't
handling the "unknown formatter" case explicitly because now it'll be a compile time error
if an unknown formatter is passed to the caller.

```go
// main.go

// Display prints the output of a Formatter.
func Display(f Formatter) {
    fmt.Println(f.Output())
}
```

Then you'd initialize the structs and pass the strategy objects to the `Display` function as
follows:

```go
func main() {
    t := &TextFormatter{Message: "Hello, World!"}
    j := &JSONFormatter{Message: "Hello, World!"}

    Display(t) // Prints "Hello, World!"
    Display(j) // Prints "{"message":"Hello, World!"}"
}
```

And voila, you're done.

Now, how would you solve the method-sharing problem in Go? What if we needed a common method
`Header` that prints a header before displaying the message in both of the concrete types?
We could just define a `Header` function and call it inside the `Output` methods to print
the header. But let's say, for the sake of argument, we want to share some functionalities
across multiple concrete types.

We could duplicate the method implementation and define `Header` on each of the
`TextFormatter` and `JSONFormatter` types along with the `Output` method. But that
introduces redundant duplication.

In Ruby, we could just implement the common method in the interface class; the interface
class isn't anything special, and we can add methods there. Then all the downstream classes
will automatically inherit the method. But if we wanted to avoid inheritance, we could also
define another strategy class wrapping the common method and pass that to `Display`.

But in Go, we can do some type embedding shenanigans to ensure compile-time check and avoid
method duplication. Here's the implementation:

```go
package main

import (
    "encoding/json"
    "fmt"
)

// Formatter interface defines a method for outputting messages
type Formatter interface {
    Output() string
}

// CommonFormatter wraps a Formatter and provides a Header method.
// This struct now is responsible for providing a common header functionality
// without requiring each concrete formatter to implement it.
type CommonFormatter struct {
    Formatter
}

// Header returns a common header string
func (c *CommonFormatter) Header() string {
    return "Common Header:"
}

// TextFormatter for plain text messages
type TextFormatter struct {
    Message string
}

// Output returns the message as plain text
func (t *TextFormatter) Output() string {
    return t.Message
}

// JSONFormatter for JSON-encoded messages
type JSONFormatter struct {
    Message string
}

// Output returns the message as a JSON string
func (j *JSONFormatter) Output() string {
    jsonData, _ := json.Marshal(map[string]string{"message": j.Message})
    return string(jsonData)
}

// Display prints the header and output of a Formatter wrapped by CommonFormatter
func Display(c *CommonFormatter) {
    fmt.Println(c.Header()) // Call Header on CommonFormatter
    fmt.Println(c.Output()) // Call Output on the embedded Formatter
}

func main() {
    t := &CommonFormatter{Formatter: &TextFormatter{Message: "Hello, World!"}}
    j := &CommonFormatter{Formatter: &JSONFormatter{Message: "Hello, World!"}}

    Display(t)
    Display(j)
}
```

Running this will print:

```txt
Common Header:
Hello, World!
Common Header:
{"message":"Hello, World!"}
```

The final snippet shows how to share a `Header` method among different types by using a
struct to embed an interface. Here, the `CommonFormatter` struct embeds the `Formatter`
interface, which means both `TextFormatter` and `JSONFormatter` can use the `Header` method
directly. This setup lets these types get a common function without having to write it
multiple times.

While this is convenient, it isn't inheritance by any means. The common `Head` method on the
`CommonFormatter` type can't operate on the attributes of the embedded `TextFormatter` or
`JSONFormatter` types without doing some runtime introspections or generic tricks. If you
need that, it's probably easier to define the method on the concrete classes anyway!

[^1]: [Escaping the template pattern hellscape in Python](/python/escape_template_pattern/)
[^2]:
    [Template method pattern in Ruby](https://refactoring.guru/design-patterns/template-method/ruby/example)

[^3]:
    [Strategy pattern in Ruby](https://refactoring.guru/design-patterns/strategy/ruby/example)

[^4]:
    All of this festivities can be avoided in languages that support first-class functions.
    You could just define your strategies as functions and pass them to the caller during
    runtime; same idea, different implementation.
