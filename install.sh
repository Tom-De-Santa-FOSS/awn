#!/usr/bin/env bash
# install.sh — curl-installable installer for awn
# Usage: curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
set -euo pipefail

REPO="${REPO:-Tom-De-Santa-FOSS/awn}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CURL_CMD="${CURL_CMD:-curl -fsSL}"
VERSION="${VERSION:-}"

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

get_latest_version() {
  $CURL_CMD "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/'
}

do_install() {
  local os="$1" arch="$2"
  local url
  url="$(build_download_url "$VERSION" "$os" "$arch")"

  local tmpdir
  tmpdir="$(mktemp -d)"

  $CURL_CMD -o "$tmpdir/awn.tar.gz" "$url"
  tar xzf "$tmpdir/awn.tar.gz" -C "$tmpdir"

  mkdir -p "$INSTALL_DIR"
  cp "$tmpdir/awn" "$tmpdir/awnd" "$INSTALL_DIR/"
  chmod +x "$INSTALL_DIR/awn" "$INSTALL_DIR/awnd"

  rm -rf "$tmpdir"
}

install_skill() {
  local skill_dir="${1:-$HOME/.claude/skills/awn}"
  mkdir -p "$skill_dir"
  $CURL_CMD -o "$skill_dir/SKILL.md" \
    "https://raw.githubusercontent.com/${REPO}/master/.claude/skills/awn/SKILL.md"
}

# Allow sourcing without executing main
if [ "${1:-}" = "--source-only" ]; then
  return 0 2>/dev/null || true
fi

# Main entrypoint
main() {
  echo "Installing awn..."
  local os arch
  os="$(detect_os)"
  arch="$(detect_arch)"

  if [ -z "$VERSION" ]; then
    VERSION="$(get_latest_version)"
  fi
  echo "Version: $VERSION | OS: $os | Arch: $arch"

  do_install "$os" "$arch"
  install_skill
  echo "Installed awn and awnd to $INSTALL_DIR"
  echo "Installed Claude Code skill to ~/.claude/skills/awn/SKILL.md"
  echo ""
  echo "Make sure $INSTALL_DIR is in your PATH."
}
main
