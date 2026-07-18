# Usage {#usage}

## Basic Syntax

```sh
docker imagemerge <image-a> <image-b> <output-image> [flags]
```

- **image-a** — the "base" image whose filesystem starts as the merged result
- **image-b** — the "incoming" image whose changes are overlaid
- **output-image** — the tag for the newly created merged image

## Conflict Resolution Strategies

| Flag | Behavior |
|------|----------|
| `--strategy interactive` (default) | Launches a TUI for per-file resolution |
| `--strategy auto-a` | Automatically takes image A's version for every conflict |
| `--strategy auto-b` | Automatically takes image B's version for every conflict |
| `--strategy fail` | Exits with an error if any conflicts are found |

```sh
# Interactive (default)
docker imagemerge alpine:3.19 busybox:1.36 merged

# Take A's version for all conflicts
docker imagemerge alpine:3.19 busybox:1.36 merged -s auto-a

# Take B's version for all conflicts
docker imagemerge alpine:3.19 busybox:1.36 merged -s auto-b

# Fail on conflicts
docker imagemerge alpine:3.19 busybox:1.36 merged -s fail
```

## Output Modes

### Layered (default)

Preserves image A's layer structure. The merged filesystem is copied on top of image A as a new layer.

```sh
docker imagemerge alpine:3.19 busybox:1.36 merged
```

### Squashed

Produces a single-layer image via `docker import`. Loses layer history but produces a smaller image.

```sh
docker imagemerge alpine:3.19 busybox:1.36 merged --squash
```

## Metadata Inheritance

Control which image's configuration (ENV, CMD, ENTRYPOINT, etc.) is inherited:

```sh
# Inherit from image A (default)
docker imagemerge alpine:3.19 busybox:1.36 merged --metadata-from a

# Inherit from image B
docker imagemerge alpine:3.19 busybox:1.36 merged --metadata-from b
```

## Dockerfile Changes

Apply Dockerfile-style instructions during import (like `docker import --change`):

```sh
docker imagemerge alpine:3.19 busybox:1.36 merged \
  --change "ENV MY_VAR=hello" \
  --change "EXPOSE 8080"
```

## Platform Targeting

Build for a specific platform:

```sh
docker imagemerge alpine:3.19 busybox:1.36 merged --platform linux/arm64
```

## Commit Message

Set a custom commit message in the image history:

```sh
docker imagemerge alpine:3.19 busybox:1.36 merged -m "My custom merge"
```

## Verbose Output

Show detailed diff statistics during the merge:

```sh
docker imagemerge alpine:3.19 busybox:1.36 merged -v
```

## All Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--strategy` | `-s` | `interactive` | Conflict resolution strategy |
| `--squash` | | `false` | Force single-layer output |
| `--metadata-from` | | `a` | Which image's metadata to inherit |
| `--message` | `-m` | | Commit message for the output image |
| `--change` | | | Dockerfile instructions to apply |
| `--platform` | | | Target platform |
| `--verbose` | `-v` | `false` | Verbose output |

## TUI Keybindings

When using interactive mode, the following keys are available:

| Key | Action |
|-----|--------|
| `a` | Resolve with image A's version |
| `b` | Resolve with image B's version |
| `s` | Skip (keep image A) |
| `e` | Open `$EDITOR` for manual merge |
| `A` | Bulk-resolve ALL remaining with A |
| `B` | Bulk-resolve ALL remaining with B |
| `n` / `→` | Next unresolved conflict |
| `p` / `←` | Previous conflict |
| `Enter` / `Space` | Advance to next (finish if all resolved) |
| `q` / `Ctrl+C` | Abort without applying |

## Standalone Mode

You can also run the binary directly without Docker CLI plugin installation:

```sh
./docker-imagemerge alpine:3.19 busybox:1.36 merged
```
