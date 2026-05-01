#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"
mkdir -p dist

GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "dist/wowschat-translator-linux-amd64" ./cmd/wowschat-translator/

echo "Build succeeded: dist/wowschat-translator-linux-amd64"
