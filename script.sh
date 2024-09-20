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

# Usage Example

env="production"  # Could be passed as an argument to the script

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
