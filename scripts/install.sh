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
  cat > "$skill_dir/awn.md" << 'SKILL_EOF'
---
name: awn
description: "TUI automation for AI agents. Use when the user needs to interact with terminal applications programmatically — create sessions, take screenshots, send input, wait for text. Trigger on: 'terminal automation', 'TUI session', 'screenshot terminal', 'awn', 'awnd'."
trigger: strategy
---

# awn — TUI Automation for AI Agents

Manage headless terminal sessions via JSON-RPC 2.0 over WebSocket.

## Quick Start

```bash
awnd &                              # Start daemon (localhost:7600)
awn create bash                     # Create session
awn screenshot <id>                 # Capture terminal state
awn input <id> "ls -la\n"          # Send input
awn wait-for-text <id> "done"      # Wait for text to appear
awn close <id>                      # End session
```

## Environment Variables

- `AWN_ADDR` — Daemon address (default: `ws://localhost:7600`)
- `AWN_TOKEN` — Bearer token for authentication (optional)

## RPC Methods

| Method | Params | Returns |
|--------|--------|---------|
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id}` | `{rows, cols, lines, cursor}` |
| `input` | `{id, data}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions: [id...]}` |

## When to Use

- Automating interactive terminal applications (htop, vim, ncurses)
- Taking screenshots of terminal state for AI agent vision
- Sending keystrokes to running TUI programs
- Waiting for specific output before proceeding
SKILL_EOF
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
  echo "Installed Claude Code skill to ~/.claude/skills/awn/"
  echo ""
  echo "Make sure $INSTALL_DIR is in your PATH."
}
main
