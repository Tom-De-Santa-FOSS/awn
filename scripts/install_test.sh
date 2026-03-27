#!/usr/bin/env bash
# Tests for install.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test_helpers.sh"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR" "$RESULTS_FILE"' EXIT

# Enable testing mode so CURL_CMD can be overridden
export AWN_TESTING=1

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
  assert_eq "downloads from repo" "1" "$(grep -c 'raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/' "$MOCK_LOG" || echo 0)"
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

# Test 8: detect_os exits on unsupported OS
echo "Test: detect_os should exit with error when OS is unsupported"
(
  source "$SCRIPT_DIR/../install.sh" --source-only
  errfile="$TMPDIR/detect_os_err"
  rc=0
  (MOCK_UNAME_S=FreeBSD detect_os 2>"$errfile") || rc=$?
  assert_eq "exit code is non-zero" "1" "$rc"
  assert_eq "error mentions unsupported" "1" "$(grep -c 'Unsupported OS' "$errfile" || echo 0)"
)

# Test 9: detect_arch exits on unsupported architecture
echo "Test: detect_arch should exit with error when architecture is unsupported"
(
  source "$SCRIPT_DIR/../install.sh" --source-only
  errfile="$TMPDIR/detect_arch_err"
  rc=0
  (MOCK_UNAME_M=mips64 detect_arch 2>"$errfile") || rc=$?
  assert_eq "exit code is non-zero" "1" "$rc"
  assert_eq "error mentions unsupported" "1" "$(grep -c 'Unsupported architecture' "$errfile" || echo 0)"
)

# Test 10: get_latest_version fails on malformed API response
echo "Test: get_latest_version should fail when GitHub API returns no tag_name"
(
  source "$SCRIPT_DIR/../install.sh" --source-only
  mock_curl() { echo '{"message": "rate limited"}'; }
  CURL_CMD=mock_curl

  errfile="$TMPDIR/version_err"
  rc=0
  get_latest_version >"$TMPDIR/version_out" 2>"$errfile" || rc=$?
  assert_eq "exit code is non-zero" "1" "$rc"
  assert_eq "error message shown" "1" "$(grep -c 'Failed to detect' "$errfile" || echo 0)"
)

# Test 11: install_skill fetches from versioned tag, not master
echo "Test: install_skill should fetch SKILL.md from versioned tag"
(
  cd "$TMPDIR"
  SKILL_DIR="$TMPDIR/verskills/awn"
  MOCK_LOG="$TMPDIR/verskill_curl.log"
  > "$MOCK_LOG"

  mock_curl() { echo "curl $*" >> "$MOCK_LOG"; echo "# skill content" > "$2"; }

  source "$SCRIPT_DIR/../install.sh" --source-only
  CURL_CMD=mock_curl
  VERSION="1.2.3"
  install_skill "$SKILL_DIR"

  tag_count="$(grep -c 'v1.2.3' "$MOCK_LOG")" || tag_count=0
  master_count="$(grep -c 'master' "$MOCK_LOG")" || master_count=0
  assert_eq "fetches from version tag" "1" "$tag_count"
  assert_eq "does not fetch from master" "0" "$master_count"
)

# Test 12: awnd is also executable after install
echo "Test: awnd should be executable after do_install"
(
  cd "$TMPDIR"
  mkdir -p "$TMPDIR/fakebuild2"
  echo "#!/bin/sh" > "$TMPDIR/fakebuild2/awn"
  echo "#!/bin/sh" > "$TMPDIR/fakebuild2/awnd"
  chmod +x "$TMPDIR/fakebuild2/awn" "$TMPDIR/fakebuild2/awnd"
  tar czf "$TMPDIR/fake2.tar.gz" -C "$TMPDIR/fakebuild2" awn awnd

  mock_curl() { cp "$TMPDIR/fake2.tar.gz" "$2"; }

  source "$SCRIPT_DIR/../install.sh" --source-only
  CURL_CMD=mock_curl
  INSTALL_DIR="$TMPDIR/testbin2"
  VERSION="0.1.0"

  do_install "linux" "amd64"
  assert_eq "awnd is executable" "1" "$([ -x "$TMPDIR/testbin2/awnd" ] && echo 1 || echo 0)"
)

print_results
