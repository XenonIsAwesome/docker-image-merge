#!/bin/sh
# Cross-platform installer for docker-image-merge.
#
# Detects OS and architecture, downloads the latest GitHub release binary,
# and installs it as a Docker CLI plugin.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/XenonIsAwesome/docker-image-merge/main/scripts/install.sh | sh
#   ./install.sh                    # install to ~/.docker/cli-plugins
#   sudo ./install.sh --system      # install to /usr/local/lib/docker/cli-plugins

set -e

REPO="XenonIsAwesome/docker-image-merge"
BINARY="docker-imagemerge"
CLI_PLUGINS_USER="$HOME/.docker/cli-plugins"
CLI_PLUGINS_SYSTEM="/usr/local/lib/docker/cli-plugins"

usage() {
    echo "Usage: $0 [--system] [--dir <path>]"
    echo ""
    echo "Options:"
    echo "  --system    Install system-wide to /usr/local/lib/docker/cli-plugins/"
    echo "  --dir PATH  Install to a custom directory"
    echo "  -h, --help  Show this help"
    exit 0
}

# Parse arguments.
INSTALL_DIR="$CLI_PLUGINS_USER"
while [ $# -gt 0 ]; do
    case "$1" in
        --system) INSTALL_DIR="$CLI_PLUGINS_SYSTEM" ;;
        --dir)    INSTALL_DIR="$2"; shift ;;
        -h|--help) usage ;;
        *)        echo "Unknown option: $1"; usage ;;
    esac
    shift
done

# Detect OS.
detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)       echo "unsupported" ;;
    esac
}

# Detect architecture.
detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)   echo "arm64" ;;
        armv7l|armhf)    echo "armv7" ;;
        *)               echo "unsupported" ;;
    esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$OS" = "unsupported" ]; then
    echo "Error: unsupported OS $(uname -s)"
    echo "Windows users: use install.ps1 or download manually from"
    echo "  https://github.com/$REPO/releases"
    exit 1
fi
if [ "$ARCH" = "unsupported" ]; then
    echo "Error: unsupported architecture $(uname -m)"
    exit 1
fi

echo "Detected: $OS/$ARCH"

# Resolve latest release tag from GitHub API.
echo "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release"
    echo "Specify a version: $0 --version v0.1.0-rc8"
    exit 1
fi

VERSION="${VERSION:-$LATEST}"
echo "Installing $VERSION for $OS/$ARCH..."

# Build download URL. Release archives follow the pattern:
#   docker-imagemerge-<os>-<arch>.tar.gz   (linux, darwin)
#   docker-imagemerge-windows-amd64.zip     (windows)
EXT="tar.gz"
URL_BASE="https://github.com/$REPO/releases/download/$VERSION"
ARCHIVE_NAME="docker-imagemerge-${OS}-${ARCH}.${EXT}"

if [ "$OS" = "windows" ]; then
    ARCHIVE_NAME="docker-imagemerge-windows-${ARCH}.zip"
    EXT="zip"
fi

DOWNLOAD_URL="${URL_BASE}/${ARCHIVE_NAME}"

# Download to a temp directory.
TMPDIR="${TMPDIR:-/tmp}"
STAGING="$TMPDIR/docker-imagemerge-install"
rm -rf "$STAGING"
mkdir -p "$STAGING"

echo "Downloading $DOWNLOAD_URL ..."
if command -v curl >/dev/null 2>&1; then
    curl -fSL -o "$STAGING/archive.$EXT" "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$STAGING/archive.$EXT" "$DOWNLOAD_URL"
else
    echo "Error: curl or wget is required"
    exit 1
fi

# Extract.
echo "Extracting..."
if [ "$EXT" = "zip" ]; then
    unzip -qo "$STAGING/archive.$EXT" -d "$STAGING"
else
    tar xzf "$STAGING/archive.$EXT" -C "$STAGING"
fi

# Find the binary.
BINARY_PATH=$(find "$STAGING" -name "$BINARY" -type f | head -1)
if [ -z "$BINARY_PATH" ]; then
    # Try without extension (Windows builds may produce .exe).
    BINARY_PATH=$(find "$STAGING" -name "${BINARY}*" -type f | head -1)
fi
if [ -z "$BINARY_PATH" ]; then
    echo "Error: binary $BINARY not found in archive"
    echo "Contents:"
    ls -la "$STAGING/"
    exit 1
fi

# Install.
echo "Installing to $INSTALL_DIR/$BINARY ..."
mkdir -p "$INSTALL_DIR"
cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

# Verify.
if "$INSTALL_DIR/$BINARY" --help >/dev/null 2>&1; then
    echo ""
    echo "Installed successfully!"
    echo "  $INSTALL_DIR/$BINARY"
    echo ""
    echo "Usage: docker imagemerge <image-a> <image-b> <output-image>"
else
    echo ""
    echo "Installed: $INSTALL_DIR/$BINARY"
    echo "(could not verify — Docker may not be running)"
fi

# Cleanup.
rm -rf "$STAGING"
