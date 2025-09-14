---
title: Process substitution in Bash
date: 2023-04-30
tags:
    - Shell
    - TIL
---

I needed to compare two large directories with thousands of similarly named PDF files and
find the differing filenames between them. In the first pass, this is what I did:

Listed out the content of the first directory and saved it in a file:

```sh
ls dir1 > dir1.txt
```

Did the same for the second directory:

```sh
ls dir2 > dir2.txt
```

Compared the difference between the two outputs:

```sh
diff dir1.txt dir2.txt
```

This returned the name of the differing files likes this:

```txt
3c3,4
< f3.pdf
---
> f4.pdf
> f5.pdf
```

It does the job, but I asked BingChat if there's a better way to accomplish the task without
creating intermediate files, and it didn't let me down. Turns out that in Bash, process
substitution allows you to do just that. Instead of running three commands, you can achieve
the same result with a simple one-liner:

```bash
diff <(ls dir1) <(ls dir2)
```

## Process substitution

In Bash, process substitution is a feature that allows you to treat the output of a command
or commands as if it were a file. It enables you to use the output of a command as an input
to another command or perform other operations that expect file input or output.

- One important thing to point out is that process substitution is specific to Bash, Zsh,
  and certain versions of Ksh. Other shells and Bash in POSIX mode don't understand it.
  Bash, Zsh, and Ksh (88,93) support process substitution, but pdksh derivatives like mksh
  don't currently have this capability.\*

The syntax for process substitution is as follows:

- `<(command)`: This form allows you to use the output of a command as a file-like input.
- `>(command)`: This form allows you to use the output of a command as a file-like output.

When using process substitution, Bash creates a named pipe (FIFO) or a special file
descriptor `/dev/fd/<n>` behind the scenes. The command within the parentheses is executed,
and its output is redirected to the named pipe or file descriptor. Then, the path to the
named pipe or file descriptor is substituted into the original command line.

This is different from the plain-old `stdin` or `stdout` redirection. Here's how:

- **Input**
    - _Plain redirection_: When using plain stdin redirection (`<`), you can redirect input
      from a file, for example, `< input.txt`. The command reads the content of the file as
      standard input (stdin).
    - _Process substitution_: With process substitution, you can use the output of a command
      as input. For example, `command < <(echo "input")`. Here, the output of the `echo`
      command is treated as a file-like object and used as the input to `command`.

- **Output**
    - _Plain redirection_: Using plain stdout redirection (`>` or `>>`), you can redirect
      the output (stdout) of a command to a file, for example, `command > output.txt`. The
      command's output is written to the specified file.
    - _Process substitution_: With process substitution, you can use the output of a command
      as output. For example, `command >(process_output)`. Here, the output of `command` is
      treated as a file-like output, and it is passed as input to the `process_output`
      command or operation.

By using process substitution, the output of a command can be seamlessly integrated into
other commands as if it were a file, even if the command doesn't explicitly support stdin or
stdout redirection. This allows for greater compatibility and enables the use of the output
in situations where direct piping or redirection may not be possible.

## A few practical examples

### Inspecting the descriptors involved in process substitution

You can inspect the descriptor used by a process substitution like this:

```sh
echo >(true) <(false)
```

This returns:

```txt
/dev/fd/13 /dev/fd/11
```

Here, the expression `>(true)` creates a temporary file-like object, and the `true` command
serves as a placeholder for its input. Similarly, `<(false)` creates another temporary
file-like object with the false command serving as a placeholder for its output. When the
`echo` command is executed, it displays the filenames associated with these temporary
file-like objects, which are `/dev/fd/13` and `/dev/fd/11` in this specific scenario. These
filenames represent the underlying descriptors of the respective process substitutions,
indicating the file descriptors associated with the temporary objects created during the
process substitution.

### Calculating the total number of lines in a file

```sh
wc -l < <(cat input.txt)
```

This command calculates the total number of lines in the `input.txt` file. Here, the
`<(cat input.txt)` commad creates an input-type file descriptor containing the output of the
`cat` command and `wc -l` reads that content from there. The extra `<` redirects the
file-like object as an input stream again. This is a roundabout way of doing the following:

```sh
cat input.txt | wc -l
```

### Processing the content of a file line by line

```sh
while read line;
    do echo $line;
done < <(cat input.txt)
```

This command reads each line from the file `input.txt` and echoes it. It uses a `while` loop
with the read command to iterate over the lines, assigning each line to the variable line
and the `echo $line` command displays the line. Process substitution `<()` is used to treat
the output of cat `input.txt` as a temporary file, providing the input to the loop.

### Comparing directory sizes

```sh
diff -r <(du -sh dir1) <(du -sh dir2)
```

The command compares the disk usage of two directories, `dir1` and `dir2`, using the `diff`
command. The process substitution `<()` is employed to capture the output of the `du -sh`
command, which calculates the disk usage of each directory and provides a summary in a
human-readable format. The output of each `du -sh` command, representing the disk usage of
`dir1` and `dir2`, is treated as temporary files and passed as arguments to the `diff`
command. This enables the comparison of the disk usage between the two directories,
highlighting any discrepancies in file sizes or subdirectories.

### Picking or rejecting lines common between two sorted files

```sh
comm <(echo 'hello world\nhello mars' | sort) \
     <(echo 'hello world\nhello venus' | sort)
```

This returns:

```txt
hello mars
        hello venus
                hello world
```

This performs a comparison between the sorted outputs of two separate commands using `comm`.
The `com` command expects two files but we're using process substitution to make two
file-like objects from stdout.

Within the first process substitution `<()`, `echo` is used to generate a string containing
two lines: `hello world` and `hello mars`. This string is then piped to the `sort` command,
which sorts the lines alphabetically.

Similarly, the second part of the command `<()` uses process substitution as well. It
follows the same pattern as the first process substitution, but this time the string
contains `hello world` and `hello venus`.

The file-like objects containing the sorted output from the two process substitutions are
then passed as arguments to the `comm` command. Then `comm` compares the input files line by
line and generates three columns of output: lines unique to the first input, lines unique to
the second input, and lines common to both inputs.

[^1]: [Process substitution](https://tldp.org/LDP/abs/html/process-sub.html) [^1]
