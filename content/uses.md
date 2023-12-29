---
title: Uses
layout: post
ShowToc: false
editPost:
    disabled: true
hideMeta: true
ShowShareButtons: false
---

## Software

#### $ uname -v | fold -w 60

```txt
Darwin Kernel Version 23.1.0: Mon Oct  9 21:28:31 PDT 2023;
root:xnu-10002.41.9~6/RELEASE_ARM64_T8112
```

#### $ for editor in code micro nano; do whereis $editor; done

```txt
code: /opt/homebrew/bin/code
micro: /opt/homebrew/bin/micro /opt/homebrew/share/man/man1/micro.1
nano: /usr/bin/nano /usr/share/man/man1/nano.1
```

#### $ echo $SHELL

```txt
/bin/zsh
```

#### $ brew leaves | xargs -n 3 | column -t

```txt
bash         curl        fzf
gh           git         git-lfs
go           htop        hugo
jq           kubectx     libevent
libgit2      libxft      libxinerama
micro        minikube    neofetch
oxipng       pkg-config  prettier
python@3.11  pyyaml      shellcheck
stow         telnet      tree
utf8proc     virtualenv  watch
z3           zsh
```

#### $ brew list --cask | xargs -n 3 | column -t

```txt
aldente             firefox   font-jetbrains-mono
google-chrome       orbstack  slack
visual-studio-code
```

## Hardware

```txt
Laptops -> Macbook Pro 16" M2 32/512
        -> Macbook Air 15" M2 16/256
```
