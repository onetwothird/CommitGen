#!/bin/sh
# Install commitgen on macOS or Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/onetwothird/CommitGen/main/install.sh | sh
#
# Downloads the latest release binary for your OS/CPU into ~/.local/bin
# (override with COMMITGEN_INSTALL_DIR). Falls back to `go install` when no
# release binary is available.
set -eu

REPO="onetwothird/CommitGen"
DIR="${COMMITGEN_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported CPU: $arch" >&2; exit 1 ;;
esac

url=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null |
  grep -o "https://[^\"]*_${os}_${arch}\.tar\.gz" | head -n 1 || true)

if [ -z "$url" ]; then
  if command -v go >/dev/null 2>&1; then
    echo "No release binary found; building with Go instead..."
    go install "github.com/$REPO@latest"
    echo "Installed to $(go env GOPATH)/bin/commitgen"
    exit 0
  fi
  echo "No release binary for ${os}/${arch} and Go is not installed." >&2
  echo "Install Go from https://go.dev/dl/ and run: go install github.com/$REPO@latest" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/commitgen.tar.gz"
tar -xzf "$tmp/commitgen.tar.gz" -C "$tmp"
mkdir -p "$DIR"
install -m 0755 "$tmp/commitgen" "$DIR/commitgen"
echo "Installed $DIR/commitgen"

case ":$PATH:" in
  *":$DIR:"*) ;;
  *) echo "Note: $DIR is not on your PATH. Add this to your shell profile:"
     echo "  export PATH=\"$DIR:\$PATH\"" ;;
esac
echo "Next: run 'commitgen setup' (or just 'commitgen' for offline mode)."
