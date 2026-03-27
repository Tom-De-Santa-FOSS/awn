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

  # Mock git and gh — record calls
  MOCK_LOG="$TMPDIR/mock_calls.log"
  > "$MOCK_LOG"
  mock_git() { echo "git $*" >> "$MOCK_LOG"; }
  mock_gh() { echo "gh $*" >> "$MOCK_LOG"; }

  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=mock_git
  GH_CMD=mock_gh

  do_release patch

  assert_eq "version bumped to 0.0.1" "0.0.1" "$(cat VERSION)"
  assert_eq "git tag created" "git tag v0.0.1" "$(sed -n '1p' "$MOCK_LOG")"
  assert_eq "git push tag" "git push origin v0.0.1" "$(sed -n '2p' "$MOCK_LOG")"
  assert_eq "gh release created" "gh release create v0.0.1 --title v0.0.1 --generate-notes" "$(sed -n '3p' "$MOCK_LOG")"
)

# Test 6: do_release rejects invalid bump type
echo "Test: do_release rejects invalid bump type"
(
  cd "$TMPDIR"
  echo "0.0.0" > VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=true
  GH_CMD=true

  output="$(do_release foo 2>&1 || true)"
  assert_eq "error message" "Error: bump type must be patch, minor, or major" "$output"
)

# Test 7: do_release with minor
echo "Test: do_release minor bumps minor version"
(
  cd "$TMPDIR"
  echo "0.0.5" > VERSION
  source "$SCRIPT_DIR/release.sh" --source-only
  GIT_CMD=true
  GH_CMD=true
  do_release minor
  assert_eq "version is 0.1.0" "0.1.0" "$(cat VERSION)"
)

echo ""
read -r pass fail < "$RESULTS_FILE"
echo "Results: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
