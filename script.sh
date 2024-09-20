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
