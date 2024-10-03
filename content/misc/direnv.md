---
title: Discovering direnv
Date: 2024-10-02
Tags:
    -   TIL
    -   Shell
---

I'm not really a fan of shims—code that automatically performs actions as a side effect or
intercepts commands when you use the shell or when a prompt runs. That's why, other than the
occasional dabbling, I've mostly stayed away from tools like `asdf` or `pyenv` and instead
stick to `apt` or `brew` for managing my binary installs, depending on the OS.

Recently, though, I've started seeing many people I admire extolling `direnv`:

> _If you're old-school like me, my `.envrc` looks like this:_
>
> ```sh
> uv sync --frozen
> source .venv/bin/activate
> ```
>
> _The sync ensures there's always a `.venv`, so no memory-baking required._
>
> _— Hynek Schlawack[^1]_

Or,

> _This is embarrassing, but after using direnv for 10+ years, I only discovered the
> `source_env` directive yesterday._
>
> _Game changer. I used it to improve our project's dev configuration ergonomics so new
> environment variables are easily distributed via Git._
>
> _—Brandur[^2]_

So I got curious and wanted to try the tool to see if it fits into my workflow, or if I'll
quickly abandon it when something goes wrong.

When I first visited their landing page[^3], I was a bit confused by the tagline:

> _direnv – unclutter your .profile_

But I don't have anything custom in my `.profile`, or more specifically, my `.zprofile`.
Here's what's in it currently:

```sh
cat ~/.zprofile
```

```txt
eval "$(/opt/homebrew/bin/brew shellenv)"

# Added by OrbStack: command-line tools and integration
source ~/.orbstack/shell/init.zsh 2>/dev/null || :
```

Then I realized that `.profile` is used here as a general term for various configuration
files like `.*profile`, `.*rc`, and `.*env`. I have quite a bit set up in both my `~/.zshrc`
and `~/.zshenv`—a mix of global and project-specific commands and environment variables.

To explain: `.*profile` files (like `.profile` or `.bash_profile`) are used by login shells,
which are started when you log into a system, such as through SSH or a terminal login. In
contrast, files like `.bashrc` or `.zshrc` are for interactive shells, meaning they run when
you open a new terminal window or tab. For Zsh, `.zshenv` is sourced by all types of
shells—both login and interactive—making it useful for global environment settings.

## What problem it solves

Direnv solves the hassle of managing environment variables across different projects by
automatically loading them when you enter a directory and unloading them when you leave. It
keeps your global environment clean and avoids cluttering up your shell configuration files.

It checks for an `.envrc` (or `.env`) file in the current or parent directories before each
prompt. If found and authorized, it loads the file into a bash sub-shell and applies the
environment variables to the current shell.

It supports hooks for common shells like Bash, Zsh, Tcsh, and Fish, allowing you to manage
project-specific environment variables without cluttering your `~/.profile`. Since it's a
fast, single static executable, direnv runs seamlessly and is language-agnostic, meaning you
can easily use it alongside tools like `rbenv`, `pyenv`, and `phpenv`.

You might argue that `source .env` works just fine, but it's an extra step to remember.
Also, being able to communicate the project-specific environment commands and variables, and
having them sourced automatically, is a nice bonus.

## Why .envrc file and not just a plain .env file

This was the first question that came to my mind: why not just use a `.env` file? Why
introduce another configuration file? Grokking the docs clarified things.

The `.envrc` file is treated like a shell script, where you can also list arbitrary shell
commands that you want to be executed when you enter a project directory. You can't do that
with a plain `.env` file. However, direnv does support `.env` files too.

It's such a simple idea that opens up many possibilities.

## How I use it

Here are a few things I'm using it for:

-   Automatically loading environment variables from a `.env` file.
-   Loading different sets of values for the same environment keys, e.g., local vs. staging
    values.
-   Activating the virtual environment when I enter the directory of a Python project.

