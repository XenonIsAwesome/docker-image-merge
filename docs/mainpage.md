# docker-image-merge {#mainpage}

Merge the filesystems of two Docker images into a new image with interactive conflict resolution.

## Overview

`docker-image-merge` extracts the flattened filesystem from two Docker images, compares every file and directory, classifies the differences (content changes, permission changes, type changes, new/deleted files), and lets you resolve each conflict interactively or automatically. The result is imported as a brand-new Docker image.

## Features

- **Interactive TUI** — side-by-side diff view with keyboard-driven resolution
- **Automatic strategies** — `--strategy auto-a` / `auto-b` / `fail`
- **Conflict detection** — content changes, permission changes, type conflicts, symlinks
- **Binary file support** — hex+ASCII dump for binary diffs
- **Squashed or layered output** — `--squash` for a single layer, or layered to preserve history
- **Metadata inheritance** — choose which image's ENV, CMD, etc. to keep
- **Docker CLI plugin** — installs into `~/.docker/cli-plugins/` for `docker imagemerge` usage

## Quick Start

### Install

```sh
# Linux / macOS
curl -fsSL https://xenonisawesome.github.io/docker-image-merge/install.sh | sh

# Windows (PowerShell)
iwr -useb https://xenonisawesome.github.io/docker-image-merge/install.ps1 | iex
```

### Merge two images

```sh
# Interactive mode (default)
docker imagemerge alpine:3.19 busybox:1.36 my-merged-image

# Take image A's version for all conflicts
docker imagemerge alpine:3.19 busybox:1.36 my-merged-image --strategy auto-a

# Fail if any conflicts exist
docker imagemerge alpine:3.19 busybox:1.36 my-merged-image --strategy fail
```

## How It Works

1. **Extract** — Each image's flattened filesystem is exported via `docker export`
2. **Diff** — A recursive walk compares every path using xxhash content hashing
3. **Resolve** — Conflicts are classified and presented for resolution (TUI, auto, or fail)
4. **Import** — The merged tree is imported as a new image via `docker import` or `docker build`

## Documentation

- \ref installation "Installation" — all install methods
- \ref usage "Usage" — CLI flags, examples, TUI keybindings
- \ref architecture "Architecture" — how the merge pipeline works
- \ref contributing "Contributing" — development setup and project structure
- \ref troubleshooting "Troubleshooting" — common issues and fixes

## License

MIT License. See [LICENSE](https://github.com/XenonIsAwesome/docker-image-merge/blob/main/LICENSE) for details.

Docker and the Docker logo are trademarks of Docker, Inc. This project is not affiliated with or endorsed by Docker, Inc.
