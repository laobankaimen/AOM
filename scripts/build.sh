#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Run from the repository root. Dependencies must already be installed.
npm run test:unit
npm run build
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/aom-go-cache}"
go test ./...
for target in linux windows; do
  output="release/aom-${target}-amd64"
  mkdir -p "$output/examples" "$output/docs"
  binary="aom"
  if [ "$target" = windows ]; then binary="aom.exe"; fi
  CGO_ENABLED=0 GOOS="$target" GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$output/$binary" ./cmd/aom
done
node scripts/release-files.mjs
node scripts/archive.mjs
printf 'Release directories are ready under release/.\n'
