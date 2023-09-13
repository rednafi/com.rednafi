---
title: Wrapping errors in Go
date: 2023-11-23
tags:
    - Go
---

In Go, errors are [values]. While it eliminates some of the pitfalls of raising and catching exceptions, it also fosters the situtation where there are too many ways of handing errors. This is execerbated by the fact
