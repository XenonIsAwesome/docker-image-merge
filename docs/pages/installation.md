# Installation {#installation}

## Quick Install (Recommended)

### Linux / macOS

```sh
curl -fsSL https://xenonisawesome.github.io/docker-image-merge/install.sh | sh
```

This detects your OS and architecture, downloads the latest release from GitHub, and installs it to `~/.docker/cli-plugins/docker-imagemerge`.

### Linux / macOS — System-wide

```sh
curl -fsSL https://xenonisawesome.github.io/docker-image-merge/install.sh | sudo sh -s -- --system
```

Installs to `/usr/local/lib/docker/cli-plugins/docker-imagemerge` (requires root).

### Linux / macOS — Custom directory

```sh
curl -fsSL https://xenonisawesome.github.io/docker-image-merge/install.sh | sh -s -- --dir /opt/docker-plugins
```

### Windows (PowerShell)

```powershell
iwr -useb https://xenonisawesome.github.io/docker-image-merge/install.ps1 | iex
```

Installs to `%USERPROFILE%\.docker\cli-plugins\docker-imagemerge.exe`.

### Windows — System-wide

```powershell
iwr -useb https://xenonisawesome.github.io/docker-image-merge/install.ps1 -OutFile install.ps1
.\install.ps1 -System
```

Installs to `%ProgramData%\Docker\cli-plugins\docker-imagemerge.exe`.

### Windows — Custom directory

```powershell
iwr -useb https://xenonisawesome.github.io/docker-image-merge/install.ps1 -OutFile install.ps1
.\install.ps1 -Dir C:\tools\docker-plugins
```

## From Source

Requires Go 1.22+.

```sh
git clone https://github.com/XenonIsAwesome/docker-image-merge.git
cd docker-image-merge
go build -o docker-imagemerge .
```

Then copy `docker-imagemerge` to your Docker CLI plugins directory:

```sh
# Linux / macOS
mkdir -p ~/.docker/cli-plugins
cp docker-imagemerge ~/.docker/cli-plugins/
chmod +x ~/.docker/cli-plugins/docker-imagemerge

# Windows (PowerShell)
mkdir -p "$env:USERPROFILE\.docker\cli-plugins"
cp docker-imagemerge.exe "$env:USERPROFILE\.docker\cli-plugins\"
```

## Docker

```sh
git clone https://github.com/XenonIsAwesome/docker-image-merge.git
cd docker-image-merge
docker build -t docker-image-merge .
```

Run via Docker (note: needs access to the Docker socket):

```sh
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  docker-image-merge imagemerge alpine:3.19 busybox:1.36 my-image
```

## Manual Binary Download

Download the latest binary for your platform from [GitHub Releases](https://github.com/XenonIsAwesome/docker-image-merge/releases), then place it in your Docker CLI plugins directory.

### Install locations

| Platform | User-level | System-level |
|----------|-----------|--------------|
| Linux / macOS | `~/.docker/cli-plugins/` | `/usr/local/lib/docker/cli-plugins/` |
| Windows | `%USERPROFILE%\.docker\cli-plugins\` | `%ProgramData%\Docker\cli-plugins\` |

## Verify Installation

```sh
docker imagemerge --help
```

If the command is recognized, the installation was successful.
