#!/usr/bin/env sh
set -eu

mkdir -p dist
: "${GOOS:=$(go env GOOS)}"
: "${GOARCH:=$(go env GOARCH)}"
output="dist/shim-${GOOS}-${GOARCH}"
if [ "$GOOS" = "windows" ]; then
  output="${output}.exe"
fi
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/shim
printf '%s\n' "$output"
