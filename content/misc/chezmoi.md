---
atprotoPath: /misc/chezmoi/
date: 2026-06-12T00:00:00+02:00
description: 'Why I swapped GNU stow''s symlink farm for chezmoi: one command to bootstrap a Mac with Homebrew packages and macOS settings, a small daily sync loop, and agent skills shared between Claude Code and Codex.'
slug: chezmoi
tags:
    - Shell
    - CLI
    - Unix
title: Migrating from GNU stow to chezmoi
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mo2dhnsg7s2k"
---

I've been managing my dotfiles with [GNU stow] for a few years. I even wrote [a piece with a
corny title] about that setup back in 2023. Stow served me well, but managing symlinks
across multiple devices slowly became a pain in the butt.

So I started looking around for a better tool and even considered writing my own. Then a
colleague pointed me to [chezmoi], and so far I'm liking it a lot. It does everything I
need, and I've started tracking my agent skill files with it too.

## The machines

I run three Macs: a MacBook Pro for work, a MacBook Air for personal use, and a Mac Mini
that acts as a small personal server. The Mini mostly gets SSHed into from the other two.
It's still a Mac with my shell on it, so the same dotfiles apply.

I also keep a few Linux VMs around, but I rarely need my dotfiles on servers. Ansible
provisions those. This workflow is strictly for the desktop machines.

## When I outgrew stow

Stow's model is symlinking. The config files live in a git repo, grouped into directories
that stow calls packages, and stowing a package links its files into the home directory. For
a single machine it still holds up. The commands are idempotent and there's almost nothing
to learn.

The trouble is that symlinks cut both ways. Every edit on every machine writes straight
through the link into that machine's clone of the repo. Months later I'd find dirty working
trees on the Air with changes I had no memory of making. Half of them conflicted with
whatever the Pro had already pushed. Keeping three clones converged turned into a chore.

Fresh machines were the other half of the problem. Stow won't link over a real file. By the
time Homebrew and a couple of tools have run on a new Mac, files like `~/.zprofile` and
`~/.gitconfig` already exist. Bootstrapping meant cloning the repo, deleting the conflicting
files by hand, and restowing every package while trying to remember what I'd named them. And
stow only does files. Homebrew packages and macOS settings lived in separate scripts that I
had to remember to run in the right order.

## How chezmoi works

Chezmoi keeps a source directory at `~/.local/share/chezmoi`, which is a regular git repo.
`chezmoi add ~/.zshrc` copies the live file into it and names the copy `dot_zshrc`. Adding
`~/.config/gh/config.yml` creates `dot_config/gh/config.yml`, parent directories included. I
never create those names by hand since `chezmoi add` derives them from the real paths. The
tree ends up mirroring the home directory, with every leading dot spelled out as a `dot_`
prefix.

`dot_` is one of several [attributes] that chezmoi encodes into file names. A `private_`
prefix strips group and world permissions from the file. A `.tmpl` suffix turns the file
into a Go template that can read per-machine data. I use templates sparingly, and every one
of them shows up later in this post.

`chezmoi apply` goes the other way. It writes every tracked file back to the home path its
name spells out, so `dot_zshrc` lands at `~/.zshrc`. The copies are real files, not
symlinks. The source directory is the single source of truth. When a file in the home
directory stops matching its source copy, `chezmoi diff` shows the difference and the next
apply puts it back.

Losing the automatic write-through of symlinks turned out to be the thing I like most.
Nothing changes in the repo unless I deliberately put the change there.

## What I track

All of it sits in that source directory. `chezmoi cd` drops me into a subshell there, and
here's the entire tree:

```txt
~/.local/share/chezmoi
├── .chezmoi.toml.tmpl
├── .chezmoiignore
├── .chezmoiscripts
│   └── macos
│       ├── run_onchange_after_disable-macos-animations.sh
│       ├── run_onchange_after_init-macos-machine.sh.tmpl
│       └── run_onchange_before_install-homebrew-bundle.sh.tmpl
├── .gitignore
├── Brewfile
├── README.md
├── dot_agents
│   └── skills
│       ├── go-modernize
│       ├── go-styleguide
│       └── meatspeak
├── dot_claude
│   ├── settings.json
│   └── symlink_skills.tmpl
├── dot_codex
│   └── private_config.toml
├── dot_config
│   ├── gh
│   │   ├── config.yml
│   │   └── private_hosts.yml
│   └── ghostty
│       └── config
├── dot_gitconfig
├── dot_gitconfig-pers
├── dot_gitconfig-werk
├── dot_shellcheckrc
├── dot_zsh_aliases
└── dot_zshrc
```

