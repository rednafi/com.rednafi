---
title: Testing Go CLIs with testscript
date: 2026-05-18
slug: testscript-cli
tags:
    - Go
    - Testing
    - CLI
    - Tooling
description: >-
    How cmd/go's script tests led me to testscript, and how to use it for CLI
    tests that exercise argv, stdout, stderr, exit codes, and scratch files.
---

While wrapping up [eon], I wanted to test the binary the same way a user would use it. The
test couldn't depend on whatever `eon` binary happened to be installed on the machine. I
also wanted to keep it inside `go test`, so unit and integration tests could run through the
same tooling.

eon is my CLI for scheduling jobs with LLMs. This command stores an hourly job named
`backup` and tells eon to run `echo hi` later:

```sh
eon add --cron '@hourly' --name backup -- echo hi
```

The `--cron` flag says when the job should run. `--name` gives it a stable name. Everything
after `--` is the command eon saves for later. Then `eon ls --json` lists the saved jobs as
JSON.

The unit tests already covered the code behind those commands: parsing schedules, writing
jobs, reading them back. The CLI can still break while those tests pass. `--cron` can parse
correctly and then get dropped before the job is saved. JSON output can change. An error can
go to stdout instead of stderr. A config lookup can touch my real home directory during a
test. Parser and store tests don't catch those failures.

I wanted the integration tests to:

- run `eon add`, `eon ls --json`, and a few invalid commands
- keep eon's files under a temporary home directory
- check stdout, stderr, exit codes, and saved state
- stay inside `go test`

