---
title: Explicit method overriding with @typing.override
date: 2024-11-05
draft: true
tags:
    - Python
    - TIL
---

Despite using Python 3.12 on production for almost a year, one neat feature in the typing
module that escaped me was the `@override` decorator. It was proposed in PEP-698[^1] and had
been living in the `typing_extensions` module for a while.

The `@override` decorator

[^1]: [PEP 698 – Override decorator for static typing](https://peps.python.org/pep-0698/)