Let's say you want to load your environment variables automatically when you `cd` into a
directory and have them removed from the shell environment when you leave it. Suppose the
project directory looks like this:

```txt
svc/
├── .env
├── .env.staging
└── .envrc
```

The `.env` file contains environment variables for local development:

```txt
FOO="foo"
BAR="bar"
```

And the `.env.staging` file contains the variables for staging:

```txt
FOO="foo-staging"
BAR="bar-staging"
```

The `.envrc` file can have just one command to load the default `.env` file:

```sh
dotenv
```

Now, from the `svc` directory, you'll need to allow direnv to load the environment variables
into the current shell:

```sh
direnv allow
```

This prints:

```txt
direnv: loading ~/canvas/rednafi.com/svc/.envrc
direnv: export +BAR +FOO
```

You can now print the values of the environment variables like this:

```sh
echo "${FOO-default}"; echo "${BAR-default}"
```

This returns:

```txt
foo
bar
```

If you want to load different variables depending on the environment, you can add the
following shell script to the `.envrc` file:

```sh
case "${ENVIRONMENT}" in
  "staging")
    if [[ -f ".env.staging" ]]; then
      dotenv .env.staging
    fi
    ;;
  *)
    if [[ -f ".env" ]]; then
      dotenv
    fi
    ;;
esac
```

The script loads the `.env.staging` file if the value of `$ENVIRONMENT` is `staging`;
otherwise, it loads the default `.env` file. From the `svc` root, run:

```sh
direnv allow
```

This will still load the variables from `.env`. To load variables from `.env.staging`, run:

```sh
export ENVIRONMENT=staging && direnv allow
```

This time, printing the variables returns the staging values:

```txt
foo-staging
bar-staging
```

Oh, and when you leave the directory, the environment variables will be automatically
unloaded from your working shell.

```sh
cd ..
```

```txt
direnv: unloading
```

You can do a lot more with the idea, but going overboard with environment variables can be
risky. You don't want to accidentally load something into the environment you didn't intend
to. Keeping it simple with sane defaults is the way to go.

Like Hynek, I've adopted `uv`[^4] in my Python workflow, and now my default `.envrc` has
these two commands:

```sh
uv sync --frozen
source .venv/bin/activate
```

The first command updates the project's environment without changing the `uv.lock` file, and
the second ensures I never need to remember to activate the virtual environment before
running commands. Now, when I `cd` into a Python project and run:

```sh
echo $VIRTUAL_ENV
```

It shows that the local `.venv` is active:

```txt
/Users/rednafi/canvas/rednafi.com/.venv
```

No more worrying about mucking up my global Python installation while running some commands.

Another neat directive is `source_up`, which lets you inherit environment variables from the
parent directory. Normally, when you move into a child directory, direnv unloads the parent
directory's environment variables. But with the `source_up` directive in your `.envrc`,
it'll keep those variables around in the child directory.

Then there's the `source_env` directive, which lets you pull one `.envrc` file into another.
So, if you’ve got some common, non-secret variables in an `.envrc.local` file, you can
easily reuse them in your `.envrc`.

Here's an example `.envrc.local` file:

```sh
export API_URL="http://localhost:5222"
export DATABASE_URL="postgres://localhost:5432/project-db"
```

You can import `.env.local` into the `.envrc` file like this:

```sh
source_env .envrc.local

# Other commands and variables go here
```

I haven't used `source_env` much yet, but I love the possibilities it unlocks.

The biggest reason I've adopted it everywhere is that it lets me share my shell environment
variables and the magic commands without having anything stashed away in my `~/.zshrc` or
`~/.zshenv`, so there's no need for out-of-band communication.

[^1]: [Hynek on Twitter](https://x.com/hynek/status/1838076629249044533)

[^2]: [Brandur on Twitter](https://x.com/brandur/status/1837104038854164645)

[^3]: [direnv](https://direnv.net/)

[^4]: [uv](https://github.com/astral-sh/uv)
