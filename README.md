# docker-imagemerge

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
go build -trimpath -ldflags='-s -w' -o docker-imagemerge .
```

### Docker

```bash
docker build -t docker-imagemerge .
```

## Usage

### Docker CLI plugin

Copy the binary to a directory on your `$PATH`:

```bash
sudo cp docker-imagemerge /usr/local/bin/docker-imagemerge
```

Then run:

```bash
docker imagemerge <image-a> <image-b> <output-image>
```

### Standalone

```bash
./docker-imagemerge <image-a> <image-b> <output-image>
```

## Examples

### Interactive TUI (default)

```bash
docker imagemerge nginx:1.25-alpine my-custom-nginx:latest merged-nginx:latest
```

Opens a full-screen TUI where you resolve each conflicting file interactively.

### Non-interactive strategies

```bash
# Always take image A's version on conflict
docker imagemerge --strategy=auto-a nginx:latest my-custom:latest merged:latest

# Always take image B's version on conflict
docker imagemerge --strategy=auto-b nginx:latest my-custom:latest merged:latest

# Fail on first conflict
docker imagemerge --strategy=fail nginx:latest my-custom:latest merged:latest
```

### Squashed output (single layer)

```bash
docker imagemerge --squash nginx:latest my-custom:latest merged:latest
```

### Inherit metadata from image B

```bash
docker imagemerge --metadata-from=b nginx:latest my-custom:latest merged:latest
```

### Apply Dockerfile changes

```bash
docker imagemerge --squash --change "ENV FOO=bar" --change "EXPOSE 8080" \
  nginx:latest my-custom:latest merged:latest
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

## TUI keybindings

| Key | Action |
|---|---|
| `a` | Take image A's version |
| `b` | Take image B's version |
| `s` | Skip (keep A) |
| `e` | Open `$EDITOR` for manual merge |
| `A` | Take A for ALL remaining conflicts |
| `B` | Take B for ALL remaining conflicts |
| `n` / `→` | Next unresolved conflict |
| `p` / `←` | Previous conflict |
| `q` / `Ctrl+C` | Abort without applying |

## Troubleshooting

### "accepts 3 arg(s), received 4"

The binary name must match `^[a-z][a-z0-9]*$` for Docker CLI plugins. Use `docker-imagemerge` (not `docker-image-merge`).

### Layered build fails with "File exists"

Docker-managed files like `/etc/resolv.conf`, `/etc/hosts`, and `/var/lock` cannot be overwritten during build. The tool ignores these errors and continues. If the build still fails, use `--squash` mode instead.

### "installation not allowed to Create organization package"

GHCR (GitHub Container Registry) is not enabled for your account. The binary releases work without GHCR.

### Binary files show garbled text

The TUI detects binary files and shows a hex dump instead. If you still see garbled output, the file may be a text file with unusual encoding.

### Merge is slow for large images

The diff engine reads and hashes every file. For very large images (1GB+), expect 30-60 seconds for the diff phase. The `--squash` mode is faster than the layered build.

## Testing

### Unit tests

```bash
go test -v ./internal/...
```

### Integration tests

```bash
go test -tags=integration -v -timeout=300s ./tests/
```

Integration tests require Docker and will pull `alpine:3.19`, `busybox:1.36`, and `nginx:1.25-alpine`.

## License

[MIT](LICENSE)
