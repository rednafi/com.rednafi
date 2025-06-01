---
title: Simple terminal text formatting with tput
date: 2023-04-23
tags:
    - Shell
    - TIL
---

When writing shell scripts, I'd often resort to using hardcoded ANSI escape codes[^1] to
format text, such as:

```bash
#!/usr/bin/env bash

BOLD="\033[1m"
UNBOLD="\033[22m"
FG_RED="\033[31m"
BG_YELLOW="\033[43m"
BG_BLUE="\033[44m"
RESET="\033[0m"

# Print a message in bold red text on a yellow background.
echo -e "${BOLD}${FG_RED}${BG_YELLOW}This is a warning message${RESET}"

# Print a message in white text on a blue background.
echo -e "${BG_BLUE}This is a debug message${RESET}"
```

This shell snippet above shows how to add text formatting and color to shell script output
via ANSI escape codes. It defines a few variables that contain different escape codes for
bold, unbold, foreground, and background colors. Then, we `echo` two log messages with
different colors and formatting options.

The first message is printed in bold red text on a yellow background, while the second
message is printed in white text on a blue background. To ensure that subsequent output is
not affected by the previous formatting, the `RESET` variable is used to reset all color and
formatting options back to their defaults after each message is printed. The `-e` option is
used with echo to enable the interpretation of backslash escapes, which includes the ANSI
escape codes.

While this works fairly well, every time I have to write a fancy shell script, I have to
either look up the ANSI color codes, copy-paste from an existing script, or explain to an
LLM what I need. Then chatGPT serendipitously recommended a shell tool called `tput` that
makes this workflow quite a bit better. Underneath `tput` also uses ANSI escape codes to
control various text formatting options but it doesn't require you to hardcode these ugly
escape codes.

## Basic usage

The basic syntax of the `tput` command goes as follows:

```sh
tput <formatting_option>
```

## Formatting options

Here are some commonly used `tput` formatting options:

- `setaf <color>`: set the foreground (text) color to a specific color. For example,
  `setaf 1` sets the color to red, while `setaf 2` sets the color to green.
- `setab <color>`: set the background color to a specific color.
- `bold`: set the text to bold.
- `sgr0`: reset all formatting options to their defaults.
- `smul`: underline the text.

## Example usage

```bash
#!/usr/bin/env bash

# Print text in red on a yellow background
tput setaf 1
tput setab 3
echo "Error: some error occurred"
tput sgr0

# Print bold text
tput bold
echo "This text is bold"
tput sgr0

# Print underlined text in blue
tput setaf 4
tput smul
echo "This text is underlined and blue"
tput sgr0

# Print text with a custom RGB color
tput setaf 38 # specify an RGB color using 8-bit mode
tput setaf 5 # specify a color index in 256-color mode
echo "This text is in a custom color"

# Print text with a background color gradient
tput setaf 0
for i in {0..7}; do
    tput setab $i
    echo "Background color $i"
done
tput sgr0

# Print blinking text
tput blink
echo "This text is blinking"
tput sgr0
```

Running the script will give you the following output:

![tput color][image_1]

This also hardcodes the color and formatting codes but it's much easier than having to
remember or search for the ANSI escape codes. Currently, I'm using a 256-bit macOS terminal
and it supports fairly large sets of formatting options. You can run `man tput` to find out
other features that are supported by your terminal. The following loop will print all the
supported colors:

```sh
for i in {0..255}; do
    tput setab $i
    printf "  "
    tput sgr0
done
```

On my terminal, it prints this nice color palette:

![tput color palette][image_2]

[^1]: [Escape codes](https://gist.github.com/fnky/458719343aabd01cfb17a3a4f7296797)

[image_1]:
    https://blob.rednafi.com/static/images/terminal_text_formatting_with_tput/img_1.png
[image_2]:
    https://blob.rednafi.com/static/images/terminal_text_formatting_with_tput/img_2.png
