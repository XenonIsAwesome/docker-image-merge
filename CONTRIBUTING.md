# Contributing to docker-imagemerge

## Development setup

```bash
git clone https://github.com/XenonIsAwesome/docker-image-merge.git
cd docker-image-merge
go build -trimpath -ldflags='-s -w' -o docker-imagemerge .
```

## Project structure

```
.
├── main.go                    # Entry point, Docker CLI plugin metadata
├── cmd/
│   └── root.go                # Cobra command, merge pipeline orchestration
├── internal/
│   ├── docker/
│   │   └── client.go          # Docker CLI wrapper (extract, import, build)
│   ├── flags/
│   │   └── options.go         # CLI flag definitions and validation
│   ├── merge/
│   │   ├── conflict.go        # ConflictKind, Resolution, FileInfo types
│   │   ├── filesystem.go      # DiffEngine, content hashing, hex dump
│   │   ├── apply.go           # ApplyResolutions, file copying
│   │   └── uid_linux.go       # Platform-specific UID/GID extraction
│   └── tui/
│       ├── model.go           # BubbleTea TUI model
│       └── styles.go          # Lipgloss styles
├── tests/
│   └── integration_test.go    # End-to-end CLI tests with real images
├── Dockerfile                 # Production build
├── Dockerfile.test            # Test runner
└── Makefile                   # Build/install shortcuts
```

## Running tests

```bash
# Unit tests
go test -v ./internal/...

# Integration tests (requires Docker)
go test -tags=integration -v -timeout=300s ./tests/

# Via Docker
docker build -t imagemerge-test -f Dockerfile.test .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock imagemerge-test
```

## Code conventions

- All exported symbols must have GoDoc-compatible docstrings
- Use `_ =` for intentionally ignored errors (lint compliance)
- Platform-specific code goes in `_linux.go` / `_other.go` files
- Integration tests use the `integration` build tag

## Pull requests

1. Fork the repo and create a feature branch
2. Add tests for new functionality
3. Ensure `go test -v ./internal/...` passes
4. Ensure `go vet ./...` has no warnings
5. Open a PR with a clear description

## Release process

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers the release workflow which builds multi-arch binaries and creates a GitHub Release.
