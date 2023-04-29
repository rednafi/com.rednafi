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
