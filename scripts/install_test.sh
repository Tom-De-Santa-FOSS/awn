#!/usr/bin/env bash
# Tests for install.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RESULTS_FILE="$(mktemp)"
echo "0 0" > "$RESULTS_FILE"

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  local pass fail
  read -r pass fail < "$RESULTS_FILE"
  if [ "$expected" = "$actual" ]; then
    echo "  PASS: $label"
    echo "$((pass + 1)) $fail" > "$RESULTS_FILE"
  else
    echo "  FAIL: $label — expected '$expected', got '$actual'"
    echo "$pass $((fail + 1))" > "$RESULTS_FILE"
  fi
}

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR" "$RESULTS_FILE"' EXIT

echo "=== install.sh tests ==="

# Test 1: detect_os returns linux or darwin
echo "Test: detect_os returns normalized OS name"
(
  source "$SCRIPT_DIR/install.sh" --source-only
  os="$(MOCK_UNAME_S=Linux detect_os)"
  assert_eq "Linux -> linux" "linux" "$os"

  os="$(MOCK_UNAME_S=Darwin detect_os)"
  assert_eq "Darwin -> darwin" "darwin" "$os"
)

# Test 2: detect_arch returns normalized arch
echo "Test: detect_arch returns normalized arch name"
(
  source "$SCRIPT_DIR/install.sh" --source-only
  arch="$(MOCK_UNAME_M=x86_64 detect_arch)"
  assert_eq "x86_64 -> amd64" "amd64" "$arch"

  arch="$(MOCK_UNAME_M=aarch64 detect_arch)"
  assert_eq "aarch64 -> arm64" "arm64" "$arch"

  arch="$(MOCK_UNAME_M=arm64 detect_arch)"
  assert_eq "arm64 -> arm64" "arm64" "$arch"
)

# Test 3: build_download_url constructs correct URL
echo "Test: build_download_url constructs GitHub release URL"
(
  source "$SCRIPT_DIR/install.sh" --source-only
  REPO="Tom-De-Santa-FOSS/awn"
  url="$(build_download_url "0.1.0" "linux" "amd64")"
  assert_eq "URL correct" "https://github.com/Tom-De-Santa-FOSS/awn/releases/download/v0.1.0/awn-linux-amd64.tar.gz" "$url"
)

# Test 4: install_dir defaults to ~/.local/bin
echo "Test: default install directory"
(
  source "$SCRIPT_DIR/install.sh" --source-only
  assert_eq "default install dir" "$HOME/.local/bin" "$INSTALL_DIR"
)

echo ""
read -r pass fail < "$RESULTS_FILE"
echo "Results: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
