---
title: "Accepted proposal: range over all command-line flags"
slug: go-flag-all-is-set
date: 2026-07-30T00:00:00+02:00
description: >-
    The flag package is getting a range-friendly iterator and a direct way to tell whether a
    flag was set successfully.
tags:
    - Go
    - CLI
    - API
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /shards/2026/07/go-flag-all-is-set/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mrszl4q7uq2l"
---

Go's [proposal for package flag] was [accepted] on July 23, 2026. It fixes an awkward case
in programs that read the same setting from command-line flags, environment variables, and
defaults.

Suppose a server has a `-debug` flag whose default is false:

```go
fs := flag.NewFlagSet("serve", flag.ContinueOnError)
debug := fs.Bool("debug", false, "enable debug logging")

if err := fs.Parse(args); err != nil {
    return err
}
```

The server also reads `APP_DEBUG`. A command-line flag should win over the environment
variable, and the environment variable should win over the default. If the user runs the
server with `-debug=false` while `APP_DEBUG=true`, the server must keep debug mode off. But
`*debug` is false both when the flag was omitted and when the user explicitly set it to
false. The parsed value alone cannot distinguish those cases.

The `flag` package knows which flags were set, but today it exposes that information through
two callbacks. `Visit` walks only the flags that were set successfully. `VisitAll` walks
every declared flag. Code that fills unset flags from the environment has to combine both:

```go
func applyEnv(fs *flag.FlagSet) error {
    alreadySet := make(map[string]bool)
    fs.Visit(func(f *flag.Flag) {
        alreadySet[f.Name] = true
    })

    var applyErr error
    fs.VisitAll(func(f *flag.Flag) {
        if applyErr != nil || alreadySet[f.Name] {
            return
        }

        key := "APP_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
        value, ok := os.LookupEnv(key)
        if !ok {
            return
        }
        if err := fs.Set(f.Name, value); err != nil {
            applyErr = fmt.Errorf("set flag -%s from %s: %w", f.Name, key, err)
        }
    })
    return applyErr
}
```

The first pass records the flags found by `Visit`. The second pass visits everything and
looks up an environment variable only when the name is absent from that map. `applyErr` sits
outside the callback because the callback cannot return an error from `applyEnv`. The [flagx
environment-variable helper] follows this two-pass approach.

The proposal adds the set state to each `Flag` and provides an iterator over all flags:

```go
type Flag struct {
    // existing fields
    IsSet bool
}

func All() iter.Seq[*Flag]
func (*FlagSet) All() iter.Seq[*Flag]
```

The same helper becomes one loop:

```go
func applyEnv(fs *flag.FlagSet) error {
    for f := range fs.All() {
        if f.IsSet {
            continue
        }

        key := "APP_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
        value, ok := os.LookupEnv(key)
        if !ok {
            continue
        }
        if err := fs.Set(f.Name, value); err != nil {
            return fmt.Errorf("set flag -%s from %s: %w", f.Name, key, err)
        }
    }
    return nil
}
```

`fs.All()` yields every flag in name order. `f.IsSet` says whether parsing or a call to
`fs.Set` successfully assigned that flag. An explicit `-debug=false` therefore has
`IsSet == true`, so `APP_DEBUG` cannot overwrite it. If `-debug` was omitted, the loop reads
`APP_DEBUG` and passes its text through the flag's normal parser. With neither source
present, the original default remains.

The side map, the first traversal, and the callback error variable disappear. The loop can
return an error directly. `flag.All()` provides the same iteration for the package-level
`flag.CommandLine`.

The [implementation] has passed the Go trybots and is still under review. It has not landed
on the main branch or appeared in a tagged release yet.

<!-- references -->
<!-- prettier-ignore-start -->

[proposal for package flag]: https://github.com/golang/go/issues/65675

[accepted]:
    https://github.com/golang/go/issues/65675#issuecomment-5063363811

[flagx environment-variable helper]:
    https://github.com/earthboundkid/flagx/blob/778fa09f26526355877978225c83f075303836e6/env.go#L10-L34

[implementation]:
    https://go-review.googlesource.com/c/go/+/804020

<!-- prettier-ignore-end -->
