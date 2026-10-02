#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=dev-common.sh
source "${SCRIPT_DIR}/dev-common.sh"
resolve_dev_paths

main() {
  stop_service "ai-chat-backend" 7080
  stop_service "ai-chat-service" 50055
  stop_service "vecfluxapi" 50057
  stop_service "tokenizer" 3002
  stop_service "sensitive" 50053
  echo "[dev] 本地 dev 已停止（mykvstore / Qdrant 不在此脚本管理范围内）"
}

main "$@"
