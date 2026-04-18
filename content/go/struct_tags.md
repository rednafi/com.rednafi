---
title: Peeking into Go struct tags
date: 2026-04-18
slug: struct-tags
aliases:
    - /go/fun-with-struct-tags/
tags:
    - Go
    - Reflection
    - Codegen
description: >-
    A quick tour of Go struct tags: how different libraries use them, how you read them
    at runtime with reflection, and how other tools read them at build time instead.
---

Struct tags in Go are these little annotations that you stick beside struct fields.
Libraries read them to decide what to do with each field, and the most familiar place
you'll see them is JSON marshalling and unmarshalling:

```go
type User struct {
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
    Admin bool   `json:"-"`
}

b, _ := json.Marshal(User{Name: "ada", Email: "a@b.com"})
// {"name":"ada","email":"a@b.com"}
```

`encoding/json` reads those tags to pick the wire key, drop a zero value with
`omitempty`, or skip the field with `-`.

Validation libraries do the same thing with a different tag key. [go-playground/validator]
reads a `validate:"..."`:

```go
type SignupRequest struct {
    Email    string `validate:"required,email"`
    Password string `validate:"required,min=8"`
    Age      int    `validate:"gte=13,lte=130"`
}

v := validator.New()
err := v.Struct(SignupRequest{Email: "not-an-email"})
// Key: 'SignupRequest.Email'
// Error:Field validation for 'Email' failed on the 'email' tag
```

My preferred envvar library, [caarlos0/env], does the same for environment variables:

```go
type Config struct {
    Port    int           `env:"PORT" envDefault:"8080"`
    DBURL   string        `env:"DATABASE_URL,required"`
    Timeout time.Duration `env:"TIMEOUT" envDefault:"30s"`
}

// Parse the environment variables
var cfg Config
_ = env.Parse(&cfg)
```

CLI libraries like [alecthomas/kong] use them for parsing flags:

```go
type CLI struct {
    Verbose bool   `help:"Enable verbose logging." short:"v"`
    Config  string `help:"Path to config." default:"/etc/app.conf"`
}

var cli CLI
kong.Parse(&cli)
```

Across all of these the pattern is the same: a string attached to a field, and some
code that reads it. They all do that at runtime through reflection, every time you
call `Marshal`, `Struct`, or `Parse`.

You can also do it earlier, reading the tag once before the program runs and writing
out plain Go that needs no reflection at call time.

## Reading the tag at runtime

The standard library exposes tags through `reflect.StructTag`. A tag is any back-quoted
string after a field, and the API gives you a key/value lookup on it. You can read
your own tag keys the same way:

```go
type User struct {
    Name string `check:"required,min=2" json:"name"`
}

t := reflect.TypeOf(User{})     // reflect.Type
f, _ := t.FieldByName("Name")   // reflect.StructField
                        // f.Tag is reflect.StructTag (the backticked string)

fmt.Println(f.Tag.Get("check")) // required,min=2
fmt.Println(f.Tag.Get("json"))  // name
```

That's the whole surface area. The compiler doesn't inspect the contents. Typos,
malformed values, garbage strings, the compiler is happy with all of it. What a
library does with the string is up to it.

A naive validator that reads a `check` tag and understands `required`, `min`, and
`email` walks the fields and dispatches with a switch:

```go
func Validate(s any) error {
    v := reflect.ValueOf(s).Elem()                  // (1)
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {
        f, name := v.Field(i), t.Field(i).Name      // (2)
        tag := t.Field(i).Tag.Get("check")

        for _, rule := range strings.Split(tag, ",") {
            head, arg, _ := strings.Cut(rule, "=")  // (3)

            switch head {                           // (4)
            case "required":
                if f.IsZero() {
                    return fmt.Errorf("%s: required", name)
                }
            case "min":
                n, _ := strconv.Atoi(arg)
                if len(f.String()) < n {
                    return fmt.Errorf("%s: min %d", name, n)
                }
            case "email":
                if _, err := mail.ParseAddress(f.String()); err != nil {
                    return fmt.Errorf("%s: %w", name, err)
                }
            }
        }
    }
    return nil
}
```

Here:

- (1) unwrap the pointer and grab the struct's type metadata
- (2) for each field, pull its value, its name, and the `check` tag
- (3) rules are comma-separated, and `min=2` cuts into `head="min"`, `arg="2"`
- (4) dispatch on the rule name; each case formats its own error

Call it like this:

```go
type User struct {
    Name  string `check:"required,min=2"`
    Email string `check:"required,email"`
}

err := Validate(&User{Name: "a", Email: "bad"})
fmt.Println(err) // Name: min 2
```

This is fine for three rules. By the time you've added `oneof`, `url`, `uuid`, `regex`,
and nested struct validation, the switch becomes unmanageable. A cleaner shape pulls
each rule into its own function and keeps a map keyed by tag name. The validator falls
into two halves: a registry of rules, and a dispatcher that runs them.

The registry maps each tag name to a small function that checks one thing:

```go
type Rule func(f reflect.Value, arg string) error

var rules = map[string]Rule{
    "required": func(f reflect.Value, _ string) error {
        if f.IsZero() {
            return errors.New("required")
        }
        return nil
    },
    "min": func(f reflect.Value, arg string) error {
        n, _ := strconv.Atoi(arg)
        if len(f.String()) < n {
            return fmt.Errorf("min length %d", n)
        }
        return nil
    },
    "email": func(f reflect.Value, _ string) error {
        _, err := mail.ParseAddress(f.String())
        return err
    },
}
```

Each rule takes a field value and an optional argument, and returns an error. Adding a
new rule is one new map entry, no changes to anything else.

