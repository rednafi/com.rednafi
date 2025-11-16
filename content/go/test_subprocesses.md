---
title: Re-exec testing Go subprocesses
date: 2025-11-16
slug: test-subprocesses
tags:
    - Go
---

When testing Go code that spawns subprocesses, you usually have three options.

**Run the real command.** It invokes the actual binary that creates the subprocess and
asserts against the output. However, that makes tests slow and tied to the environment. You
have to make sure the same binary exists and behaves the same everywhere, which is harder
than it sounds.

**Fake it.** Mock the subprocess to keep tests fast and isolated. The problem is that the
fake version doesn't behave like a real process. It won't fail, write to stderr, or exit
with a non-zero code. That makes it hard to trust the result, and over time the mock can
drift away from what the real command actually does.

**Re-exec.** I picked this neat trick from Go's [stdlib `os/exec` tests]. With re-exec, your
test binary spawns a new subprocess that runs itself again. Inside that subprocess, the code
emulates the behavior of the real command. The parent process then interacts with this
subprocess exactly as it would with a real command. In short:

- The parent test process spawns the subprocess.
- The subprocess emulates the behavior of the target command.
- The parent process interacts with the emulated subprocess as if it were the real command.

This setup makes re-exec a middle ground between mocking and invoking the actual subprocess.

The first two paths are well-trodden, so let's look closer at the third one. Here's how it
works:

- The test re-launches itself with a special flag or environment variable to signal it's
  running in "child" mode.
- In this mode, it acts as the subprocess and can print output, write to stderr, or exit
  with any code you want. This subprocess basically emulates the real command's subprocess.
- The main test process then runs as usual and interacts with it just like it would with a
  real subprocess.

You still get a real subprocess, but the behavior of your original binary invocation is
emulated inside it. So you don't invoke the original command. Observe:

```go
// /cmd/echo/main.go
package main

import (
    "os/exec"
)

// RunEcho executes the system "echo" command with the provided message
// and returns the command's output.
func RunEcho(msg string) (string, error) {
    cmd := exec.Command("echo", msg)
    out, err := cmd.Output()
    return string(out), err
}
```

`RunEcho` invokes the system's `echo` binary with some argument and returns the output. Now
let's test it using the re-exec trick:

```go
// /cmd/echo/main_test.go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "testing"
)

// TestEchoHelper runs when the binary is re-executed with
// GO_WANT_HELPER_PROCESS=1. It prints its argument and exits,
// emulating "echo".
func TestEchoHelper(t *testing.T) {
    if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
        return
    }
    fmt.Print(os.Args[len(os.Args)-1])
    os.Exit(0)
}

func TestRunEcho(t *testing.T) {
    // Spawn the same test binary as a subprocess instead of calling the
    // real "echo". This runs only the TestEchoHelper test in a subprocess
    // which emulates the behavior of "echo"
    cmd := exec.Command(os.Args[0], "-test.run=TestEchoHelper", "--", "hello")
    cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

    out, err := cmd.Output()
    if err != nil {
        t.Fatal(err)
    }
    if string(out) != "hello" {
        t.Fatalf("got %q, want %q", out, "hello")
    }
}
```

`TestRunEcho` creates a command that re-runs the same test binary (`os.Args[0]`) as a
subprocess via the `exec.Command`. The `-test.run=TestEchoHelper` flag tells Go's test
runner to execute only the `TestEchoHelper` function inside that new process. The `"--"`
marks the end of the test runner's own flags, and everything after it (`"hello"`) becomes an
argument available to the helper process in `os.Args`.

When this subprocess starts, it sees that the environment variable
`GO_WANT_HELPER_PROCESS=1` is set. That tells it to behave like a helper instead of running
the full test suite. The `TestEchoHelper` function then prints its last argument (`"hello"`)
to standard output and exits. In other words, we're emulating `echo` inside
`TestEchoHelper`. This part is intentionally kept simple, but you can do all kinds of things
here to emulate the actual `echo` command. In real tests, this will also include different
failure modes.

From the parent process's perspective, it looks just like running `/bin/echo hello`, except
everything is happening within the Go test binary. The subprocess is real, but its behavior
is entirely controlled by the test.

You might find it strange that the actual `RunEcho` function isn't called anywhere. That's
on purpose. The goal of this example is not to test production logic, but to show how to
emulate and control subprocesses inside a test environment. The production function here
doesn't contain any logic beyond calling `exec.Command`, so there's nothing meaningful to
verify yet.

In real code, typically, you'd split subprocess management into two parts: one that spawns
the process and another that handles its output and errors. The handler is where the bulk of
your logic should live. This way, the subprocess handling code can be tested in isolation
without having to tie it with a real subprocess.

Consider this example where the production code invokes the `git switch mybranch` command.
The `RunGitSwitch` command calls the `git` binary with the appropriate arguments and passes
the `*exec.Cmd` pointer to the `handleGitSwitch` function. This handler function has the
bulk of the logic that interacts with the git subprocess.

```go
// path: /cmd/git/main.go
package main

import (
    "os/exec"
)

// handleGitSwitch runs a command and returns its combined output and error.
func handleGitSwitch(cmd *exec.Cmd) (string, error) {
    out, err := cmd.CombinedOutput()
    return string(out), err
}

// RunGitSwitch constructs the subprocess to run "git switch".
func RunGitSwitch(branch string) (string, error) {
    cmd := exec.Command("git", "switch", branch)
    return handleGitSwitch(cmd)
}
```

And the corresponding test:

```go
// path: /cmd/git/main_test.go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "testing"
)

// TestGitSwitchHelper acts as the fake "git switch" subprocess.
func TestGitSwitchHelper(t *testing.T) {
    if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
        return
    }
    // Emulate "git switch" output.
    fmt.Printf("Switched to branch '%s'\n", os.Args[len(os.Args)-1])
    os.Exit(0)
}

func TestGitSwitch(t *testing.T) {
    cmd := exec.Command(
        os.Args[0],
        "-test.run=TestGitSwitchHelper", "--", "feature-branch",
    )
    cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

    // This time we're invoking the production handler.
    out, err := handleGitSwitch(cmd)
    if err != nil {
        t.Fatal(err)
    }

    want := "Switched to branch 'feature-branch'\n"
    if out != want {
        t.Fatalf("got %q, want %q", out, want)
    }
}
```

In this test, the subprocess behavior (`git switch`) is emulated by `TestGitSwitchHelper`.
The helper prints predictable output that mimics the real command, but the subprocess itself
is still a separate process spawned by the parent test.

What's under test here is `handleGitSwitch`, which manages subprocess execution, reads its
output, and handles errors. The subprocess is fake in behavior but real in execution, which
means the I/O boundaries are still exercised.

This separation between subprocess creation and handling keeps tests focused and repeatable.
You can emulate different subprocess outcomes, such as errors or unexpected output, while
keeping the process interaction logic untouched.

<!-- References -->

<!-- prettier-ignore-start -->

[stdlib `os/exec` tests]:
    https://cs.opensource.google/go/go/+/refs/tags/go1.25.4:src/os/exec/exec_test.go

<!-- prettier-ignore-end -->