I didn't know about testscript yet, so I started by reading how the Go project tests the
`go` command itself. That led me to [cmd/go's script tests]: `src/cmd/go/testdata/script`.
The directory is full of `.txt` fixtures for `go test`, `go build`, modules, workspaces,
vendoring, and other command-line behavior.

Those files are script fixtures. The Go command runs them with its own internal script
runner. The driver lives in [script_test.go], and these imports show the parts doing most of
the work:

```go
import (
    "internal/txtar"

    "cmd/internal/script"
    "cmd/internal/script/scripttest"
)
```

In that file, the test function is named `TestScript`. For every fixture, it roughly does
this:

- scans `testdata/script/*.txt`
- creates a temporary directory for the case
- exposes that directory to the script as `$WORK`
- sets `GOPATH` to `$WORK/gopath` and moves into `$WORK/gopath/src`
- parses the fixture as a txtar archive
- extracts the embedded files into `$WORK/gopath/src`
- runs the archive comment with Go's internal script engine

A shortened version of the driver looks like this. The comments and highlights are mine:

```go {hl_lines=["21","26","32","34","38"]}
func TestScript(t *testing.T) {
    engine := &script.Engine{
        Conds: scriptConditions(t),
        Cmds:  scriptCommands(quitSignal(), gracePeriod),
        Quiet: !testing.Verbose(),
    }

    files, err := filepath.Glob("testdata/script/*.txt")
    if err != nil {
        t.Fatal(err)
    }

    for _, file := range files {
        name := strings.TrimSuffix(filepath.Base(file), ".txt")
        workdir, err := os.MkdirTemp(testTmpDir, name)
        if err != nil {
            t.Fatal(err)
        }

        // This is the per-script work directory.
        s, err := script.NewState(tbContext(ctx, t), workdir, env)
        if err != nil {
            t.Fatal(err)
        }

        a, err := txtar.ParseFile(file)
        if err != nil {
            t.Fatal(err)
        }
        // initScriptDirs exposes workdir as $WORK, sets GOPATH to
        // $WORK/gopath, and chdirs to $WORK/gopath/src.
        telemetryDir := initScriptDirs(t, s)
        // The -- filename -- sections are extracted into $WORK/gopath/src.
        if err := s.ExtractFiles(a); err != nil {
            t.Fatal(err)
        }
        // The archive comment is the script body.
        scripttest.Run(t, engine, s, file, bytes.NewReader(a.Comment))
        checkCounters(t, telemetryDir)
    }
}
```

I covered txtar separately in [A tour of txtar], so I won't repeat the format here. For
these script tests, cmd/go uses the format this way:

- the text before the first `-- filename --` marker is the script body
- the sections after those markers are files
- those files get written under `$WORK/gopath/src` before the script runs

The [README] in that directory documents the same format.

A real fixture from the Go tree, trimmed from [test_regexps.txt], looks like this:

```txt
go test -cpu=1 -run=X/Y -bench=X/Y -count=2 -v testregexp

# TestX/Y is run, twice
stdout -count=2 '^=== RUN   TestX/Y$'

# TestZ is not run
! stdout '^=== RUN   TestZ$'

-- go.mod --
module testregexp

go 1.16
-- x_test.go --
package x
...
-- z_test.go --
package x
...
func TestZ(t *testing.T) {
    t.Logf("LOG: Z running")
}
```

> [!NOTE]
>
> Read that fixture as:
>
> - the command section runs `go test` and checks its output
> - `stdout -count=2` requires the regex to match twice
> - `! stdout` is the negative assertion, so `TestZ` must not appear
> - `go.mod`, `x_test.go`, and `z_test.go` are written into `$WORK/gopath/src`
>
> The `go` command works because the driver registers it with the script engine in
> [scriptcmds_test.go]. The fixture contains both the commands and the throwaway module.

Go's driver sits under internal packages, so normal projects can't import it. Roger Peppe
published the extracted public package in [go-internal]. The README traces testscript back
to Go's internal script package, and the package you import is [testscript]. That's the
package I used for eon.

Install it like any other test dependency:

```sh
go get github.com/rogpeppe/go-internal/testscript
```

Then point testscript at a directory of scripts:

```go {hl_lines=["3"]}
func TestScripts(t *testing.T) {
    testscript.Run(t, testscript.Params{
        Dir: "testdata/script",
    })
}
```

> [!TIP]
>
> The usual setup is:
>
> - put scripts under `testdata/script`
> - each `.txt` or `.txtar` file becomes a subtest
> - each subtest gets an isolated directory at `$WORK`
> - use `exec` to run a command
> - use `stdout` and `stderr` to assert regexes against the last command
> - use `cmp`, `env`, and `exists` when the filesystem or environment is part of the case
>
> The [testscript] docs cover the full syntax. The language isn't `/bin/sh`, so run `sh -c`
> explicitly when you need shell behavior.

## Testing a tiny CLI

Here's a tiny CLI called `hello`. It prints `hello, world` when you invoke it as `hello`,
and `-shout` uppercases the output:

```go
// main.go
package main

import (
    "flag"
    "fmt"
    "io"
    "os"
    "strings"
)

func main() {
    os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
    fs := flag.NewFlagSet("hello", flag.ContinueOnError)
    fs.SetOutput(stderr)

    shout := fs.Bool("shout", false, "uppercase output")
    if err := fs.Parse(args); err != nil {
        return 2
    }

    name := "world"
    if fs.NArg() > 0 {
        name = fs.Arg(0)
    }

    msg := "hello, " + name
    if *shout {
        msg = strings.ToUpper(msg)
    }
    fmt.Fprintln(stdout, msg)
    return 0
}
```

The test file registers `hello` as a command that scripts can execute. The highlighted lines
are the testscript wiring:

```go {hl_lines=["12-16","20-24"]}
// main_test.go
package main

import (
    "os"
    "testing"

    "github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
    testscript.Main(m, map[string]func(){
        "hello": func() {
            os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
        },
    })
}

func TestScripts(t *testing.T) {
    testscript.Run(t, testscript.Params{
        Dir:                 "testdata/script",
        RequireExplicitExec: true,
    })
}
```

Now add `testdata/script/greet.txt`:

```txt {hl_lines=["2-3","7-8","11-12"]}
# Default greeting.
exec hello
stdout '^hello, world$'
! stderr .

# Positional argument plus a flag.
exec hello -shout redowan
stdout '^HELLO, REDOWAN$'

# Bad flags should fail and print the flag package error.
! exec hello -bogus
stderr 'flag provided but not defined: -bogus'
```

Run it with:

```sh
go test ./...
```

I also put the example in a [playground version]. The playground runs the test binary in a
sandbox, so that version writes the script into `t.TempDir` before calling
`testscript.RunT`. In a normal project, keep the script under `testdata/script`.

To run only this script while iterating:

```sh
go test -run 'TestScripts/^greet$' -v
```

> [!IMPORTANT]
>
> `exec hello` doesn't use a system-wide `hello` binary.
>
> - `testscript.Main` puts its temp `bin` directory first in `PATH`
> - during `go test`, it copies the current test binary there as `hello`
> - `exec hello -shout redowan` starts that copied binary as a subprocess
> - the child process re-enters `testscript.Main`
> - `testscript.Main` dispatches by the basename of `os.Args[0]` and calls the registered
>   `"hello"` function
>
> So the test gets real argv, stdout, stderr, and exit status behavior without installing
> the CLI.

For longer output, put the expected text in the same script and compare against it:

```txt
exec hello -shout gopher
cmp stdout want

-- want --
HELLO, GOPHER
```

The `want` section is written into `$WORK` before the script starts. After `exec`,
`cmp stdout want` compares the previous command's stdout with that file and prints a diff on
failure.

## Using testscript in eon

The eon setup lives in [eon's script_test.go]. The [TestMain] block registers the real CLI
entrypoint as `eon`. It also registers a small `timeout` helper for the log-following daemon
script:

```go {hl_lines=["3-4"]}
func TestMain(m *testing.M) {
    testscript.Main(m, map[string]func(){
        "eon":     func() { os.Exit(runEonMain()) },
        "timeout": func() { os.Exit(runTimeoutMain()) },
    })
}
```

[runEonMain] builds the root command and runs it through the same Fang execution path as the
production binary:

```go {hl_lines=["6"]}
func runEonMain() int {
    ctx, cancel := signal.NotifyContext(
        context.Background(),
        syscall.SIGINT,
        syscall.SIGTERM,
    )
    defer cancel()

    root := newRoot()
    if err := fang.Execute(ctx, root, fangOptions()...); err != nil {
        return exitCode(err)
    }
    return 0
}
```

The [TestScripts setup] points eon's data directories at the script's `$WORK` directory:

```go {hl_lines=["5-9"]}
func TestScripts(t *testing.T) {
    testscript.Run(t, testscript.Params{
        Dir: "testdata/script",
        Setup: func(env *testscript.Env) error {
            env.Setenv("HOME", env.WorkDir)
            env.Setenv("XDG_DATA_HOME", env.WorkDir+"/xdg")
            env.Setenv("XDG_CONFIG_HOME", env.WorkDir+"/xdg-config")
            env.Setenv("CLICOLOR", "0")
            env.Setenv("NO_COLOR", "1")
            return nil
        },
    })
}
```

> [!NOTE]
>
> eon stores jobs in SQLite under the platform data directory. During tests, I point all of
> those paths at `$WORK`:
>
> - `HOME` under `$WORK`
> - `XDG_DATA_HOME` under `$WORK/xdg`
> - `XDG_CONFIG_HOME` under `$WORK/xdg-config`
> - color disabled for stable stdout and stderr assertions
>
> The scripts can add jobs, list them, and read logs without touching my real scheduler
> state.

One eon script, [add_basic.txt], covers the add/list/show path:

```txt {hl_lines=["1-2","12-15","18-19","21-23"]}
exec eon add --cron '@hourly' --name backup -- echo hi
stdout 'added job [0-9A-Za-z]+ \(cron, @hourly\)'

exec eon add --at '+1h' --name morning -- echo wake
stdout 'added job [0-9A-Za-z]+ \(oneshot, at .*\)'

# Name defaults to the command when --name is omitted.
exec eon add --cron '@daily' -- /bin/echo from-cmd
stdout 'added job [0-9A-Za-z]+'

# Three jobs created — verify by name (IDs are random 5-char strings).
exec eon ls --json
stdout '"name": "backup"'
stdout '"name": "morning"'
stdout '"cron": "@hourly"'

# Show resolves by name.
exec eon show backup --json
stdout '"kind": "cron"'

exec eon show morning --json
stdout '"kind": "oneshot"'
stdout '"fire_at"'
```

With that in place, `go test ./...` covers the CLI behavior I care about:

- parser tests exercise schedule parsing directly
- store tests hit SQLite APIs directly
- testscript tests cover flags, output, exit codes, and state written under an isolated home
  directory

The tests don't install eon or pick up a stale command from `PATH`. They still run the
command as a subprocess, so argv, stdout, stderr, and exit codes go through the same code
path a user hits in a terminal.

<!-- references -->
<!-- prettier-ignore-start -->

[eon]:
    https://github.com/rednafi/eon

[A tour of txtar]:
    /go/txtar/

[eon's script_test.go]:
    https://github.com/rednafi/eon/blob/857935f9fe411dce7a5b306d5b898397fdac87e5/cmd/eon/script_test.go#L18-L92

[TestMain]:
    https://github.com/rednafi/eon/blob/857935f9fe411dce7a5b306d5b898397fdac87e5/cmd/eon/script_test.go#L18-L25

[runEonMain]:
    https://github.com/rednafi/eon/blob/857935f9fe411dce7a5b306d5b898397fdac87e5/cmd/eon/script_test.go#L27-L38

[TestScripts setup]:
    https://github.com/rednafi/eon/blob/857935f9fe411dce7a5b306d5b898397fdac87e5/cmd/eon/script_test.go#L76-L92

[add_basic.txt]:
    https://github.com/rednafi/eon/blob/857935f9fe411dce7a5b306d5b898397fdac87e5/cmd/eon/testdata/script/add_basic.txt#L3-L25

[cmd/go's script tests]:
    https://go.dev/src/cmd/go/testdata/script/

[script_test.go]:
    https://go.dev/src/cmd/go/script_test.go

[scriptcmds_test.go]:
    https://go.dev/src/cmd/go/scriptcmds_test.go#L20-L43

[README]:
    https://go.dev/src/cmd/go/testdata/script/README

[test_regexps.txt]:
    https://go.dev/src/cmd/go/testdata/script/test_regexps.txt

[go-internal]:
    https://github.com/rogpeppe/go-internal

[testscript]:
    https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript

[playground version]:
    https://go.dev/play/p/eW2KHO8Ir-_1

<!-- prettier-ignore-end -->
