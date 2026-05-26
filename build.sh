#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

(
  cd frontend
  npm ci
  npm run build
)

go build -ldflags "-s -w -X main.version=dev" -o gol ./frontend
