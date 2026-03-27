#!/usr/bin/env bash
# release.sh — version bumping and GitHub release creation for awn
set -euo pipefail

init_version() {
  [ -f VERSION ] || echo "0.0.0" > VERSION
}

read_version() {
  cat VERSION
}

bump_patch() {
  local ver
  ver="$(read_version)"
  local major minor patch
  IFS='.' read -r major minor patch <<< "$ver"
  echo "$major.$minor.$((patch + 1))" > VERSION
}

bump_minor() {
  local ver
  ver="$(read_version)"
  local major minor patch
  IFS='.' read -r major minor patch <<< "$ver"
  echo "$major.$((minor + 1)).0" > VERSION
}

bump_major() {
  local ver
  ver="$(read_version)"
  local major minor patch
  IFS='.' read -r major minor patch <<< "$ver"
  echo "$((major + 1)).0.0" > VERSION
}

# Allow sourcing without executing main
if [ "${1:-}" = "--source-only" ]; then
  return 0 2>/dev/null || true
fi
