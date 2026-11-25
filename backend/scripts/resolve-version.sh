#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BACKEND_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(CDPATH= cd -- "$BACKEND_DIR/.." && pwd)"
VERSION_FILE="$BACKEND_DIR/cmd/server/VERSION"

# Prefer the latest release tag reachable from HEAD so
# source builds with custom commits on top of vX.Y.Z still resolve to X.Y.Z.
if command -v git >/dev/null 2>&1; then
  TAG="$(
    git -C "$REPO_DIR" describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || \
    git -C "$REPO_DIR" describe --tags --abbrev=0 --match '[0-9]*' 2>/dev/null || \
    true
  )"
  if [ -n "$TAG" ]; then
    printf '%s\n' "${TAG#v}"
    exit 0
  fi
fi

printf '%s\n' "$(tr -d '\r\n' < "$VERSION_FILE")"
