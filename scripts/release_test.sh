#!/usr/bin/env bash
# Tests for release.sh
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

assert_file_exists() {
  local label="$1" path="$2"
  local pass fail
  read -r pass fail < "$RESULTS_FILE"
  if [ -f "$path" ]; then
    echo "  PASS: $label"
    echo "$((pass + 1)) $fail" > "$RESULTS_FILE"
  else
    echo "  FAIL: $label — file '$path' does not exist"
    echo "$pass $((fail + 1))" > "$RESULTS_FILE"
  fi
}

# Setup: temp directory for isolated tests
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR" "$RESULTS_FILE"' EXIT

echo "=== release.sh tests ==="

# Test 1: VERSION file is created with default 0.0.0 when missing
echo "Test: init_version creates VERSION file with 0.0.0"
(
  cd "$TMPDIR"
  rm -f VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  init_version
  assert_file_exists "VERSION file exists" "$TMPDIR/VERSION"
  assert_eq "default version is 0.0.0" "0.0.0" "$(cat VERSION)"
)

# Test 2: bump_patch increments patch version
echo "Test: bump_patch 0.0.0 -> 0.0.1"
(
  cd "$TMPDIR"
  echo "0.0.0" > VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  bump_patch
  assert_eq "patch bumped" "0.0.1" "$(cat VERSION)"
)

# Test 3: bump_minor increments minor, resets patch
echo "Test: bump_minor 0.0.1 -> 0.1.0"
(
  cd "$TMPDIR"
  echo "0.0.1" > VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  bump_minor
  assert_eq "minor bumped" "0.1.0" "$(cat VERSION)"
)

# Test 4: bump_major increments major, resets minor and patch
echo "Test: bump_major 0.1.1 -> 1.0.0"
(
  cd "$TMPDIR"
  echo "0.1.1" > VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  bump_major
  assert_eq "major bumped" "1.0.0" "$(cat VERSION)"
)

# Test 5: do_release calls bump, git tag, git push, gh release
echo "Test: do_release patch creates tag and calls gh release"
(
  cd "$TMPDIR"
  echo "0.0.0" > VERSION

  # Mock git, gh, go — record calls
  MOCK_LOG="$TMPDIR/mock_calls.log"
  > "$MOCK_LOG"
  mock_git() { echo "git $*" >> "$MOCK_LOG"; }
  mock_gh() { echo "gh $*" >> "$MOCK_LOG"; }
  mock_go() { touch "$TMPDIR/dist/awn" "$TMPDIR/dist/awnd"; }

  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=mock_git
  GH_CMD=mock_gh
  GO_CMD=mock_go
  DIST_DIR="$TMPDIR/dist"

  do_release patch

  assert_eq "version bumped to 0.0.1" "0.0.1" "$(cat VERSION)"
  assert_eq "git add VERSION" "git add VERSION" "$(sed -n '1p' "$MOCK_LOG")"
  assert_eq "git commit" "1" "$(grep -c 'git commit' "$MOCK_LOG" || echo 0)"
  assert_eq "git tag created" "git tag v0.0.1" "$(sed -n '3p' "$MOCK_LOG")"
  assert_eq "git push with tag" "git push origin v0.0.1" "$(sed -n '4p' "$MOCK_LOG")"
  assert_eq "gh release created" "gh release create v0.0.1 --title v0.0.1 --generate-notes" "$(sed -n '5p' "$MOCK_LOG")"
)

# Test 6: do_release rejects invalid bump type
echo "Test: do_release rejects invalid bump type"
(
  cd "$TMPDIR"
  echo "0.0.0" > VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=true
  GH_CMD=true
  GO_CMD=true

  output="$(do_release foo 2>&1 || true)"
  assert_eq "error message" "Error: bump type must be patch, minor, or major" "$output"
)

# Test 7: do_release with minor
echo "Test: do_release minor bumps minor version"
(
  cd "$TMPDIR"
  echo "0.0.5" > VERSION
  mkdir -p "$TMPDIR/dist2"
  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=true
  GH_CMD=true
  GO_CMD="mock_go_noop"
  DIST_DIR="$TMPDIR/dist2"
  mock_go_noop() { touch "$TMPDIR/dist2/awn" "$TMPDIR/dist2/awnd"; }
  do_release minor
  assert_eq "version is 0.1.0" "0.1.0" "$(cat VERSION)"
)

# Test 8: build_all creates tarballs for each platform
echo "Test: build_all creates platform tarballs"
(
  cd "$TMPDIR"
  echo "0.1.0" > VERSION
  mkdir -p dist

  # Mock go build to create fake binaries
  mock_go() { touch "$TMPDIR/dist/awn" "$TMPDIR/dist/awnd"; }

  source "$SCRIPT_DIR/release.sh" --source-only
  GO_CMD=mock_go

  build_all "$TMPDIR/dist"

  assert_file_exists "linux-amd64 tarball" "$TMPDIR/dist/awn-linux-amd64.tar.gz"
  assert_file_exists "linux-arm64 tarball" "$TMPDIR/dist/awn-linux-arm64.tar.gz"
  assert_file_exists "darwin-amd64 tarball" "$TMPDIR/dist/awn-darwin-amd64.tar.gz"
  assert_file_exists "darwin-arm64 tarball" "$TMPDIR/dist/awn-darwin-arm64.tar.gz"
)

# Test 9: do_release uploads assets
echo "Test: do_release uploads tarballs to release"
(
  cd "$TMPDIR"
  echo "0.0.0" > VERSION
  mkdir -p dist

  MOCK_LOG="$TMPDIR/mock_calls2.log"
  > "$MOCK_LOG"
  mock_git() { echo "git $*" >> "$MOCK_LOG"; }
  mock_gh() { echo "gh $*" >> "$MOCK_LOG"; }
  mock_go() { touch "$TMPDIR/dist/awn" "$TMPDIR/dist/awnd"; }

  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=mock_git
  GH_CMD=mock_gh
  GO_CMD=mock_go
  DIST_DIR="$TMPDIR/dist"

  do_release patch

  # Check that gh release upload was called
  assert_eq "assets uploaded" "1" "$(grep -c 'gh release upload' "$MOCK_LOG" || echo 0)"
)

echo ""
read -r pass fail < "$RESULTS_FILE"
echo "Results: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
