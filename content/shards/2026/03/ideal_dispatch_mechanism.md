---
title: What's the ideal dispatch mechanism?
date: 2026-03-31
slug: ideal-dispatch-mechanism
tags:
    - Go
    - API
description: >-
  Switch, map of functions, and interface registry for dispatching in Go.
---

Someone [asked] in r/golang:

> I'm creating a multi-format converter that converts all graphic formats used on a
> user's system to JPG. I can use a switch based on the file extension to choose how to
> convert each file. Is there a more idiomatic way to structure the code, or is a switch
> preferable for this kind of problem? What construct would be more optimal to maintain,
> extend, and use long term, based on your experience, in place of a switch (number of
> formats up to 20)?

I took a [stab] at it there. Here's the longer version.

---

A switch is fine as a starting point, and I'd start there. Once you hit 10-20 formats, it
becomes a long, central piece of code that you keep touching every time a new format shows
up. But I still wouldn't change anything if maintaining a bunch of case arms isn't actually
causing problems.

```go
package jpgconv

func Convert(srcPath, dstPath string) error {
    switch strings.ToLower(filepath.Ext(srcPath)) {
    case ".png":
        return convertPNG(srcPath, dstPath)
    case ".gif":
        return convertGIF(srcPath, dstPath)
    case ".webp":
        return convertWEBP(srcPath, dstPath)
    default:
        return fmt.Errorf("unsupported source format: %s", filepath.Ext(srcPath))
    }
}
```

But sometimes I don't start with a switch and instead go straight to a map of functions.
This removes the growing conditional. Adding a new format becomes a one-line map entry
instead of editing a big block:

```go
package jpgconv

type ConverterFunc func(srcPath, dstPath string) error

var converters = map[string]ConverterFunc{
    ".png":  convertPNG,
    ".gif":  convertGIF,
    ".webp": convertWEBP,
}

func Convert(srcPath, dstPath string) error {
    ext := strings.ToLower(filepath.Ext(srcPath))

    fn, ok := converters[ext]
    if !ok {
        return fmt.Errorf("unsupported source format: %s", ext)
    }
    return fn(srcPath, dstPath)
}

func convertPNG(srcPath, dstPath string) error {
    // read PNG from srcPath and write JPG to dstPath
    return nil
}
```

This is usually where I stop. But you can keep going and replace the map of functions
with a map of interfaces. Instead of a flat function map, you define a converter interface
and keep a registry of types that satisfy it. The dispatch logic stays the same:

```go
package jpgconv

type Converter interface {
    Extensions() []string
    Convert(srcPath, dstPath string) error
}
```

Now instead of a map of functions, you keep a registry of converters:

```go
package jpgconv

var registry = map[string]Converter{}

func Register(c Converter) {
    for _, ext := range c.Extensions() {
        registry[strings.ToLower(ext)] = c
    }
}

func Convert(srcPath, dstPath string) error {
    ext := strings.ToLower(filepath.Ext(srcPath))

    c, ok := registry[ext]
    if !ok {
        return fmt.Errorf("unsupported source format: %s", ext)
    }
    return c.Convert(srcPath, dstPath)
}
```

Each format can live in its own file and register itself. This avoids touching central code
when adding new formats, which is the main long-term win:

```go
package jpgconv

type PNGConverter struct{}

func (PNGConverter) Extensions() []string {
    return []string{".png"}
}

func (PNGConverter) Convert(srcPath, dstPath string) error {
    return convertPNG(srcPath, dstPath)
}

func init() {
    Register(PNGConverter{})
}
```

I rarely go for the interface approach unless I know for sure that the map of functions is
a bottleneck, which almost never happens. It feels a bit heavy, and I'm not a big fan of
the extra abstraction unless it actually solves a real problem.

---

If you want to avoid global state in the map of func approach, wrap the map in a struct and
hang `Convert` off of it:

```go
package jpgconv

type Registry struct {
    converters map[string]ConverterFunc
}

func NewRegistry() *Registry {
    return &Registry{converters: make(map[string]ConverterFunc)}
}

func (r *Registry) Register(ext string, fn ConverterFunc) {
    r.converters[strings.ToLower(ext)] = fn
}

func (r *Registry) Convert(srcPath, dstPath string) error {
    ext := strings.ToLower(filepath.Ext(srcPath))

    fn, ok := r.converters[ext]
    if !ok {
        return fmt.Errorf("unsupported source format: %s", ext)
    }
    return fn(srcPath, dstPath)
}
```

<!-- references -->
<!-- prettier-ignore-start -->

[asked]:
    https://www.reddit.com/r/golang/comments/1s8j4qj/

[stab]:
    https://www.reddit.com/r/golang/comments/1s8j4qj/comment/odh77wi/

<!-- prettier-ignore-end -->
