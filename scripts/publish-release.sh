#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REMOTE="${REMOTE:-origin}"
BRANCH="${BRANCH:-$(git -C "$ROOT_DIR" branch --show-current)}"
VERSION="${1:-${VERSION:-}}"

usage() {
  cat <<EOF
Usage:
  $0 v0.1.0

Environment:
  REMOTE=origin       Git remote to push.
  BRANCH=<current>    Branch to push before tagging.
  TAG_MESSAGE="..."   Annotated tag message.

This script pushes the branch, creates an annotated version tag, and pushes the tag.
GitHub Actions will build packages and create the GitHub Release after the tag is pushed.
EOF
}

if [ -z "$VERSION" ]; then
  usage
  exit 1
fi

if [[ "$VERSION" != v* ]]; then
  echo "release version must start with v, for example v0.1.0" >&2
  exit 1
fi

if [ -z "$BRANCH" ]; then
  echo "could not detect current branch; set BRANCH explicitly" >&2
  exit 1
fi

cd "$ROOT_DIR"

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "git remote '$REMOTE' does not exist" >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "working tree is not clean; commit or stash changes before publishing" >&2
  git status --short
  exit 1
fi

echo "==> fetch tags from $REMOTE"
git fetch "$REMOTE" --tags

if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null; then
  echo "local tag $VERSION already exists" >&2
  exit 1
fi

if git ls-remote --exit-code --tags "$REMOTE" "refs/tags/$VERSION" >/dev/null 2>&1; then
  echo "remote tag $VERSION already exists on $REMOTE" >&2
  exit 1
fi

echo "==> push branch $BRANCH to $REMOTE"
git push "$REMOTE" "$BRANCH"

echo "==> create tag $VERSION"
git tag -a "$VERSION" -m "${TAG_MESSAGE:-JnmProxy $VERSION}"

echo "==> push tag $VERSION to $REMOTE"
git push "$REMOTE" "$VERSION"

echo "release workflow triggered for $VERSION"
