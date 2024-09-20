---
title: Bash namerefs for dynamic variable referencing
date: 2024-09-19
tags:
  - Shell
  - TIL
---

While going through a script at work today, I came across Bash's `nameref` feature. It uses
`declare -n ref="$1"` to set up a variable that allows you to reference another variable by
name—kind of like pass-by-reference in C. I'm pretty sure I've seen it before, but I
probably just skimmed over it.

As I dug into the man page[^1], I realized there's a gap in my understanding of how variable
references actually work in Bash—probably because I never gave it proper attention and just
got by cobbling together scripts.

## Namerefs

By default, Bash variables are global unless declared as `local` within a function. However,
when you pass variables as arguments to a function, they are accessed via positional
parameters like `$1`, `$2`, etc., and any changes to these parameters inside the function do
not affect the original variables outside the function.

Namerefs allow you to essentially define a pointer to another variable. By creating a
nameref, you can indirectly reference and manipulate the target variable without knowing its
name beforehand. This is incredibly useful for writing generic functions that can operate on
different variables based on input parameters.

## Basic usage

Here's an example:

```sh
#!/usr/bin/env bash

# Declare a variable
original_var="Hello, World!"

# Function that creates a nameref to a variable
create_ref() {
    local ref_name=$1
    declare -n ref="$ref_name"
    ref="Hello from nameref!"
}

# Call the function with the name of the variable
create_ref original_var

# Print the updated variable
echo "$original_var"
```

Running this will print:

```txt
Hello from nameref!
```

By running the `create_ref` function, we can dynamically update the value of
`$original_var`, which exists outside of it. Notice that the function doesn't even need to
know about `$original_var`; it works on any variable name provided, making it generic.

In this script:

-   We declare a variable `original_var` with the value `"Hello, World!"`.
-   The `create_ref` function takes the name of a variable as an argument.
-   Inside the function, `declare -n ref="$ref_name"` creates a nameref `ref` that points to
    the variable named by `$ref_name`.
-   By setting `ref="Hello from nameref!"`, we indirectly update `original_var`.
-   Finally, we print `original_var` to see the updated value.

Without the nameref, you could achieve the same thing with this `eval` (read: evil) trick:

```sh
#!/usr/bin/env bash

# Declare a variable
original_var="Hello, World!"

# Function that updates a variable dynamically using eval
create_ref() {
    local var_name=$1
    local new_value="Hello from eval!"
    eval "$var_name=\"$new_value\""  # eval 😈
}

# Call the function with the name of the variable
create_ref original_var

# Print the updated variable
echo "$original_var"
```

This achieves the same result. The `eval "$var_name=\"$new_value\""` dynamically updates the
`$original_var` variable through `$var_name`. However, `eval` can be risky for security, and
the nameref approach looks much cleaner syntactically.

## Managing multiple arrays

Namerefs shine when you need to manage multiple arrays dynamically. Consider a scenario
where you have several datasets stored in different arrays, and you want to process them
using a single function.

```sh
#!/usr/bin/env bash

# Declare multiple arrays
declare -a dataset1=(1 2 3 4 5)
declare -a dataset2=(10 20 30 40 50)
declare -a dataset3=(100 200 300 400 500)

# Function to calculate the sum of an array
sum_array() {
    local array_name=$1
    declare -n arr="$array_name"
    local sum=0
    for num in "${arr[@]}"; do
        sum=$((sum + num))
    done
    echo "Sum of $array_name: $sum"
}

# Process each dataset
sum_array dataset1
sum_array dataset2
sum_array dataset3
```

This returns:

```txt
Sum of dataset1: 15
Sum of dataset2: 150
Sum of dataset3: 1500
```

Here:

-   We declare three arrays: `dataset1`, `dataset2`, and `dataset3`.
-   The `sum_array` function takes the name of an array as an argument.
-   Using `declare -n arr="$array_name"`, we create a nameref `arr` that points to the
    specified array.
-   We then iterate over the elements of `arr` to calculate the sum.
-   Finally, we call `sum_array` for each dataset, and the function correctly processes each
    array based on the reference.

Without the nameref, you could again use the `eval` trick to achieve the same thing, but
this time it looks even uglier:

