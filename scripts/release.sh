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

GIT_CMD="${GIT_CMD:-git}"
GH_CMD="${GH_CMD:-gh}"

do_release() {
  local bump_type="${1:?Usage: release.sh <patch|minor|major>}"

  case "$bump_type" in
    patch) bump_patch ;;
    minor) bump_minor ;;
    major) bump_major ;;
    *) echo "Error: bump type must be patch, minor, or major" >&2; return 1 ;;
  esac

  local ver
  ver="$(read_version)"
  local tag="v$ver"

  $GIT_CMD tag "$tag"
  $GIT_CMD push origin "$tag"
  $GH_CMD release create "$tag" --title "$tag" --generate-notes
}

# Allow sourcing without executing main
if [ "${1:-}" = "--source-only" ]; then
  return 0 2>/dev/null || true
fi

# Main entrypoint
init_version
do_release "${1:?Usage: scripts/release.sh <patch|minor|major>}"