The list is short because I dislike customizing tools and stick to defaults where I can. The
dotfiles proper are the zsh, git, shellcheck, [ghostty], and GitHub CLI configs. I track
Claude Code's `settings.json` and Codex's `config.toml` too, so the agents behave the same
on every machine. The `private_` prefix on gh's `hosts.yml` and the Codex config keeps those
two at `0600`. I'll talk about the skills under `dot_agents` at the end.

The three gitconfigs split my identities. All my projects live under two directories,
`~/canvas/werk/` for work and `~/canvas/pers/` for everything personal, and both exist on
every machine. The main gitconfig routes identity by where a repo lives:

```txt
[includeIf "gitdir:~/canvas/pers/"]
    path = ~/.gitconfig-pers

[includeIf "gitdir:~/canvas/werk/"]
    path = ~/.gitconfig-werk
```

Repos under `~/canvas/pers/` get my personal email and repos under `~/canvas/werk/` get the
work one. That's a plain git feature, not chezmoi templating, but chezmoi guarantees all
three files exist on every machine.

That `.chezmoi.toml.tmpl` at the top is chezmoi's own config template. It asks for the
machine's name once and remembers the answer in `~/.config/chezmoi/chezmoi.toml`:

```txt
{{- $machineName := promptStringOnce . "machineName" "machineName" .chezmoi.hostname -}}

[data]
    machineName = {{ $machineName | quote }}
```

The machine setup script reads that value to set the hostname. It's the only per-machine
data in the whole repo. Everything else is identical everywhere. I keep it this way partly
for simplicity and partly because I'm not a big fan of Go's template syntax, so the less I
have to muck around with it, the better.

`.chezmoiignore` lists `README.md`, the `Brewfile`, and `Brewfile.lock.json`, so all three
stay in the source directory without ever being written to the home directory. A plain
`.gitignore` keeps the lock file out of version control. I'll cover the `Brewfile` and the
scripts under `.chezmoiscripts` in the next section.

## Bootstrapping a new Mac

Homebrew goes on first, and then the whole setup is two commands:

```sh
brew install chezmoi
chezmoi init --apply \
    --promptString machineName=mini \
    https://github.com/rednafi/dotfiles.git
```

`chezmoi init` clones the repo into `~/.local/share/chezmoi`, and `--apply` writes every
tracked file into place right away. The `--promptString` flag pre-answers the config
template's question. Without it, chezmoi asks interactively. Scripts run as part of the same
apply.

Anything under `.chezmoiscripts/` gets [executed during apply], and the file names control
the timing:

- A `before` script runs before chezmoi writes any files.
- An `after` script runs once they're all in place.
- The `run_onchange_` prefix makes a script fire on the first apply and after that only when
  its contents change.

On a fresh machine that works out to: install the Homebrew packages, lay down the dotfiles,
then configure macOS itself. The `onchange` part enables a trick I took [straight from the
chezmoi docs]. Here's the Homebrew script, trimmed:

```sh
#!/usr/bin/env bash
# Brewfile checksum: {{ include "Brewfile" | sha256sum }}

# ... elided

brewfile={{ joinPath .chezmoi.sourceDir "Brewfile" | quote }}

"$brew_bin" bundle check --no-upgrade --file "$brewfile" >/dev/null 2>&1 \
    || "$brew_bin" bundle install --no-upgrade --file "$brewfile"
```

The elided lines locate the Homebrew binary and store its path in `$brew_bin`. The template
inlines a hash of the `Brewfile` into a comment. Adding a package to the `Brewfile` changes
the hash, which changes the rendered script, which makes chezmoi run it again on the next
apply. So [brew bundle] fires exactly when the package list changes and stays quiet
otherwise. The `--no-upgrade` flag keeps it from touching packages that are already
installed. Upgrades stay manual since I want to see what's about to change first.

The `Brewfile` is about sixty lines long. An excerpt:

```rb
brew "chezmoi"
brew "fzf"
brew "gh"
brew "micro"
brew "ripgrep"
brew "uv"

cask "claude-code"
cask "codex"
cask "ghostty"
cask "raycast"
```

Two more scripts run after the files land. The first sets the hostname from `machineName`
and writes every macOS default I'd otherwise set by clicking through System Settings on each
new machine. The second turns off most of the UI animations. Both are long lists of plain
`defaults write` calls, and the details are in the repo.

Every script starts with a Darwin check and exits early anywhere else, so nothing fires if I
ever apply this on a Linux box. I used to keep all of this in a setup script that I'd forget
to run. Now it's part of apply and I can't forget.

## Day to day

The whole routine is about five commands.

