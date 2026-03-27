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
  source "$SCRIPT_DIR/../install.sh" --source-only
  os="$(MOCK_UNAME_S=Linux detect_os)"
  assert_eq "Linux -> linux" "linux" "$os"

  os="$(MOCK_UNAME_S=Darwin detect_os)"
  assert_eq "Darwin -> darwin" "darwin" "$os"
)

# Test 2: detect_arch returns normalized arch
echo "Test: detect_arch returns normalized arch name"
(
  source "$SCRIPT_DIR/../install.sh" --source-only
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
  source "$SCRIPT_DIR/../install.sh" --source-only
  REPO="Tom-De-Santa-FOSS/awn"
  url="$(build_download_url "0.1.0" "linux" "amd64")"
  assert_eq "URL correct" "https://github.com/Tom-De-Santa-FOSS/awn/releases/download/v0.1.0/awn-linux-amd64.tar.gz" "$url"
)

# Test 4: install_dir defaults to ~/.local/bin
echo "Test: default install directory"
(
  source "$SCRIPT_DIR/../install.sh" --source-only
  assert_eq "default install dir" "$HOME/.local/bin" "$INSTALL_DIR"
)

# Test 5: do_install downloads, extracts, and places binaries
echo "Test: do_install flow with mocked curl and tar"
(
  cd "$TMPDIR"
  MOCK_LOG="$TMPDIR/mock_calls.log"
  > "$MOCK_LOG"

  # Create fake tarball with fake binaries
  mkdir -p "$TMPDIR/fakebuild"
  echo "#!/bin/sh" > "$TMPDIR/fakebuild/awn"
  echo "#!/bin/sh" > "$TMPDIR/fakebuild/awnd"
  chmod +x "$TMPDIR/fakebuild/awn" "$TMPDIR/fakebuild/awnd"
  tar czf "$TMPDIR/fake.tar.gz" -C "$TMPDIR/fakebuild" awn awnd

  # Mock curl to copy our fake tarball
  mock_curl() { echo "curl $*" >> "$MOCK_LOG"; cp "$TMPDIR/fake.tar.gz" "$2"; }

  source "$SCRIPT_DIR/../install.sh" --source-only
  CURL_CMD=mock_curl
  INSTALL_DIR="$TMPDIR/testbin"
  VERSION="0.1.0"

  do_install "linux" "amd64"

  assert_eq "awn binary installed" "1" "$([ -f "$TMPDIR/testbin/awn" ] && echo 1 || echo 0)"
  assert_eq "awnd binary installed" "1" "$([ -f "$TMPDIR/testbin/awnd" ] && echo 1 || echo 0)"
  assert_eq "awn is executable" "1" "$([ -x "$TMPDIR/testbin/awn" ] && echo 1 || echo 0)"
)

# Test 6: install_skill downloads SKILL.md to target dir
echo "Test: install_skill downloads SKILL.md"
(
  cd "$TMPDIR"
  SKILL_DIR="$TMPDIR/fakeskills/awn"
  MOCK_LOG="$TMPDIR/skill_curl.log"
  > "$MOCK_LOG"

  mock_curl() { echo "curl $*" >> "$MOCK_LOG"; echo "# skill content" > "$2"; }

  source "$SCRIPT_DIR/../install.sh" --source-only
  CURL_CMD=mock_curl
  install_skill "$SKILL_DIR"

  assert_eq "SKILL.md exists" "1" "$([ -f "$SKILL_DIR/SKILL.md" ] && echo 1 || echo 0)"
  assert_eq "downloads from repo" "1" "$(grep -c 'raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/.claude/skills/awn/SKILL.md' "$MOCK_LOG" || echo 0)"
)

# Test 7: get_latest_version fetches version from GitHub API
echo "Test: get_latest_version parses tag from GitHub API"
(
  source "$SCRIPT_DIR/../install.sh" --source-only
  mock_curl() { echo '{"tag_name": "v0.2.0"}'; }
  CURL_CMD=mock_curl

  ver="$(get_latest_version)"
  assert_eq "latest version parsed" "0.2.0" "$ver"
)

echo ""
read -r pass fail < "$RESULTS_FILE"
echo "Results: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
