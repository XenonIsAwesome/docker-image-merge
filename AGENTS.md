# AGENTS.md

Single-package Go CLI (`docker-imagemerge`) that merges two Docker image filesystems with interactive TUI conflict resolution.

## Build & verify

```bash
go build -trimpath -ldflags='-s -w' -o docker-imagemerge .
go vet ./...
golangci-lint run          # uses .golangci.yml (errcheck, govet, revive, staticcheck, unused, ineffassign)
go test -v ./internal/...  # unit tests
```

Integration tests require Docker and pull alpine:3.19, busybox:1.36, nginx:1.25-alpine:

```bash
go test -tags=integration -v -timeout=300s ./tests/
```

CI order: build → vet → unit tests → lint, then integration tests (depend on unit passing), then Docker build smoke test.

## Architecture

- `main.go` — entry point; handles Docker CLI plugin metadata subcommand, strips plugin prefix, delegates to Cobra.
- `cmd/root.go` — Cobra command, full merge pipeline orchestration.
- `internal/docker/` — Docker CLI wrapper (extract, import, build). Shells out to `docker` binary; no Docker SDK.
- `internal/merge/` — DiffEngine (xxhash content hashing), conflict types, apply resolutions.
- `internal/flags/` — CLI flag definitions and validation.
- `internal/tui/` — BubbleTea TUI for interactive conflict resolution.
- `tests/integration_test.go` — End-to-end CLI tests; uses `integration` build tag, pre-pulls images in TestMain.

## Key conventions

- Go 1.22 module, built with Go 1.24 in CI/Docker.
- Binary must be named `docker-imagemerge` (not `docker-image-merge`) for Docker CLI plugin compatibility (`^[a-z][a-z0-9]+$`).
- All exported symbols must have GoDoc docstrings.
- Use `_ =` for intentionally ignored errors (lint compliance).
- Platform-specific code: `_linux.go` / `_other.go` files.
- Integration tests gated behind `//go:build integration` tag.
- `tests/` directory excluded from linting (per `.golangci.yml`).
- Uses `//nolint:errcheck` on intentional error ignores (e.g., `defer` calls).

## Gotchas

- Layered build mode can fail on Docker-managed files (`/etc/resolv.conf`, `/etc/hosts`). Tool falls back to squash mode automatically.
- `--squash` mode uses `docker import` (single flat layer); layered mode builds on top of image A preserving its layers.
- The Dockerfile uses `FROM scratch` for production, so the binary must be statically linked (`CGO_ENABLED=0`).
