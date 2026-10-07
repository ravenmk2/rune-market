#!/usr/bin/env bash
# Local multi-platform build (design §14): dist placeholder → version
# injection → six platform archives into dist/. Requires Git Bash on Windows.
set -euo pipefail
cd "$(dirname "$0")/.."

# frontend dist placeholder so //go:embed works without a real web build
mkdir -p web/dist
touch web/dist/index.html

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
VERSION=${VERSION#v}
LDFLAGS="-s -w -X main.version=${VERSION}"

mkdir -p dist
for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${platform%/*}
  arch=${platform#*/}
  out="dist/runemarket_${VERSION}_${os}_${arch}"
  if [ "$os" = windows ]; then
    out="${out}.exe"
  fi
  echo "building ${out}"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/runemarket
done

echo "done: version=${VERSION}"
