---
title: Returning values from a shell function
date: 2022-09-25
slug: return-values-from-a-shell-function
aliases:
    - /misc/return_values_from_a_shell_function/
tags:
    - Shell
    - TIL
---

TIL that returning a value from a function in bash doesn't do what I thought it does.
Whenever you call a function that's returning some value, instead of giving you the value,
Bash sets the return value of the callee as the status code of the calling command. Consider
this example:

```bash
#!/usr/bin/bash
# script.sh

return_42() {
    return 42
}

# Call the function and set the return value to a variable.
value=$return_42

# Print the return value.
echo $value
```

I was expecting this to print out `42` but instead it doesn't print anything to the console.
Turns out, a shell function doesn't return the value when it encounters the `return`
keyword. Rather, it stops the execution of the function and sets the status code of the last
command in the function as the value that the function returns.

To test it out, you can print out the status code of the last command when a script exits
with `echo $?`. Here's the same snippet from the previous section where the last line is the
command that calls the `return_42` function:

```bash
#!/usr/bin/bash
# script.sh

return_42() {
    return 42
}

# Call the function.
return_42
```

Run the snippet and print the exit code of the last line of the script with the following
command:

```sh
./script.sh; echo $?
```

This prints out:

```txt
42
```

## Status code evaluation pattern

Here's one pattern that you can use whenever you need to return a value from a shell
function. In the following snippet, I'm evaluating whether a number provided by the user is
a prime or not and printing out a message accordingly:

```bash
#!/usr/bin/bash
# script.sh

# Check whether a number is prime or not.
is_prime(){
    factor_count=$(factor $1 | wc -w)

    if [[ $factor_count -eq 2 ]]; then
        return 0 # Sets the status code to 0.
    else
        return 1 # Any non-zero value will work here.
    fi
}

# Call the function.
is_prime $1

# Inspect the status code.
status=$?

# Print message according to the status code.
if [[ $status -eq 0 ]]; then
    echo "$1 is prime."
else
    echo "$1 is not prime."
fi
```

Since the returned values are treated as status codes where `0` is used to denote no error
and a non-zero value represents an error, you'll need to return `0` as a truthy value and
`1` as a falsy value. While this works, returning `0` to denote a truthy value is the
opposite of what you'd usually do in other programming languages and can confuse someone who
might not be familiar with shell quirks. If you only need to return a boolean value from a
function, here's a better pattern:

```bash
#!/usr/bin/bash
# script.sh

# Check whether a number is prime or not.
is_prime(){
    factor_count=$(factor $1 | wc -w)

    if [[ $factor_count -eq 2 ]]; then
        true
    else
        false
    fi
}

# Call the function.
is_prime $1

# Inspect the status code.
status=$?

# Print message according to the status code.
if [[ $status -eq 0 ]]; then
    echo "$1 is prime."
else
    echo "$1 is not prime."
fi
```

In this snippet, notice how the `is_prime` function doesn't explicitly return anything.
Instead, it just adds the `true` or `false` expression to the end of the return path
accordingly. This implicitly sets the status code to `0` when the input number is a prime
and to `1` when it's not. The rest of the status checking works the same as in the previous
script.

The second pattern won't work if you need to set the status code to something other than `0`
or `1`. In that case you can resort the first pattern without confusing anyone.

[^1]:
    [Returning a boolean from a Bash function](https://stackoverflow.com/questions/5431909/returning-a-boolean-from-a-bash-function/43840545#43840545)
    [^1]
