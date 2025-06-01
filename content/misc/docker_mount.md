---
Title: Docker mount revisited
Date: 2024-10-22
Tags:
    -   TIL
    -   Docker
---

I always get tripped up by Docker's different mount types and their syntax, whether I'm
stringing together some CLI commands or writing a `docker-compose` file. Docker's docs cover
these, but for me, the confusion often comes from how "bind" is used in various contexts and
how "volume" and "bind" sometimes get mixed up in the documentation.

Here's my attempt to disentangle some of my most-used mount commands.

## Volume mounts

Volume mounts[^1] let you store data outside the container in a location managed by Docker.
The data persists even after the container stops. On non-Linux systems, volume mounts are
faster than bind mounts because data doesn't need to cross the virtualization boundary.

### The `-v` option

The `-v` flag is the older and more common way to define volume mounts in the Docker CLI.
For example:

```sh
docker run -v myvolume:/usr/share/nginx/html:ro nginx
```

Here's what each part means:

- `myvolume`: The name of the Docker-managed volume on the host.
- `/usr/share/nginx/html`: The mount point inside the container.
- `:ro`: Mounts the volume as read-only inside the container. The host can still write to
  the volume, but the container cannot.

The general syntax is:

```sh
-v [SOURCE]:[TARGET]:[OPTIONS]
```

It can be tricky to remember which part is the host and which is the container, especially
since with volumes, the `SOURCE` is a volume name, not a host path.

### The `--mount` option

To make things clearer, Docker introduced the `--mount` option, which uses key-value pairs.
The same volume mount using `--mount` looks like this:

```sh
docker run \
    --mount \
    type=volume,source=myvolume,target=/usr/share/nginx/html,readonly nginx
```

Or using shorthands:

```sh
docker run --mount type=volume,src=myvolume,dst=/usr/share/nginx/html,ro nginx
```

I find this syntax more explicit and less error-prone, even if it's a bit more verbose.

### In `docker-compose.yml`

In `docker-compose`, volumes can be defined using both the old and new syntax. Here's how
they compare:

**Old style**

```yaml
services:
  app:
    image: nginx
    volumes:
      - myvolume:/usr/share/nginx/html:ro
volumes:
  myvolume:
```

**New style**

```yaml
services:
  app:
    image: nginx
    volumes:
      - type: volume
        source: myvolume
        target: /usr/share/nginx/html
        read_only: true
volumes:
  myvolume:
```

I prefer the new style because it reduces ambiguity and makes the configuration clearer.

## Bind mounts

Bind mounts[^2] let you directly mount a file or directory from the host into the container.
This is especially useful in development when you want the container to have access to your
code or data.

The key difference between volume mounts and bind mounts is that volumes are fully managed
by Docker and stored in a special location, while bind mounts rely on specific host paths.
Volumes are more portable and isolated from the host, whereas bind mounts give you direct
access to host files but can introduce permission issues and depend on the exact file
structure of the host.

### The `-v` option

Using the `-v` syntax for bind mounts:

```sh
docker run -v /path/on/host:/usr/share/nginx/html:ro nginx
```

Here:

- `/path/on/host`: The path on the host machine. This must be an absolute path.
- `/usr/share/nginx/html`: The mount point inside the container.
- `:ro`: Mounts the directory as read-only inside the container.

### The `--mount` option

Using `--mount` for a bind mount:

```sh
docker run \
    --mount \
    type=bind,source=/path/on/host,target=/usr/share/nginx/html,readonly nginx
```

This syntax makes it clear that you're using a bind mount and specifies exactly which paths
are involved.

### In `docker-compose.yml`

In `docker-compose`, bind mounts can be specified like this:

**Old style**

```yaml
services:
  app:
    image: nginx
    volumes:
      - ./path/on/host:/usr/share/nginx/html:ro
```

**New style**

```yaml
services:
  app:
    image: nginx
    volumes:
      - type: bind
        source: ./path/on/host
        target: /usr/share/nginx/html
        read_only: true
```

> **Note:** In `docker-compose`, if you specify a `source` that doesn't start with `/` (an
> absolute path) or `./` (a relative path), Docker might think you're referring to a volume.
> To ensure it's interpreted as a bind mount, start the path with `./` or `/`.

## Tmpfs mounts

Tmpfs mounts[^3] store data in the host's memory, not on disk. This makes them ideal for
temporary storage that doesn't need to persist after the container stops. They're great for
things like caches or scratch space.

### The `--tmpfs` option

Docker provides a `--tmpfs` option to create a tmpfs mount more concisely:

```sh
docker run --tmpfs /app/tmp:rw,size=64m nginx
```

- `/app/tmp`: The target directory inside the container.
- `rw`: This option allows read and write access to the tmpfs mount.
- `size=64m`: Sets the size of the tmpfs mount to 64 MB.

### The `--mount` option

Alternatively, using the more flexible `--mount` option:

```sh
docker run \
    --mount type=tmpfs,target=/app/tmp,tmpfs-size=64m,tmpfs-mode=1777 nginx
```

Here's what each part means:

- `type=tmpfs`: Specifies that this is a tmpfs mount, using the host's memory.
- `target=/app/tmp`: The directory inside the container where the tmpfs mount is mounted.
- `tmpfs-size=64m`: Limits the size of the tmpfs mount to 64 MB.
- `tmpfs-mode=1777`: Sets permissions for the tmpfs mount (1777 grants read, write, and
  execute permissions to everyone).

### In `docker-compose.yml`

In `docker-compose`, tmpfs mounts can be defined using both the old and new syntax.

**Old style**

```yaml
services:
  app:
    image: nginx
    tmpfs:
      - /app/tmp:size=64m
```

**New style**

```yaml
services:
  app:
    image: nginx
    volumes:
      - type: tmpfs
        target: /app/tmp
        tmpfs:
          size: 64m
          mode: 1777
```

## Build cache mounts

Build cache mounts[^4] help speed up Docker image builds by caching intermediate files like
package downloads or compiled artifacts. They're used during the build process and aren't
part of the final container image.

In a `Dockerfile`, you might use a build cache like this:

```dockerfile
RUN --mount=type=cache,target=/var/cache/apt \
    apt-get update && apt-get install -y curl
```

Here's what each option does:

- `--mount=type=cache`: Defines a cache mount that stores the files from the `apt-get`
  commands to speed up future builds by reusing the downloaded packages.
- `target=/var/cache/apt`: Specifies the location inside the container where the cache will
  be stored during the build process.

In my Python projects, I cache my dependencies and install them with `uv` like this:

```dockerfile
RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=bind,source=uv.lock,target=uv.lock \
    --mount=type=bind,source=pyproject.toml,target=pyproject.toml \
    uv sync --no-install-project --locked --no-dev
```

Above, the cache mount at `/root/.cache/uv` saves dependencies so they don't need to be
re-downloaded in future builds if nothing changes. The bind mounts provide access to
`uv.lock` and `pyproject.toml` from the host, allowing the container to read these config
files during the build. Any changes to the files are picked up, while cached dependencies
are reused unless the configurations have been updated.

[^1]: [Volume mounts](https://docs.docker.com/storage/volumes/)

[^2]: [Bind mounts](https://docs.docker.com/storage/bind-mounts/)

[^3]: [Tmpfs mounts](https://docs.docker.com/storage/tmpfs/)

[^4]: [Build cache mounts](https://docs.docker.com/build/cache/optimize/#use-cache-mounts)
