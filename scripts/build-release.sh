#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"
NPM_BIN="${NPM_BIN:-npm}"
GO_TAGS="${GO_TAGS:-with_quic with_utls}"
OUTPUT="${OUTPUT:-$ROOT_DIR/bin/jnmproxy}"
GOCACHE="${GOCACHE:-/tmp/jnm-go-cache}"

echo "==> build web dashboard"
cd "$ROOT_DIR/web"
if [ ! -d node_modules ]; then
  "$NPM_BIN" ci
fi
"$NPM_BIN" run build

echo "==> sync embedded web assets"
mkdir -p "$ROOT_DIR/internal/webui/dist"
find "$ROOT_DIR/internal/webui/dist" -depth -mindepth 1 -delete
cp -R "$ROOT_DIR/web/dist/." "$ROOT_DIR/internal/webui/dist/"

echo "==> run go tests"
cd "$ROOT_DIR"
GOCACHE="$GOCACHE" "$GO_BIN" test -tags "$GO_TAGS" ./...

echo "==> build go binary"
mkdir -p "$(dirname "$OUTPUT")"
GOCACHE="$GOCACHE" "$GO_BIN" build -tags "$GO_TAGS" -o "$OUTPUT" ./cmd/jnmproxy

echo "release binary: $OUTPUT"
