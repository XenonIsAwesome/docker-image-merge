# Architecture {#architecture}

## Overview

`docker-image-merge` follows a four-stage pipeline: **Extract → Diff → Resolve → Import**.

```
┌─────────────┐    ┌─────────────┐
│  Image A     │    │  Image B     │
│  (base)      │    │  (incoming)  │
└──────┬──────┘    └──────┬──────┘
       │ docker export    │ docker export
       ▼                  ▼
┌─────────────┐    ┌─────────────┐
│  Filesystem │    │  Filesystem │
│  /tmp/a/    │    │  /tmp/b/    │
└──────┬──────┘    └──────┬──────┘
       │                  │
       └────────┬─────────┘
                │ DiffEngine.Run()
                ▼
       ┌────────────────┐
       │   Conflicts[]  │
       │   + DiffStats  │
       └───────┬────────┘
               │ Resolution
               ▼
       ┌────────────────┐
       │  Resolved      │
       │  Filesystem    │
       └───────┬────────┘
               │ docker import / docker build
               ▼
       ┌────────────────┐
       │  Output Image  │
       └────────────────┘
```

## Stage 1: Extract

The `docker.ExtractFilesystem` function:

1. Creates a temporary container from the image (`docker create`)
2. Exports its flattened filesystem to a tar archive (`docker export`)
3. Extracts the tar into a temporary directory
4. Removes the temporary container

This produces a single-layer snapshot of the image's filesystem, regardless of how many layers the original image has.

## Stage 2: Diff

The `merge.DiffEngine` walks both directory trees simultaneously using a sorted union walk. For each path encountered, it classifies the difference:

| Classification | Meaning |
|---------------|---------|
| `OnlyA` | Path exists only in image A |
| `OnlyB` | Path exists only in image B |
| `Same` | Path exists in both with identical content |
| `ContentConflict` | Path exists in both with different content |
| `TypeChange` | File in one, directory (or other type) in the other |
| `PermOnly` | Same content but different permissions/ownership |
| `BothDeleted` | Path does not exist in either (should not occur) |

Content comparison uses **xxhash** for fast hashing. Files are first compared by size — only files of equal size are hash-compared. Binary files are detected by null byte inspection and displayed as hex dumps in the TUI.

## Stage 3: Resolve

Conflicts are resolved through one of four strategies:

- **Interactive** — The TUI presents each conflict that `NeedsResolution()` (content changes, type changes, permission changes). Non-conflicting diffs (OnlyA/OnlyB) are auto-resolved before launching the TUI.
- **Auto-A** — Take image A's version for all conflicts.
- **Auto-B** — Take image B's version for all conflicts.
- **Fail** — Exit with an error if any conflicts need resolution.

The `merge.ApplyResolutions` function starts with image A's filesystem as the base, overlays image B's unique files, and applies per-file resolution choices. It preserves file permissions, ownership (UID/GID), and symlinks.

## Stage 4: Import

Two modes for creating the output image:

### Squashed (`--squash`)

The merged directory is piped through tar into `docker import` as a single flattened layer. This is fast and small but loses all layer history.

### Layered (default)

A minimal Dockerfile is generated that copies the merged filesystem on top of image A:

```dockerfile
FROM <image-a>
COPY . /tmp/merged/
RUN cp -a /tmp/merged/. / 2>/dev/null; rm -rf /tmp/merged
```

This preserves image A's layer structure. The `2>/dev/null` suppresses errors from Docker-managed files (`/etc/resolv.conf`, `/etc/hosts`, etc.).

## Project Structure

```
main.go                       Entry point, Docker CLI plugin metadata
cmd/root.go                   Cobra command, merge pipeline orchestration
internal/
├── docker/client.go          Docker CLI wrapper (extract, import, build)
├── flags/options.go          CLI flag definitions and validation
├── merge/
│   ├── conflict.go           ConflictKind, Resolution, Conflict, FileInfo
│   ├── filesystem.go         DiffEngine, content hashing, binary detection
│   ├── apply.go              ApplyResolutions, BuildDockerfileContent
│   ├── uid_linux.go          Linux-specific UID/GID extraction
│   └── uid_other.go          No-op UID/GID for non-Linux
└── tui/
    ├── model.go              BubbleTea TUI model
    └── styles.go             Lipgloss styles
tests/
└── integration_test.go       End-to-end CLI tests (requires Docker)
scripts/
├── install.sh                Cross-platform shell installer
└── install.ps1               Windows PowerShell installer
```
