#!/usr/bin/env bash
# 紹介サイト用の画面集を site/img/ に撮影する（Docker の Playwright を使用）。
#   前提: 開発用の画面(5173)と API が起動し、scripts/demo-seed.sh のデモデータが入っていること。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PW_IMAGE="mcr.microsoft.com/playwright:v1.64.0-noble" # frontend の @playwright/test のバージョンと揃える
mkdir -p "$ROOT/site/img"
docker run --rm --network host --ipc=host --user "$(id -u):$(id -g)" -e HOME=/tmp -e SHOT_BASE="${SHOT_BASE:-http://127.0.0.1:5173}" \
  -v "$ROOT":/repo -w /repo/frontend "$PW_IMAGE" node ../site/tools/screenshots.mjs
