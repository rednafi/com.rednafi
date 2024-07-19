---
title: Here-doc headache
date: 2024-07-19
tags:
    - Shell
---

I was working on the deployment pipeline for a service that launches an app in a dedicated
VM using GitHub Actions. In the last step of the workflow, the CI SSHs into the VM and runs
several commands using a here document[^1] in bash. The simplified version looks like this:

```sh
# SSH into the remote machine and run a bunch of commands to deploy the service
ssh $SSH_USER@$SSH_HOST <<EOF
    # Go to the work directory
    cd $WORK_DIR

    # Make a git pull
    git pull

    # Export environment variables required for the service to run
    export AUTH_TOKEN=$APP_AUTH_TOKEN

    # Start the service
    docker compose up -d --build
EOF
```

The fully working version can be found on GitHub[^2].

Here, environment variables like `SSH_USER`, `SSH_HOST`, and `APP_AUTH_TOKEN` are defined in
the surrounding local scope of the CI. The variables then get propagated to the remote
machine when we run the commands via here-doc.

However, I couldn't figure out why the Docker containers weren't able to access the value of
the `AUTH_TOKEN` variable. The other variables were getting through just fine.

It turns out, `export AUTH_TOKEN=$AUTH_TOKEN` within the here-doc block, doesn't export the
variable in the remote shell. So this doesn't do what I thought it would:

```sh
cat <<EOF
    export FOO=bar
    echo $FOO
EOF
```

I was expecting it to print:

```txt
export FOO=bar
echo bar
```

But instead, it just prints:

```txt
export FOO=bar
echo
```

So `export FOO=bar` in the here-doc block doesn't set the variable in the remote shell. One
solution is to set it before the block like this:

```sh
export FOO=bar
cat <<EOF
    echo $FOO
EOF
```

This prints:

```txt
echo bar
```

So, in the CI pipeline, we could do the following to propagate the environment variable from
local to the remote machine:

```sh
export FOO=bar
ssh $SSH_USER@$SSH_HOST <<EOF
    echo $FOO
EOF
```

This will print the value of the environment variable on the remote machine correctly.
However, this doesn't set the value in the remote shell's environment. If you SSH into the
remote machine and try to print the variable's value, you'll see nothing gets printed. The
previous command only passes the value to the remote machine temporarily and doesn't set it
permanently in the remote shell.

To fix it, you could pipe the value into a file and load it in the remote shell like this:

```sh
ssh $SSH_USER@$SSH_HOST <<EOF
    echo "export FOO=$FOO" > /tmp/.env
    source /tmp/.env
    echo \$FOO
EOF
```

Here, `echo \$FOO` instead of `echo $FOO` ensures that the shell expansion is done on the
remote machine, not on the local. This allows us to know that the environment variable has
been set in the remote shell correctly.

Maybe the behavior makes sense, but it still broke my mental model.

So I decided to get rid of here-doc in the pipeline altogether and went with this:

```sh
SCRIPT="
    # Go to the work directory
    cd $WORK_DIR

    # Make a git pull
    git pull

    # Export environment variables required for the service to run
    export AUTH_TOKEN=$APP_AUTH_TOKEN

    # Start the service
    docker compose up -d --build
    "

# Run the script on the remote machine
ssh $SSH_USER@$SSH_HOST "$SCRIPT"
```

It works[^3]!

One thing to keep in mind with the second approach is that if you need to run any expanded
commands, you'll need to defer it with a backslash so that it's run on the remote machine,
not on the local:

```sh
SCRIPT="
    # ...

    # Here, without the backslash, shell will try to run it on the local machine
    docker rmi -f \$(docker compose images -q) || true
    "
# Run the script on the remote machine
ssh $SSH_USER@$SSH_HOST "$SCRIPT"
```

Without the backslash, the `$(...)` will be expanded on the local machine, which is not
desirable here. The backslash defers it so that it runs on the remote instead.

[^1]: [Here documents](https://tldp.org/LDP/abs/html/here-docs.html)

[^2]:
    [Service deployment steps - with here-doc](https://github.com/rednafi/serve-init/blob/7232c55c9aa3a6c34c5da6aeb9d14afc88d9aa0e/.github/workflows/ci.yml#L86-L115)

[^3]:
    [Service deployment steps - without here-doc](https://github.com/rednafi/serve-init/blob/54b9b0fc94030eb4b9749fd4a5823a8867545f6a/.github/workflows/ci.yml#L86-L113)
