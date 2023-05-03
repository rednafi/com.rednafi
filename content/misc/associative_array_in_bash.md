---
title: Associative arrays in Bash
date: 2023-05-03
tags:
    - Shell
    - TIL
---

One of my favorite pastimes these days is to set BingChat to creative mode, ask it to
teach me a trick about topic X, and then write a short blog post about it to reinforce
my understanding. Some of the things it comes up with are absolutely delightful. In the
spirit of that, I asked it *to teach me a Shell trick that I can use to mimic maps or
dictionaries in a shell environment*. I didn't even know what I was expecting.

It didn't disappoint and introduced me to the idea of associative arrays in Bash. This
data structure is basically the Bash equivalent of a map.

First, we have our usual arrays which are containers that can store multiple values,
indexed by numbers. Associative arrays are similar, but they use strings as keys instead
of numbers. For example, if you want to store the names of some fruits in a regular
array, you can use:

```bash
fruits=(apple banana cherry)
```

This will create an array called fruits with three elements. You can access the elements
by using the index number inside brackets, such as:

```bash
echo ${fruits[0]}
```

This prints:

```txt
apple
```

You can also use a range of indices to get a slice of the array, such as:

```bash
echo ${fruits[@]:1:2}
```

This will print: 

```txt
banana cherry
```

You can also use `*` or `@` to get all the elements of
the array, such as:

```bash
echo ${fruits[*]}
```

This returns:

```txt
apple banana cherry
```

Associative arrays are declared with the `declare -A` command, and then assigned values
using the `=` operator and brackets. For example, if you want to store the prices of
some fruits in an associative array, you can use:

```bash
declare -A prices
prices[apple]=1.00
prices[banana]=0.50
prices[cherry]=2.00
```

This will create an associative array called `prices` with three key-value pairs. You
can access the values by using the keys inside brackets, such as:

```bash
echo ${prices[apple]}
```

This will print:

```txt
1.00
```

Similar to regular arrays, you can use `*` or `@` to get all the keys or values of the
associative array. Run the following command to get all the keys of the `prices`
associative array:

```bash
echo ${!prices[*]}
```

This will print:

```txt
apple banana cherry
```

To get the values, run:

```bash
echo ${prices[@]}
```

This returns:

```txt
1.00 0.50 2.00
```

Arrays and associative arrays can be useful when you want to store and manipulate
complex data structures in bash. You can use them to perform arithmetic operations,
string operations, or loop over them with for or while commands. For example, you can
use:

```bash
for fruit in ${!prices[*]}; do
    echo "$fruit costs ${prices[$fruit]}";
done
```

In the above snippet, we iterate through the keys of `prices` in a `for` loop. The
`${!prices[*]}` notation expands to a list of all the keys in the `prices` array. Inside
the loop, we print the key-value pairs, where `$fruit` represents the current key and
`${prices[$fruit]}` represents the corresponding value. So in each iteration, the
snippet will output the name of each fruit along with its corresponding price.

Running the snippet will print:

```txt
apple costs 1.00
banana costs 0.50
cherry costs 2.00
```

## A more practical example

Here's a script that downloads three famous RFCs using cURL. We're using an associative array
for bookkeeping purposes.

```bash
#!/usr/bin/env bash

set -euo pipefail

declare -A rfc_urls

base_url="https://www.rfc-editor.org/rfc"

rfc_urls["http-error"]="${base_url}/rfc7808.txt"
rfc_urls["http-one"]="${base_url}/rfc7231.txt"
rfc_urls["datetime-format"]="${base_url}/rfc3339.txt"

echo "======================"
echo "start downloading rfcs"
echo "======================"
echo ""

for key in ${!rfc_urls[*]}; do
    value=${rfc_urls[$key]}
    echo "Downloading rfcs ${key}: ${value}"
    curl -OJLs "${value}"
done

echo ""
echo "======================"
echo "done downloading rfcs"
echo "======================"
```

Running this will download the RFCs in the current directory:

```txt
======================
start downloading rfcs
======================

Downloading rfcs http-error: https://www.rfc-editor.org/rfc/rfc7808.txt
Downloading rfcs datetime-format: https://www.rfc-editor.org/rfc/rfc3339.txt
Downloading rfcs http-one: https://www.rfc-editor.org/rfc/rfc7231.txt

======================
done downloading rfcs
======================
```

The script begins by declaring an associative array called `rfc_urls`. This array serves
as a convenient way to keep track of the RFCs we want to download. Each key in the array
represents a unique identifier for an RFC, while the corresponding value holds the
complete URL to download that specific RFC.

Next, we set the base_url variable to `https://www.rfc-editor.org/rfc`, which will be
used as the base URL for all RFC downloads.

Inside a loop that iterates over the keys of the `rfc_urls` array, we retrieve the URL
value associated with each key. To provide a progress update, we echo a message
indicating the RFC being downloaded.

Using the `curl` command with the options `-OJLs`, we initiate the download process.
The `-O` flag ensures that the remote file is saved with its original filename, while
the `-J` flag takes advantage of the `Content-Disposition` header in the HTTP response
to determine the filename. We include the `-L` flag to follow redirects, and the `-s`
flag to silence curl's progress output.

## Resources

* [Advanced Bash scripting guide – devconnected]
* [Advanced Bash scripting techniques for Linux administrators]
* [Useful Bash command line tips and tricks examples – part 1 - Linux config]
* [3 command line games for learning Bash the fun way]

[Advanced Bash scripting guide – devconnected]: https://devconnected.com/advanced-bash-scripting-guide/
[Advanced Bash scripting techniques for Linux administrators]: https://tecadmin.net/advanced-bash-scripting-techniques/
[Useful Bash command line tips and tricks examples – part 1 - Linux config]: https://linuxconfig.org/useful-bash-command-line-tips-and-tricks-examples-part-1
[3 command line games for learning Bash the fun way]: https://opensource.com/article/19/10/learn-bash-command-line-games
