# Troubleshooting {#troubleshooting}

## "Cannot connect to the Docker daemon"

The Docker daemon is not running or the current user does not have permission to access it.

```sh
# Check if Docker is running
docker info

# On Linux, add your user to the docker group
sudo usermod -aG docker $USER
# Then log out and back in
```

## "Error: image not found"

The specified image could not be pulled from the registry. Check:

- The image name and tag are correct
- You have internet connectivity
- You are logged in to the registry (for private images): `docker login`

## "Conflict" errors during merge

If using `--strategy fail`, conflicts are expected. Use `--strategy auto-a` or `--strategy auto-b` to auto-resolve, or omit the flag for interactive mode.

## TUI does not display correctly

The interactive TUI requires a terminal with at least 80 columns and 24 rows. If your terminal is too small, the TUI may render incorrectly.

Try resizing your terminal window, or use a non-interactive strategy:

```sh
docker imagemerge <a> <b> <out> --strategy auto-a
```

## "Permission denied" when installing

The install script tries to write to `~/.docker/cli-plugins/`. If this directory requires elevated permissions:

```sh
# Linux / macOS — use sudo for system-wide install
curl -fsSL https://xenon.github.io/docker-image-merge/install.sh | sudo sh -s -- --system

# Windows — run PowerShell as Administrator
```

## Binary is not recognized as a Docker plugin

Ensure the binary is in the correct location and is executable:

```sh
# Check the location
ls -la ~/.docker/cli-plugins/docker-imagemerge

# Make it executable
chmod +x ~/.docker/cli-plugins/docker-imagemerge

# Verify Docker sees it
docker info | grep -i imagemerge
```

## "No differences found. Images are identical"

This is not an error — it means the two images have the same filesystem. No merge is needed.

## Merge produces unexpected results

The diff engine compares flattened filesystems. Layered history is not considered. If an image modifies a file across multiple layers, only the final state is compared.

Use `--verbose` to see detailed statistics about what was detected:

```sh
docker imagemerge <a> <b> <out> --verbose
```

## Large images are slow to process

Extraction and comparison time scales with the number of files and total size. Very large images (several GB) may take several minutes to process. This is expected.

## "unknown strategy" error

Valid strategies are: `interactive`, `auto-a`, `auto-b`, `fail`. Check your flag value:

```sh
docker imagemerge <a> <b> <out> --strategy interactive
```
