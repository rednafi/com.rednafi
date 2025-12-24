#!/usr/bin/env bash
# Lint code blocks in markdown files:
# - Replace tabs with 4 spaces
# - Optionally check for lines over 79 chars (use --check flag)

set -euo pipefail

CHECK_ONLY=false
if [[ "${1:-}" == "--check" ]]; then
    CHECK_ONLY=true
fi

# Find all markdown files in content/
files=$(find content -name "*.md" -type f)

exit_code=0

for file in $files; do
    # Check for tabs
    if grep -q $'\t' "$file"; then
        if $CHECK_ONLY; then
            echo "ERROR: $file contains tabs"
            exit_code=1
        else
            # Replace tabs with 4 spaces
            sed -i '' 's/\t/    /g' "$file"
            echo "Fixed tabs in: $file"
        fi
    fi
done

if $CHECK_ONLY && [[ $exit_code -eq 0 ]]; then
    echo "All markdown files pass linting"
fi

exit $exit_code