```sh
#!/usr/bin/env bash

# Declare multiple arrays
declare -a dataset1=(1 2 3 4 5)
declare -a dataset2=(10 20 30 40 50)
declare -a dataset3=(100 200 300 400 500)

# Function to calculate the sum of an array without namerefs
sum_array() {
    local array_name=$1
    local sum=0
    local index=0
    local array_length
    eval "array_length=\${#$array_name[@]}"
    for (( index=0; index<array_length; index++ )); do
        eval "element=\${$array_name[$index]}"
        sum=$((sum + element))
    done
    echo "Sum of $array_name: $sum"
}

# Process each dataset
sum_array dataset1
sum_array dataset2
sum_array dataset3
```

This approach is more complex, less secure, and harder to read in general. But the above
`eval` example was a bit contrived to make it look bad. You can achieve the same thing
without `eval` or nameref in this particular case like this:

```sh
#!/usr/bin/env bash

# Declare multiple arrays
dataset1=(1 2 3 4 5)
dataset2=(10 20 30 40 50)
dataset3=(100 200 300 400 500)

# Function to calculate the sum of an array
sum_array() {
    local sum=0
    for element in "$@"; do
        sum=$((sum + element))
    done
    echo "Sum: $sum"
}

# Process each dataset
sum_array "${dataset1[@]}"
sum_array "${dataset2[@]}"
sum_array "${dataset3[@]}"
```

Here, instead of passing the name of the dataset arrays as strings, we pass the elements of
the array to the function and add them. But I digress!

## Associative arrays and nested references

Namerefs also work with associative arrays and can be used for more complex data structures.

```sh
#!/usr/bin/env bash

# Declare an associative array
declare -A user_info=(
    [name]="Alice"
    [age]=30
    [email]="alice@example.com"
)

# Function to update user information
update_info() {
    local info_name=$1
    local key=$2
    local new_value=$3
    declare -n info="$info_name"
    info[$key]=$new_value
}

# Update the user's email
update_info user_info email "alice@newdomain.com"

# Print updated information
for key in "${!user_info[@]}"; do
    echo "$key: ${user_info[$key]}"
done
```

It prints:

```txt
name: Alice
age: 30
email: alice@newdomain.com
```

And voilà! We have a function that can dynamically update the values in an associative
array. This technique is useful for changing environments or contexts (staging/production)
in shell scripts.

In this example:

-   We declare an associative array `user_info` containing user details.
-   The `update_info` function takes the name of the associative array, the key to update,
    and the new value.
-   Using `declare -n info="$info_name"`, we create a nameref `info` pointing to
    `user_info`.
-   We update the specified key in the array.
-   Finally, we echo the updated user information.

Doing this with `eval` isn't pretty. I'll leave that as an exercise for you if you like to
torment yourself.

## Implementing generic setter and getter functions

Building on the earlier examples, you can use namerefs to create generic setter and getter
functions, making it easier to manage configuration variables or environment settings in
scripts.

Here's an example:

```sh
#!/usr/bin/env bash

# Generic setter function
set_var() {
    local var_name="$1"
    local value="$2"
    declare -n ref="$var_name"
    ref="$value"
}

# Generic getter function
get_var() {
    local var_name="$1"
    declare -n ref="$var_name"
    echo "$ref"
}

# Usage example

env="staging"  # Can be passed as an argument to the script

# Define default variables
db_host="localhost"
db_port=5432
db_user="admin"
db_pass="secret"

# Set different values based on the environment
if [[ "$env" == "production" ]]; then
  set_var "db_host" "prod.db.example.com"
  set_var "db_user" "prod_admin"
elif [[ "$env" == "staging" ]]; then
  set_var "db_host" "staging.db.example.com"
  set_var "db_user" "staging_admin"
fi

# Retrieve and display values
echo "Using Database: $(get_var "db_host")"
echo "Database User: $(get_var "db_user")"
```

To keep things simple, the `env` variable isn't a CLI argument. Based on whether `env` is
set to staging or production, the script will print the relevant database values.

For `staging`, you'll see:

```txt
Using Database: staging.db.example.com
Database User: staging_admin
```

For `production`:

```txt
Using Database: prod.db.example.com
Database User: prod_admin
```

Oh, one extra thing: nameref was introduced in Bash 4.3, so you might run into problems if
you're using an ancient version like the one shipped with macOS.

[^1]:
    [Shell Builtin Commands - Bash Reference Manual](https://www.gnu.org/software/bash/manual/bash.html#Shell-Builtin-Commands)
