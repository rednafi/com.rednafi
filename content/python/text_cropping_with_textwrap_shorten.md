---
title: Cropping texts in Python with 'textwrap.shorten'
date: 2022-01-06
tags:
    - Python
    - TIL
---

## Problem

A common interview question that I've seen goes as follows:

Write a function to crop a text corpus without breaking any word.

-   Take the length of the text up to which **character** you should trim.
-   Make sure that the cropped text doesn't have any trailing space.
-   Try to maximize the number of words you can pack in your trimmed text.

Your function should look something like this:

```python
def crop(text: str, limit: int) -> str:
    """Crops 'text' upto 'limit' characters."""

    # Crop the text.
    cropped_text = perform_crop()
    return cropped_text
```

For example, if `text` looks like this—

```txt
"A quick brown fox jumps over the lazy dog."
```

and you're asked to crop it up to `9` characters, then the function `crop` should return:

```txt
"A quick"
```

and not:

```txt
"A quick "
```

or:

```txt
"A quick b"
```

## Solution

This is quite easily solvable by using Python's `textwrap.shorten` function. The `shorten`
function takes quite a few parameters[^1]. However, we'll only need the following ones to do
our job:

-   `text: str`: Target text that we're going to operate on.
-   `width: int` : Desired width after cropping.
-   `initial_indent: str`: Character to use for the initial indentation. Provide empty
    string for no initial indentation.
-   `subsequent_indent: str`: Character to use for the subsequent indentation. Provide empty
    string for no subsequent indentation.
-   `break_long_words: bool`: Whether to break long words or not.
-   `break_on_hyphens: bool`: Whether to break words on hyphens or not.
-   `placeholder: bool`: Placeholder character. The default here is `[...]`. However,
    provide an empty string if you don't want any placeholder after the cropped string. The
    length of the placeholder is going to be included in the total length of the cropped
    text.

With the descriptions out of the way, let's write the `crop` function here:

```python
# src.py
import textwrap


def crop(text: str, limit: int) -> str:
    cropped_text = textwrap.shorten(
        text,
        width=limit,
        initial_indent="",
        subsequent_indent="",
        break_long_words=False,
        break_on_hyphens=False,
        placeholder="",
    )
    return cropped_text


if __name__ == "__main__":
    cropped_text = crop(
        text="A quick brown fox jumps over the lazy dog.",
        limit=9,
    )

    print(cropped_text)
```

This prints out the desired output as follows:

```txt
A quick
```

You can see that we achieved our goal of cropping a text corpus without breaking any word.
Try playing around with the `initial_indent`, `subsequent_indent`, and `placeholder`
parameters and see how they change the output.

## Complete solution with tests

```python
# src.py
import textwrap
import unittest


def crop(text: str, limit: int) -> str:
    cropped_text = textwrap.shorten(
        text,
        width=limit,
        initial_indent="",
        subsequent_indent="",
        break_long_words=False,
        break_on_hyphens=False,
        placeholder="",
    )
    return cropped_text


class TestCrop(unittest.TestCase):
    def setUp(self):
        self.text = (
            "This is an example of speech synthesis in English."
        )
        self.text_complex = """
        wrap(), fill() and shorten() work by creating a TextWrapper instance
        and calling a single method on it.
        """

    def test_ok(self):
        cropped_text = crop(self.text, limit=10)
        self.assertEqual(cropped_text, "This is an")

    def test_complex_ok(self):
        cropped_text = crop(self.text_complex, limit=15)
        self.assertEqual(cropped_text, "wrap(), fill()")

    def test_no_word_break(self):
        cropped_text = crop(self.text, limit=9)
        self.assertNotEqual(cropped_text, "This is a")

    def test_no_trailing_space(self):
        cropped_text = crop(self.text, limit=8)
        self.assertNotEqual(cropped_text, "This is ")


if __name__ == "__main__":
    unittest.main()
```

[^1]: [textwrap.shorten](https://docs.python.org/3/library/textwrap.html#textwrap.shorten)
