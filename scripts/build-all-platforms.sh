#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"
NPM_BIN="${NPM_BIN:-npm}"
APP_NAME="${APP_NAME:-jnmproxy}"
VERSION="${VERSION:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo dev)}"
RELEASE_DIR="${RELEASE_DIR:-$ROOT_DIR/release/packages}"
GOCACHE="${GOCACHE:-/tmp/jnm-go-cache}"
GO_TAGS="${GO_TAGS:-sqlite_purego with_quic with_utls}"
RUN_WEB_BUILD="${RUN_WEB_BUILD:-1}"
RUN_TESTS="${RUN_TESTS:-1}"
PACKAGE_FORMAT="${PACKAGE_FORMAT:-auto}"

platforms=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

if [ "$RUN_WEB_BUILD" = "1" ]; then
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
fi

echo "==> prepare release directory"
mkdir -p "$RELEASE_DIR"
find "$RELEASE_DIR" -depth -mindepth 1 -delete

cd "$ROOT_DIR"
if [ "$RUN_TESTS" = "1" ]; then
  echo "==> run go tests with tags: $GO_TAGS"
  CGO_ENABLED=0 GOCACHE="$GOCACHE" "$GO_BIN" test -tags "$GO_TAGS" ./...
fi

for platform in "${platforms[@]}"; do
  read -r goos goarch <<<"$platform"
  package_name="$APP_NAME-$VERSION-$goos-$goarch"
  work_dir="$RELEASE_DIR/$package_name"
  binary_name="$APP_NAME"
  archive_name="$package_name.tar.gz"
  if [ "$goos" = "windows" ]; then
    binary_name="$APP_NAME.exe"
    archive_name="$package_name.zip"
  fi
  case "$PACKAGE_FORMAT" in
    auto) ;;
    zip) archive_name="$package_name.zip" ;;
    tar.gz|tgz) archive_name="$package_name.tar.gz" ;;
    *)
      echo "unsupported PACKAGE_FORMAT: $PACKAGE_FORMAT" >&2
      echo "supported values: auto, zip, tar.gz" >&2
      exit 1
      ;;
  esac

  echo "==> build $goos/$goarch"
  mkdir -p "$work_dir"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 GOCACHE="$GOCACHE" "$GO_BIN" build \
    -tags "$GO_TAGS" \
    -trimpath \
    -ldflags "-s -w" \
    -o "$work_dir/$binary_name" \
    ./cmd/jnmproxy

  cp "$ROOT_DIR/config.example.yaml" "$work_dir/config.example.yaml"
  cp "$ROOT_DIR/README.md" "$work_dir/README-project.md"
  cat > "$work_dir/README-run.txt" <<EOF
JnmProxy $goos/$goarch

Build tags: $GO_TAGS
SQLite: sqlite_purego package uses modernc.org/sqlite and does not require CGO.

Quick start:
1. Copy config:
   cp config.example.yaml config.yaml
2. Start:
   ./$binary_name -config config.yaml
3. Open dashboard:
   http://127.0.0.1:8080/

macOS note:
If macOS blocks the binary, run:
   xattr -dr com.apple.quarantine ./$binary_name

Windows note:
Run in PowerShell:
   .\\$binary_name -config config.yaml
EOF

  if [[ "$archive_name" == *.zip ]]; then
    if ! command -v zip >/dev/null 2>&1; then
      echo "zip command is required when PACKAGE_FORMAT=zip or building Windows packages" >&2
      exit 1
    fi
    (cd "$RELEASE_DIR" && zip -qr "$archive_name" "$package_name")
  else
    (cd "$RELEASE_DIR" && tar -czf "$archive_name" "$package_name")
  fi
done

echo "==> write checksums"
(
  cd "$RELEASE_DIR"
  archives=()
  for pattern in *.tar.gz *.zip; do
    if [ -e "$pattern" ]; then
      archives+=("$pattern")
    fi
  done
  if [ "${#archives[@]}" -eq 0 ]; then
    echo "no release archives found" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${archives[@]}" > SHA256SUMS
  else
    shasum -a 256 "${archives[@]}" > SHA256SUMS
  fi
)

echo "release packages:"
find "$RELEASE_DIR" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' -o -name 'SHA256SUMS' \) -print | sort
