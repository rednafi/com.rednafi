---
title: Strategy pattern in Go
date: 2024-02-17
slug: strategy-pattern
aliases:
    - /go/strategy_pattern/
tags:
    - Go
    - TIL
---

These days, I don't build hierarchical types through inheritance even when writing languages
that support it. Type composition has replaced almost all of my use cases where I would've
reached for inheritance before.

I've written about how to [escape the template pattern hellscape] and replace that with
_strategy pattern_ in Python before. While by default, Go saves you from shooting yourself
in the foot by disallowing inheritance, it wasn't obvious to me how I could apply the
strategy pattern to make things more composable and testable.

Also, often the Go community exhibits a knee-jerk reaction to the word "pattern," even when
it has nothing to do with OO. However, I feel it's important to use a specific term while
explaining a concept, and I'd rather not attempt to relabel a concept when an established
term already exists for it.

Just a quick recap: the strategy pattern is a design approach where you can choose from a
set of methods to solve a problem, each method wrapped in its own class. This way, you can
swap out these methods easily without messing with the rest of your code, making it simple
to adjust behaviors on the fly.

Let's say you're writing a `display` service that prints a message in either plain text or
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

The `display` function simply takes a `message` and a `formatter`, and calls
`formatter.output(message)` without knowing anything about what the formatter does.

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

Finally, at runtime, you can instantiate the strategy classes and pass them explicitly to
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

The same thing can be achieved in a more functional manner and we'll see that in the Go
example.

But Ruby is still primarily an OO language and it has classes. How'd you model the same
solution in a language like Go where there's no concept of a class or explicit interface
implementation? This wasn't clear to me from the get-go until I started playing with the
language a little more and digging through some OSS codebases.

Turns out, in Go, you can do the same thing using interfaces and custom types, and with even
fewer lines of code. Here's how:

```go
// main.go

// Formatter interface defines a method for outputting messages
type Formatter interface {
    Output(message string) string
}

// OutputFunc is a function type that matches the signature of the Output
// method in the Formatter interface
type OutputFunc func(message string) string

// Output method makes OutputFunc satisfy the Formatter interface
func (f OutputFunc) Output(message string) string {
    return f(message)
}
```

Above, we're defining a `Formatter` interface that contains only a single method `Output`.
Then we define an `OutputFunc` type that implements the `Output` method on the function to
satisfy the `Formatter` interface. We could opt in for a struct type here instead of
defining a function type but since we don't need to hold any state, a function type keeps
things concise.

The display function will look as follows:

```go
func Display(message string, format Formatter) {
    fmt.Println(format.Output(message))
}
```

Similar to the Ruby example, `Display` intakes a string message and an object of any type
that implements the `Formatter` interface. Next, it calls the `Output` method on `format`
without having any knowledge of what that does, achieving polymorphism.

Also, notice that we aren't handling the "unknown formatter" case explicitly because now
it'll be a compile-time error if an unknown formatter is passed to the caller.

Next, you'll define your strategies and pass them to the `Display` function as follows:

```go
func main() {
    message := "Hello, World!"

    // Each strategy needs to be wrapped in OutputFunc so that the
    // underlying function satisfies the Formatter interface.
    TextFormatted := OutputFunc(func (message string) string {
        return message
    })

    JSONFormatted := OutputFunc(func (message string) string {
        jsonData, _ := json.Marshal(map[string]string{"message": message})
        return string(jsonData)
    })

    Display(message, TextFormatted) // Prints "Hello, World!"
    Display(message, JSONFormatted) // Prints "{"message":"Hello, World!"}"
}
```

We're defining each formatting strategy as a function and casting it to the `OutputFunc` so
that it satisfies the `Formatter` interface. Then we just pass the message and the strategy
instance to the `Display` function as before. Notice how your data and strategies are also
decoupled in this case; one has no knowledge of the existence of the other.

And voila, you're done!

_Update: The original Go example used struct types rather than a function type to meet the
`Formatter` interface requirements. In this particular case, the function type makes things
simpler. However, if your strategy needs to do multiple things, then a struct with multiple
methods is probably going to be better._

<!-- References -->
<!-- prettier-ignore-start -->

<!-- escaping the template pattern hellscape in Python -->
[escape the template pattern hellscape]:
    /python/escape_template_pattern/

<!-- prettier-ignore-end -->
