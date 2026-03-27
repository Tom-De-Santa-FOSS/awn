#!/usr/bin/env bash
# install.sh — curl-installable installer for awn
# Usage: curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
set -euo pipefail

REPO="${REPO:-Tom-De-Santa-FOSS/awn}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

detect_os() {
  local os="${MOCK_UNAME_S:-$(uname -s)}"
  case "$os" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac
}

detect_arch() {
  local arch="${MOCK_UNAME_M:-$(uname -m)}"
  case "$arch" in
    x86_64)  echo "amd64" ;;
    aarch64) echo "arm64" ;;
    arm64)   echo "arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac
}

build_download_url() {
  local version="$1" os="$2" arch="$3"
  echo "https://github.com/${REPO}/releases/download/v${version}/awn-${os}-${arch}.tar.gz"
}

# Allow sourcing without executing main
if [ "${1:-}" = "--source-only" ]; then
  return 0 2>/dev/null || true
fi
