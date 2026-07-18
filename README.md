# docker-image-merge

Merge the filesystems of two Docker images with interactive conflict resolution.

## Disclaimer

Docker is a trademark of Docker, Inc. This project is not affiliated with,
endorsed by, or sponsored by Docker, Inc.

## Installation

### Binary (recommended)

Download a pre-built binary from [Releases](https://github.com/XenonIsAwesome/docker-image-merge/releases).

### From source

```bash
git clone https://github.com/XenonIsAwesome/docker-image-merge.git
cd docker-image-merge
go build -trimpath -ldflags='-s -w' -o docker-image-merge .
```

### Docker

```bash
docker build -t docker-image-merge .
```

## Usage

### Docker CLI plugin

Copy the binary to a directory on your `$PATH`:

```bash
sudo cp docker-image-merge /usr/local/bin/docker-image-merge
```

Then run:

```bash
docker image-merge <image-a> <image-b> <output-image>
```

### Standalone

```bash
./docker-image-merge <image-a> <image-b> <output-image>
```

## Examples

### Interactive TUI (default)

```bash
docker image-merge nginx:1.25-alpine my-custom-nginx:latest merged-nginx:latest
```

Opens a full-screen TUI where you resolve each conflicting file interactively.

### Non-interactive strategies

```bash
# Always take image A's version on conflict
docker image-merge --strategy=auto-a nginx:latest my-custom:latest merged:latest

# Always take image B's version on conflict
docker image-merge --strategy=auto-b nginx:latest my-custom:latest merged:latest

# Fail on first conflict
docker image-merge --strategy=fail nginx:latest my-custom:latest merged:latest
```

### Squashed output (single layer)

```bash
docker image-merge --squash nginx:latest my-custom:latest merged:latest
```

### Inherit metadata from image B

```bash
docker image-merge --metadata-from=b nginx:latest my-custom:latest merged:latest
```

## Flags

| Flag | Description | Default |
|---|---|---|
| `--strategy` | Conflict resolution strategy: `interactive`, `auto-a`, `auto-b`, `fail` | `interactive` |
| `--squash` | Produce a single-layer image via `docker import` | `false` |
| `--metadata-from` | Inherit base image metadata from `a` or `b` | `a` |
| `--message` | Commit message for the merged image | `Merged image` |
| `--change` | Apply a Dockerfile `CMD`/`ENTRYPOINT`/etc. change | — |
| `--platform` | Target platform (e.g. `linux/amd64`) | host |
| `--verbose` | Enable verbose output | `false` |

## How it works

1. **Extract** both images' filesystems into temporary directories.
2. **Diff** the two trees using xxhash-based content comparison.
3. **Resolve** conflicts interactively or via the chosen strategy.
4. **Import** the merged tree as a new Docker image.

The layered build mode preserves image A's layers by adding a final COPY layer.
The `--squash` mode produces a single flat layer via `docker import`.

## License

[MIT](LICENSE)
