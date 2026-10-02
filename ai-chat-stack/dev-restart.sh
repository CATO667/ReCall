#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=dev-common.sh
source "${SCRIPT_DIR}/dev-common.sh"
resolve_dev_paths

usage() {
  cat <<EOF
用法: bash dev-restart.sh [服务名]

服务名:
  ai-chat-service   :50055  ← 改 RAG 代码后最常用
  ai-chat-backend   :7080
  tokenizer         :3002
  sensitive         :50053
  all               重启上面 4 个服务

示例:
  bash dev-restart.sh ai-chat-service
  bash dev-restart.sh

日志: ${LOG_DIR}/
EOF
}

main() {
  local target="${1:-all}"
  case "${target}" in
    -h|--help|help)
      usage
      exit 0
      ;;
    all|"")
      echo "[dev] 重启 dev 栈（sensitive / tokenizer / ai-chat-service / ai-chat-backend）"
      for svc in "${ALL_DEV_SERVICES[@]}"; do
        restart_service "${svc}"
      done
      ;;
    sensitive|tokenizer|ai-chat-service|ai-chat-backend)
      restart_service "${target}"
      ;;
    *)
      echo "[dev] 不认识的服务: ${target}" >&2
      usage
      exit 1
      ;;
  esac
  echo "[dev] 完成"
}

main "$@"