I usually edit at the source. `chezmoi edit` opens the source copy behind a home file, and
`--apply` writes it through when I close the editor:

```sh
chezmoi edit --apply ~/.zshrc
```

Sometimes the edit happens in the other direction. An installer appends to `~/.zshrc`, or I
tweak the live file directly out of habit. Now the home directory is ahead of the source,
and `chezmoi diff` will show that an apply would undo my change. When the change should
stick, I import the live file back into the source:

```sh
chezmoi add ~/.zshrc
```

When several home files have moved ahead of their sources like this, `chezmoi re-add`
re-imports them all in one go.

Once the source state looks right, I share it with plain git from inside the source repo:

```sh
chezmoi cd
git add -A
git commit -m "Update dotfiles"
git push
exit
```

On the other machines, I catch up with one command:

```sh
chezmoi update --verbose
```

That pulls the repo and applies it in one shot. When I want to inspect what's coming, I
split it up and read the diff first:

```sh
chezmoi git pull -- --autostash --rebase
chezmoi diff
chezmoi apply --verbose
```

Packages can fall out of sync with the `Brewfile` too. `brew bundle check` reports anything
the `Brewfile` expects but the machine lacks, `brew outdated --greedy` shows what's stale,
and `brew bundle cleanup` lists what's installed but untracked:

```sh
brew bundle check --no-upgrade --file "$(chezmoi source-path)/Brewfile"
brew outdated --greedy
brew bundle cleanup --file "$(chezmoi source-path)/Brewfile"
```

## Tracking agent skills

The newest things I've added to the repo are [skills for LLM agents]. A skill is a folder
with a `SKILL.md` and whatever reference files it needs. The `SKILL.md` carries name and
description frontmatter followed by instructions. The layout comes straight from the [Agent
Skills] spec, an open standard that started at Anthropic and has been adopted by a growing
list of agent products.

Because the format is standard, one copy should work everywhere. I use both [Claude Code]
and [Codex], and the skills live in `~/.agents/skills`, which Codex picks up by default. In
chezmoi terms that's a regular directory at `dot_agents/skills/`, tracked like any other
config.

Claude Code hasn't caught up with that convention yet. It looks for personal skills in
`~/.claude/skills` and knows nothing about `~/.agents`. The fix is a one-line file in the
source repo at `dot_claude/symlink_skills.tmpl`:

```txt
{{ .chezmoi.homeDir }}/.agents/skills
```

Three name parts work together here:

- The `dot_claude/` directory and the file name map the target to `~/.claude/skills`, the
  same way `dot_zshrc` maps to `~/.zshrc`.
- The `symlink_` prefix tells chezmoi to create that target as a symlink instead of a
  regular file, pointing wherever the file's content says.
- The `.tmpl` suffix makes chezmoi render the content first, so `{{ .chezmoi.homeDir }}`
  expands to the right home directory on whichever machine is applying.

After an apply:

```sh
ls -ld ~/.claude/skills
```

```txt
lrwxr-xr-x 1 rednafi staff 29 Jun 11 17:37 /Users/rednafi/.claude/skills -> /Users/rednafi/.agents/skills
```

There's a mild irony in leaving stow to escape symlinks and then having chezmoi manage the
one symlink I still need. But that's on Anthropic being a baby and not following the
convention other agents already follow. Now both agents read the same skill files, git holds
a single copy, and a new machine picks all of it up from the same `chezmoi init` as
everything else.

Everything here lives in my [dotfiles repo]. Steal whatever looks useful.

<!-- references -->
<!-- prettier-ignore-start -->

[gnu stow]:
    https://www.gnu.org/software/stow/

[a piece with a corny title]:
    /misc/dotfile-stewardship-for-the-indolent/

[chezmoi]:
    https://www.chezmoi.io/

[attributes]:
    https://www.chezmoi.io/reference/source-state-attributes/

[ghostty]:
    https://ghostty.org/

[executed during apply]:
    https://www.chezmoi.io/user-guide/use-scripts-to-perform-actions/#understand-how-scripts-work

[straight from the chezmoi docs]:
    https://www.chezmoi.io/user-guide/use-scripts-to-perform-actions/#run-a-script-when-the-contents-of-another-file-changes

[brew bundle]:
    https://docs.brew.sh/Brew-Bundle-and-Brewfile

[skills for llm agents]:
    https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills

[agent skills]:
    https://agentskills.io/home

[claude code]:
    https://code.claude.com/docs/en/skills

[codex]:
    https://developers.openai.com/codex/

[dotfiles repo]:
    https://github.com/rednafi/dotfiles

<!-- prettier-ignore-end -->
