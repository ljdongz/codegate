#!/bin/sh
set -e

REPO="ljdongz/codegate"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)
    echo "Error: unsupported OS: $OS"
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest version
VERSION="$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i "^location:" | sed 's|.*/tag/||' | tr -d '\r\n')"
if [ -z "$VERSION" ]; then
  echo "Error: could not determine latest version"
  exit 1
fi

FILENAME="codegate_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

echo "Downloading codegate $VERSION for $OS/$ARCH..."

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

curl -sL "$URL" -o "$TMPDIR/$FILENAME"
tar -xzf "$TMPDIR/$FILENAME" -C "$TMPDIR"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/codegate" "$INSTALL_DIR/codegate"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMPDIR/codegate" "$INSTALL_DIR/codegate"
fi

echo "codegate $VERSION installed to $INSTALL_DIR/codegate"
