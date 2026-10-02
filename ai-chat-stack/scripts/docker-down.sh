#!/usr/bin/env bash
set -euo pipefail

STACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${STACK_DIR}"

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose -f docker-compose.yml)
else
  COMPOSE=(docker-compose -f docker-compose.yml)
fi

"${COMPOSE[@]}" down
echo "[docker-down] 应用容器已停止（宿主机 VecFluxkv 需手动 Ctrl+C）"
