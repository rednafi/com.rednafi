---
title: Use strict mode while running bash scripts
date: 2021-11-08
slug: use-strict-mode-while-running-bash-scripts
aliases:
    - /misc/use_strict_mode_while_running_bash_scripts/
tags:
    - Shell
    - TIL
---

Use unofficial bash strict mode while writing scripts. Bash has a few gotchas and this helps
you to avoid that. For example:

```bash
#!/bin/bash

set -euo pipefail

echo "Hello"
```

Where,

```txt
-e              Exit immediately if a command exits with a non-zero status.
-u              Treat unset variables as an error when substituting.
-o pipefail     The return value of a pipeline is the status of
                the last command to exit with a non-zero status,
                or zero if no command exited with a non-zero status.
```

## References

The idea is a less radical version of this post[^1]. I don't recommend messing with the IFS
without a valid reason.

[^1]:
    [Unofficial bash strict mode](http://redsymbol.net/articles/unofficial-bash-strict-mode/)
