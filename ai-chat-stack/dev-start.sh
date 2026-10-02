#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=dev-common.sh
source "${SCRIPT_DIR}/dev-common.sh"
resolve_dev_paths

main() {
  echo "[dev] 本地开发环境启动"
  echo "[dev] 日志目录: ${LOG_DIR}"

  warn_kvstore

  require_port_free 50053 "sensitive"
  require_port_free 3002 "tokenizer"
  require_port_free 50057 "vecfluxapi"
  require_port_free 50055 "ai-chat-service"
  require_port_free 7080 "ai-chat-backend"

  start_service sensitive
  start_service tokenizer
  start_service vecfluxapi
  start_service ai-chat-service
  start_service ai-chat-backend

  echo ""
  echo "[dev] 全部服务已启动："
  echo "  sensitive        :50053"
  echo "  tokenizer        :3002"
  echo "  vecfluxapi       :50057"
  echo "  ai-chat-service  :50055"
  echo "  ai-chat-backend  :7080  → http://127.0.0.1:7080"
  echo ""
  echo "  mykvstore        :2000  ← 需自行启动，见 README"
  echo ""
  echo "停止: bash ${STACK_DIR}/dev-stop.sh"
  echo "重启: bash ${STACK_DIR}/dev-restart.sh ai-chat-service"
}

main "$@"
