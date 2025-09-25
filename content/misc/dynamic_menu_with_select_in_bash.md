---
title: Dynamic menu with select statement in Bash
date: 2023-04-29
slug: dynamic-menu-with-select-in-bash
aliases:
    - /misc/dynamic_menu_with_select_in_bash/
tags:
    - Shell
    - TIL
---

Whenever I need to whip up a quick command line tool, my go-to is usually Python. Python's
CLI solutions tend to be more robust than their Shell counterparts. However, dealing with
its portability can sometimes be a hassle, especially when all you want is to distribute a
simple script. That's why while toying around with `argparse` to create a dynamic menu, I
decided to ask ChatGPT if there's a way to achieve the same using native shell scripting.
Delightfully, it introduced me to the dead-simple `select` command that I probably should've
known about years ago. But I guess better late than never! Here's what I was trying to
accomplish:

_Print a menu that allows a user to choose an option and then trigger a specific function
associated with the chosen option. When you run the script, it should present you with
something similar to this:_

```txt
1) Option 1
2) Option 2
3) Option 3
4) Quit
Please enter your choice: 1
You selected Option 1.
Please enter your choice: 2
You selected Option 2.
Please enter your choice: 3
You selected Option 3.
Please enter your choice: 4
```

Whenever the user selects an option, the script dispatches an associated function with the
option. Currently, the associated function just prints `You selected option x` but it has
the freedom to do whatever it wants. The following native Shell script uses `select` to
produce the output above:

```bash
#!/usr/bin/env bash
# script.sh

# Define an array of menu options
options=("Option 1" "Option 2" "Option 3" "Quit")

# Function to handle Option 1
function option1 {
    echo "You selected Option 1."
    # Add your Option 1 code here
}

# Function to handle Option 2
function option2 {
    echo "You selected Option 2."
    # Add your Option 2 code here
}

# Function to handle Option 3
function option3 {
    echo "You selected Option 3."
    # Add your Option 3 code here
}

# Display the menu and process user selection
PS3="Please enter your choice: "
select option in "${options[@]}"; do
    case $option in
        "Option 1")
            option1
            ;;
        "Option 2")
            option2
            ;;
        "Option 3")
            option3
            ;;
        "Quit")
            break
            ;;
        *)
            echo "Invalid option. Try again."
            ;;
    esac
done
```

The snippet allows users to make selections from a list of options. It starts by defining an
array called `options` which holds the available possibilities. Each option corresponds to a
specific function, such as `option1`, `option2`, and `option3` which can be customized to
perform specific actions or tasks.

The script then prompts the user to enter their choice using the `select` statement. The
user's selection is stored in the variable `option`. Then it uses a case statement to match
the selected option and execute the corresponding function. For example, if the user chooses
`Option 1` by typing `1` into the console, the script calls the `option1` function and
displays a message confirming the selection. The same applies to `Option 2` and `Option 3`.
If the user selects `Quit` by typing `4`, the script breaks out of the loop and terminates.
Moreover, if the user enters an invalid option, the script displays an error message
indicating that the option is not recognized and prompts the user to try again.

Here's a little more useful script to run some common Docker commands based on the user's
selection. The script assumes that Docker engine is installed on the targeted system:

```bash
#!/usr/bin/env bash

# Define an array of menu options
options=(
"Show Docker Images"
"Remove Docker Image"
"Show Docker Containers"
"Remove Docker Container"
"Stop All Containers"
"Reprint Options"
"Quit"
)

# Function to show all Docker images
function show_docker_images {
    echo "Listing Docker images:"
    docker images
}

# Function to remove a Docker image
function remove_docker_image {
    read -p "Enter the image ID or name to remove: " image_id
    echo "Removing Docker image: $image_id"
    docker rmi "$image_id"
}

# Function to show all Docker containers
function show_docker_containers {
    echo "Listing Docker containers:"
    docker ps -a
}

# Function to remove a Docker container
function remove_docker_container {
    read -p "Enter the container ID or name to remove: " container_id
    echo "Removing Docker container: $container_id"
    docker rm "$container_id"
}

# Function to stop all Docker containers
function stop_all_containers {
    echo "Stopping all Docker containers:"
    docker stop $(docker ps -aq)
}

# Function to reprint the options
function reprint_options {
    echo "Available Options:"
    for index in "${!options[@]}"; do
        echo "$((index+1))) ${options[index]}"
    done
}

# Display the menu and process user selection
PS3="Please enter your choice: "
select option in "${options[@]}"; do
    case $option in
        "Show Docker Images")
            show_docker_images
            ;;
        "Remove Docker Image")
            remove_docker_image
            ;;
        "Show Docker Containers")
            show_docker_containers
            ;;
        "Remove Docker Container")
            remove_docker_container
            ;;
        "Stop All Containers")
            stop_all_containers
            ;;
        "Reprint Options")
            reprint_options
            ;;
        "Quit")
            break
            ;;
        *)
            echo "Invalid option. Try again."
            ;;
    esac
done
```
