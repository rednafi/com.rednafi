#!/bin/bash

set -euo pipefail

name=$name

echo "Content-type: text/html"
echo ""
echo '<html><body>'
echo "Hello $name, greetings from bash!"
echo '</body></html>'
