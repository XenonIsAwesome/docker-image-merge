# Contributing {#contributing}

## Development Setup

### Prerequisites

- Go 1.22+
- Docker (for integration tests and builds)
- golangci-lint (for linting)

### Clone and Build

```sh
git clone https://github.com/XenonIsAwesome/docker-image-merge.git
cd docker-image-merge
go build -o docker-imagemerge .
```

### Build with Docker

```sh
make build
```

## Running Tests

### Unit Tests

```sh
go test ./internal/...
```

Or via Docker:

```sh
docker build -f Dockerfile.test -t imagemerge-test .
docker run --rm imagemerge-test go test ./internal/... -v
```

### Integration Tests

Integration tests require Docker and actually pull images, extract filesystems, and create output images.

```sh
go test -tags=integration ./tests/ -v -timeout=300s
```

Or via Docker:

```sh
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  imagemerge-test go test -tags=integration ./tests/ -v
```

### Linting

```sh
golangci-lint run
```

Or via Docker:

```sh
docker run --rm -v $(pwd):/src -w /src golangci/golangci-lint:latest golangci-lint run
```

## Project Structure

```
main.go                       Entry point, Docker CLI plugin metadata
cmd/root.go                   Cobra command, merge pipeline orchestration
internal/
├── docker/client.go          Docker CLI wrapper
├── flags/options.go          CLI flag definitions
├── merge/                    Diff engine, conflict resolution, filesystem ops
│   ├── conflict.go           ConflictKind, Resolution, Conflict, FileInfo
│   ├── filesystem.go         DiffEngine, content hashing
│   ├── apply.go              ApplyResolutions, BuildDockerfileContent
│   ├── uid_linux.go          Linux UID/GID extraction
│   └── uid_other.go          Non-Linux UID/GID stub
└── tui/                      BubbleTea interactive TUI
    ├── model.go              TUI model and state machine
    └── styles.go             Lipgloss styles and renderers
tests/                        End-to-end integration tests
scripts/                      Installation scripts
```

## Code Conventions

- All exported symbols must have GoDoc comments
- Use `_ =` for intentionally ignored error returns (with `//nolint:errcheck`)
- Platform-specific code goes in `*_linux.go` / `*_other.go` build-tagged files
- Integration tests use the `//go:build integration` build tag
- Linter config is in `.golangci.yml` — run `golangci-lint run` before submitting PRs

## Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./internal/... && go test -tags=integration ./tests/ -v`
5. Run linter: `golangci-lint run`
6. Submit a pull request

## Release Process

Releases are automated via GitHub Actions:

1. Update version references if needed
2. Commit changes
3. Create and push a tag:
   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```
4. The release workflow builds cross-platform binaries and creates a GitHub Release
