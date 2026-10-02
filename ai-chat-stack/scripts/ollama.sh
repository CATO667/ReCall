#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="ai-chat-ollama"
IMAGE="ollama/ollama:latest"
VOLUME="ai-chat-ollama-data"
PORT="11434"
MODEL="bge-m3"

usage() {
  cat <<EOF
用法: $0 {start|stop|restart|status|pull|test|logs}

  start   启动 Ollama 容器
  stop    停止并删除容器（模型数据保留在 Docker volume）
  restart 重启容器
  status  查看容器与已拉取模型
  pull    拉取/更新 ${MODEL} embedding 模型
  test    发送一条测试请求，验证 1024 维向量
  logs    查看容器日志
EOF
}

ensure_running() {
  if ! docker ps --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
    echo "容器 ${CONTAINER_NAME} 未运行，请先执行: $0 start" >&2
    exit 1
  fi
}

cmd_start() {
  if docker ps -a --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
    docker start "${CONTAINER_NAME}" >/dev/null
    echo "已启动已有容器 ${CONTAINER_NAME}"
    return
  fi
  docker run -d \
    --name "${CONTAINER_NAME}" \
    --restart unless-stopped \
    -p "${PORT}:11434" \
    -v "${VOLUME}:/root/.ollama" \
    -e OLLAMA_HOST=0.0.0.0:11434 \
    "${IMAGE}" >/dev/null
  echo "已创建并启动 ${CONTAINER_NAME}，端口 ${PORT}"
}

cmd_stop() {
  if docker ps -a --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null
    echo "已停止并删除 ${CONTAINER_NAME}"
  else
    echo "容器 ${CONTAINER_NAME} 不存在"
  fi
}

cmd_status() {
  docker ps -a --filter "name=${CONTAINER_NAME}" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
  if docker ps --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
    echo
    docker exec "${CONTAINER_NAME}" ollama list
  fi
}

cmd_pull() {
  ensure_running
  docker exec "${CONTAINER_NAME}" ollama pull "${MODEL}"
}

cmd_test() {
  ensure_running
  curl -sf "http://127.0.0.1:${PORT}/v1/embeddings" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"${MODEL}\",\"input\":\"测试向量化\",\"encoding_format\":\"float\"}" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); e=d['data'][0]['embedding']; print(f'ok dim={len(e)} first3={e[:3]}')"
}

cmd_logs() {
  docker logs -f "${CONTAINER_NAME}"
}

main() {
  local cmd="${1:-}"
  case "${cmd}" in
    start) cmd_start ;;
    stop) cmd_stop ;;
    restart) cmd_stop; cmd_start ;;
    status) cmd_status ;;
    pull) cmd_pull ;;
    test) cmd_test ;;
    logs) cmd_logs ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
