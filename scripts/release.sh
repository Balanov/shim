#!/usr/bin/env bash
set -euo pipefail

rm -rf dist
mkdir -p dist

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

for target in "${targets[@]}"; do
  read -r goos goarch <<< "$target"
  name="shim_${goos}_${goarch}"
  binary="dist/${name}"
  if [[ "$goos" == "windows" ]]; then
    binary+=".exe"
  fi
  echo "Building $name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="-s -w" -o "$binary" ./cmd/shim
  if [[ "$goos" == "windows" ]]; then
    if command -v zip >/dev/null 2>&1; then
      zip -j -q "dist/${name}.zip" "$binary"
    else
      python3 -c 'import os,sys,zipfile; z=zipfile.ZipFile(sys.argv[2],"w",zipfile.ZIP_DEFLATED); z.write(sys.argv[1],os.path.basename(sys.argv[1])); z.close()' "$binary" "dist/${name}.zip"
    fi
  else
    tar -C dist -czf "dist/${name}.tar.gz" "$name"
  fi
  rm "$binary"
done

(
  cd dist
  sha256sum ./*.tar.gz ./*.zip > checksums.txt
)
