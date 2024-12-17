---
Title: SSH saga
Date: 2024-12-17
Tags:
    - Shell
---

Setting up SSH access to a new VM usually follows the same routine: generate a key pair,
copy it to the VM, tweak some configs, confirm the host's identity, and maybe set up an
agent to avoid typing passphrases all day. Tools like cloud-init and Ansible handle most of
this for me now, so I rarely think about it. But I realized I don't fully understand how all
the parts work together.

Explaining the interplay between the SSH client, `sshd` on the server, `ssh-agent` on the
client, and config files scattered across both machines turned out to be a bit more involved
than I expected.

This post attempts to give an overview of what happens when you type `ssh user@host`,
covering key pairs, `authorized_keys`, `sshd_config`, `~/.ssh/config`, `known_hosts`, and
how they connect.

## A new VM in the void

You've provisioned a new VM and need key-based SSH access. This involves generating a key
pair on your local machine, installing the public key on the remote VM, and ensuring the SSH
daemon (`sshd`) on the VM trusts it. Done right, `ssh user@host` drops you into a shell
without a password prompt.

First, generate a key pair on your _local machine_:

```bash
ssh-keygen -t ed25519 -C "your_email@example.com"
```

This creates two files: a private key (`~/.ssh/id_ed25519`) and a public key
(`~/.ssh/id_ed25519.pub`). The private key stays local. The public key is shared.

## Your local fortress

Your local private key proves your identity and must be protected (`~/.ssh/id_ed25519`). The
public key (`~/.ssh/id_ed25519.pub`) gets copied to the VM.

To view the public key locally:

```bash
cat ~/.ssh/id_ed25519.pub
# Output: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG... user@local
```

Copy this public key to the VM.

## The remote gatekeeper

On the VM, `sshd` listens for connections and authenticates users. Its configuration file,
`/etc/ssh/sshd_config`, defines policies: whether password logins are allowed, which keys
are trusted, and which crypto settings to use. A hardened snippet might look like this:

```sh
# /etc/ssh/sshd_config (on the VM)
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile     .ssh/authorized_keys
```

With `PasswordAuthentication no`, only keys can unlock access.

## Authorized keys and the handshake

The `~/.ssh/authorized_keys` file on the VM decides who gets access. Add your public key
there to tell `sshd` that anyone holding the matching private key (you) can connect.

On the VM, under the user's home directory:

```sh
mkdir -p ~/.ssh
chmod 700 ~/.ssh
echo "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG... user@local" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

Now when you run `ssh user@host`, the server matches your key to one in `authorized_keys`
and lets you in.

## The client and its configs

Your local SSH client can be configured via `~/.ssh/config` to simplify hostnames, ports,
and key paths:

```bash
# ~/.ssh/config (on your local machine)
Host myvm
    HostName 203.0.113.10
    User ubuntu
    IdentityFile ~/.ssh/id_ed25519
    Port 22
```

Now you can connect with:

```bash
ssh myvm
```

## Known hosts and server identity

When you connect to the VM for the first time, SSH prompts you to confirm its identity.
Accepting it adds the server's host key to `~/.ssh/known_hosts`. SSH will detect any
identity changes during subsequent connections:

```sh
203.0.113.10 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...
```

This prevents man-in-the-middle attacks.

## Agent and forwarding

If your private key has a passphrase, typing it constantly is annoying. `ssh-agent` caches
the key in memory, so you only unlock it once:

```sh
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519
# Enter passphrase once
```

For hopping through multiple servers (`local → bastion → internal server`), enable agent
forwarding:

```bash
# ~/.ssh/config (local)
Host myvm
    HostName 203.0.113.10
    User ubuntu
    IdentityFile ~/.ssh/id_ed25519
    ForwardAgent yes
```

Now, you can connect through intermediary hosts without copying private keys around.

## Configuring and hardening the daemon

On the VM, refine `/etc/ssh/sshd_config` to enforce stricter security:

```sh
# /etc/ssh/sshd_config (on the VM)
PasswordAuthentication no
PermitRootLogin prohibit-password
AllowUsers ubuntu
PubkeyAuthentication yes
KexAlgorithms curve25519-sha256@libssh.org
Ciphers aes256-gcm@openssh.com,chacha20-poly1305@openssh.com
MACs hmac-sha2-512-etm@openssh.com
```

These settings ensure that only trusted keys with modern crypto algorithms can connect.

## Bringing it all together

Here's a quick summary of the steps:

1. Generate a key pair locally.
2. Add the public key to `authorized_keys` on the VM.
3. Configure `sshd_config` for key-based authentication.
4. Use `~/.ssh/config` to simplify connections.
5. Verify the server's identity via `known_hosts`.
6. Use `ssh-agent` for convenience.

```txt
          ┌──────────────────────┐
          │      LOCAL           │
          │  ~/.ssh/config       │
          │  ~/.ssh/id_*         │
          │  ~/.ssh/known_hosts  │
          │  ssh-agent           │
          └───▲───────┬──────────┘
              │       │
              │       │ SSH Connection (Port 22)
              │       │
              │       │
              │       ▼
          ┌────────────────────────┐
          │     REMOTE             │
          │ /etc/ssh/sshd_config   │
          │ ~/.ssh/authorized_keys │
          │ sshd daemon            │
          └────────────────────────┘
```