The dispatcher is the same reflection loop as before, but without the switch. It looks
up a handler by tag name and calls it:

```go
func Validate(s any) error {
    v := reflect.ValueOf(s).Elem()
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {              // (1)
        tag := t.Field(i).Tag.Get("check")           // (2)
        for _, rule := range strings.Split(tag, ",") {
            head, arg, _ := strings.Cut(rule, "=")
            fn, ok := rules[head]                    // (3)
            if !ok {
                continue
            }
            if err := fn(v.Field(i), arg); err != nil {  // (4)
                return fmt.Errorf("%s: %w", t.Field(i).Name, err)
            }
        }
    }
    return nil
}
```

- (1) walk every field of the struct
- (2) grab the `check` tag and split it on commas, one rule per comma
- (3) look up the rule's handler in the `rules` map; unknown rules are skipped
- (4) call the handler with this field's value, bubble the error up with the field name

The dispatcher doesn't know what any given rule does, only that it exists in the map.

Open [baked_in.go] in `go-playground/validator` and you'll find the same shape: a
`bakedInValidators` map with entries like `"required"`, `"email"`, `"len"`, `"min"`,
each pointing to a small function. The public `validate.RegisterValidation("uuid", ...)`
call inserts another entry into that map at runtime. The reflection sits in around
twenty lines, and every new rule is one more function in the map.

## Reading the tag at build time

The runtime shape pays a reflection cost on every call. You can skip that by reading
the tag once, before the program runs. [easyjson] is built around this idea: it's a
drop-in alternative to `encoding/json` that reads your `json:"..."` tags at
`go generate` time and writes out a `MarshalJSON` and `UnmarshalJSON` per type, with
no reflection left in either one.

Take the `User` we marshalled at the top of the post, with one line added to it:

```go
//easyjson:json
type User struct {
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
    Admin bool   `json:"-"`
}
```

Run `easyjson user.go` and you get a `user_easyjson.go` alongside it. The full file
([user_easyjson.go] in the examples repo) is ~90 lines of straight-line Go, but it's
mostly plumbing. The skeleton is:

```go
// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.
package ejdemo

import ( /* encoding/json, easyjson/jwriter, easyjson/jlexer */ )

// Decoder: reads JSON off the lexer into *User.
func easyjson...Decode(in *jlexer.Lexer, out *User) { /* ... */ }

// Encoder: writes User into the jwriter.
func easyjson...Encode(out *jwriter.Writer, in User) {
    out.RawByte('{')
    out.RawString(`"name":`)              // (1)
    out.String(string(in.Name))
    if in.Email != "" {                   // (2)
        out.RawString(`,"email":`)
        out.String(string(in.Email))
    }
    // Admin has no branch at all (3)
    out.RawByte('}')
}

// Standard-library interfaces, wired to the generated functions above.
func (v User)  MarshalJSON()   ([]byte, error) { /* calls Encode */ }
func (v *User) UnmarshalJSON(data []byte) error { /* calls Decode */ }
```

The encoder is where the tag decisions show up. Every choice the tag made has been
frozen into the code:

- (1) `json:"name"` becomes a literal `"name":` in the output, no lookup at call time
- (2) `json:"email,omitempty"` turns into a plain `if in.Email != ""` check
- (3) `json:"-"` drops `Admin` entirely. The field doesn't appear in the encoder, and
  the decoder's `switch` has no `case "admin"`

The only place the tag string meets code is [parseFieldTags] in easyjson's
`gen/encoder.go`, which is the build-time twin of the `bakedInValidators` map from the
runtime half:

```go
// Tag.Get("json") returns everything between the quotes,
// e.g. "email,omitempty".
func parseFieldTags(f reflect.StructField) fieldTags {
    var ret fieldTags

    for i, s := range strings.Split(f.Tag.Get("json"), ",") {
        switch {
        case i == 0 && s == "-":
            ret.omit = true
        case i == 0:
            ret.name = s
        case s == "omitempty":
            ret.omitEmpty = true
        case s == "required":
            ret.required = true
        // ... omitzero, string, intern, nocopy
        }
    }
    return ret
}
```

The returned `fieldTags` is what the surrounding generator consumes: `omit` skips the
field, `omitEmpty` wraps the emit in an `if`, `name` becomes the literal `"name":`
string. That one switch decides every branch in the generated output.

easyjson isn't the only tool that does this. [ent] walks Go schema files and emits a
typed builder per entity, [sqlc] walks SQL queries and emits typed scanners, and
`protoc-gen-go` walks `.proto` files and emits the structs. Different inputs, same
trick: read the schema once at build time and write the Go that would otherwise need
reflection at call time.

Both versions are on [GitHub]: the runtime validator, and a small codegen tool that
emits per-type `Validate` methods.

<!-- references -->
<!-- prettier-ignore-start -->

[go-playground/validator]:
    https://github.com/go-playground/validator

[caarlos0/env]:
    https://github.com/caarlos0/env

[alecthomas/kong]:
    https://github.com/alecthomas/kong

[sqlc]:
    https://github.com/sqlc-dev/sqlc

[ent]:
    https://github.com/ent/ent

[easyjson]:
    https://github.com/mailru/easyjson

[baked_in.go]:
    https://github.com/go-playground/validator/blob/master/baked_in.go

[parseFieldTags]:
    https://github.com/mailru/easyjson/blob/master/gen/encoder.go#L68

[GitHub]:
    https://github.com/rednafi/examples/tree/main/fun-with-struct-tags

[user_easyjson.go]:
    https://github.com/rednafi/examples/blob/main/fun-with-struct-tags/easyjson/user_easyjson.go

<!-- prettier-ignore-end -->
