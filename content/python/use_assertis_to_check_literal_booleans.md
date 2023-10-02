---
title: Use 'assertIs' to check literal booleans in Python unittest
date: 2022-01-24
tags:
    - Python
    - TIL
---

I used to use Unittest's `self.assertTrue` / `self.assertFalse` to check both
**literal booleans** and **truthy**/**falsy** values in Unittest. Committed the same sin
while writing tests in Django.

> I feel like `assertTrue` and `assertFalse` are misnomers. They don't specifically check
> literal booleans, only truthy and falsy states respectively.

Consider this example:

```python
# src.py
import unittest


class TestFoo(unittest.TestCase):
    def setUp(self):
        self.true_literal = True
        self.false_literal = False
        self.truthy = [True]
        self.falsy = []

    def is_true(self):
        self.assertTrue(self.true_literal, True)

    def is_false(self):
        self.assertFalse(self.false_literal, True)

    def is_truthy(self):
        self.assertTrue(self.truthy, True)

    def is_falsy(self):
        self.assertFalse(self.falsy, True)


if __name__ == "__main__":
    unittest.main()
```

In the above snippet, I've used `assertTrue` and `assertFalse` to check both literal
booleans and truthy/falsy values. However, to test the literal boolean values, `assertIs`
works better and is more explicit. Here's how to do the above test properly:

```python
# src.py
import unittest


class TestFoo(unittest.TestCase):
    def setUp(self):
        self.true_literal = True
        self.false_literal = False
        self.truthy = [True]
        self.falsy = []

    def is_true(self):
        self.assertIs(self.true_literal, True)

    def is_false(self):
        self.assertIs(self.false_literal, False)

    def is_truthy(self):
        self.assertTrue(self.truthy, True)

    def is_falsy(self):
        self.assertFalse(self.falsy, True)


if __name__ == "__main__":
    unittest.main()
```

Notice how I've used `self.assertIs` in the `is_true` and `is_false` methods to explicitly
test out the literal boolean values. The `is_truthy` and `is_falsy` methods were kept
unchanged from the previous snippet.

[^1]: [Tweet by Drewrey Lupton](https://twitter.com/chieftanbonobo/status/741689567590395905) [^1]
