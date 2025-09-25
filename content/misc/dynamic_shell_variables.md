---
title: Dynamic shell variables
date: 2025-01-11
slug: dynamic-shell-variables
aliases:
    - /misc/dynamic_shell_variables/
tags:
    - Shell
    - TIL
---

I came across a weird shell syntax today—dynamic shell variables. It lets you dynamically
construct and access variable names in Bash scripts, which I haven't encountered in any of
the mainstream languages I juggle for work.

In an actual programming language, you'd usually use a hashmap to achieve the same effect,
but directly templating variable names is a quirky shell feature that sometimes comes in
handy.

## A primer

Dynamic shell variables allow shell scripts to define and access variables based on runtime
conditions. Variable indirection (`${!var}` syntax) lets you reference the value of a
variable through another variable. This can be useful for managing environment-specific
configurations and function dispatch mechanisms.

Here's an example:

```sh
#!/usr/bin/env bash
# script.sh

config_path="/etc/config"
var="config_path"

echo "The value of \$config_path is: ${!var}"
```

```txt
The value of $config_path is: /etc/config
```

Here, `${!var}` resolves to the value of the variable `config_path` because `var` contains
its name. This allows you to dynamically decide which variable to reference at runtime.

## Context-aware environment management

A more practical use of dynamic shell variables is managing environment-specific
configurations. This is particularly handy in scenarios where you have multiple environments
like `staging` and `prod`, each with its own unique configuration settings.

```sh
#!/usr/bin/env bash
# script.sh

# Define environment-specific configurations dynamically
declare staging_URL="https://staging.example.com"
declare staging_PORT=8081

declare prod_URL="https://example.com"
declare prod_PORT=80

# Set the current environment
env=$1

# Validate input
if [[ "$env" != "staging" && "$env" != "prod" ]]; then
  echo "Invalid environment. Please specify 'staging' or 'prod'."
  exit 1
fi

# Dynamically access the environment-specific variables
URL="${env}_URL"
PORT="${env}_PORT"

echo "URL: ${!URL}"
echo "Port: ${!PORT}"
```

Run the script with an environment as the argument:

```sh
./script.sh staging
```

Output for `env="staging"`:

```txt
URL: https://staging.example.com
Port: 8081
```

By passing the environment as an argument, you can switch between environments without
duplicating configuration logic.

One gotcha to be aware of is that appending text directly to the `${!VAR}` syntax (e.g.,
`${!env}_URL`) doesn't produce the intended results. Instead of resolving `staging_URL`,
this line will print only `_URL`:

```sh
echo "${!env}_URL"
```

Output:

```txt
_URL
```

This happens because `${!VAR}` only resolves the value of `VAR` and doesn't support direct
concatenation. To avoid this, construct the full variable name (`URL="${env}_URL"`) before
using `${!VAR}` for indirect expansion. This ensures the correct variable is accessed.

## Function dispatch

Another neat use case for dynamic variables is function dispatch—calling the appropriate
function based on runtime conditions. This technique can be used to simplify scripts that
need to handle multiple services or operations.

```sh
#!/usr/bin/env bash
# script.sh

# Define functions for operations on different services

web_start() {
    echo "Starting web service..."
}

web_stop() {
    echo "Stopping web service..."
}

db_status() {
    echo "Checking database status..."
}

# Dynamically bind operation to function
declare web_start_function="web_start"
declare web_stop_function="web_stop"
declare db_status_function="db_status"

# Input variables for service and operation
service=$1
operation=$2

# Build dynamic function name
func="${service}_${operation}_function"

# Dispatch function dynamically
if [[ $(type -t ${!func}) == "function" ]]; then
    ${!func}  # Call the dynamically resolved function
else
    echo "Unknown operation: $service $operation"
fi
```

Run the script with service and operation as arguments:

```sh
./script.sh web start
```

This returns:

```txt
Starting web service...
```

Similarly, running `./script.sh db status` prints:

```txt
Checking database status...
```

## Temporary file handling

Dynamic variables can also help manage temporary files or logs in scripts that process
multiple datasets. By dynamically generating variable names, you can track temporary file
paths for each dataset without conflicts.

```sh
#!/usr/bin/env bash
# script.sh

# Process multiple datasets with temporary files
for dataset in data1 data2 data3; do
    # Dynamically declare a temporary file variable
    temp_file_var="${dataset}_temp_file"
    declare $temp_file_var="/tmp/${dataset}_processing.tmp"

    # Simulate processing and logging
    echo "Processing $dataset..." > ${!temp_file_var}
    cat ${!temp_file_var}

    # Clean up (or add a trap to make this more robust)
    rm -f ${!temp_file_var}
done
```

Running this prints the following:

```txt
Processing data1...
Processing data2...
Processing data3...
```

Here, each dataset gets a unique temporary file, managed dynamically by the script. It
eliminates the need for manually creating and tracking file names.

This works, but like everything else in shell scripts, it can quickly turn into a hairball
if we're not careful. While the syntax is nifty, I find it a bit hard to read at times!
