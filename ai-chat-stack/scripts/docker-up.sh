#!/usr/bin/env bash
# 先在本机手动启动 VecFluxkv（:2000），再执行本脚本 build/up 应用栈
set -euo pipefail

STACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-default}"

cd "${STACK_DIR}"

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "已创建 .env，请编辑 DEEPSEEK_API_KEY 后重新运行。" >&2
  exit 1
fi

# shellcheck disable=SC1091
source .env

if [[ "${DEEPSEEK_API_KEY:-}" == "YOUR_DEEPSEEK_API_KEY" || -z "${DEEPSEEK_API_KEY:-}" ]]; then
  echo "请在 .env 中设置有效的 DEEPSEEK_API_KEY" >&2
  exit 1
fi

KV_PORT="${KV_PORT:-2000}"
KV_READ_PORT="${KV_READ_PORT:-2000}"

# 旧配置 vecfluxkv 服务名已移除
if [[ "${KV_WRITE_HOST:-}" == "vecfluxkv" ]]; then
  KV_WRITE_HOST=host.docker.internal
fi

case "${MODE}" in
  default)
    export KV_WRITE_HOST="${KV_WRITE_HOST:-host.docker.internal}"
    export KV_READ_HOST="${KV_READ_HOST:-host.docker.internal}"
    if [[ "${KV_READ_HOST}" == "vecfluxkv" ]]; then
      export KV_READ_HOST=host.docker.internal
    fi
    KV_CHECK_HOST="127.0.0.1"
    ;;
  replication)
    export KV_WRITE_HOST="${KV_WRITE_HOST:-host.docker.internal}"
    if [[ -z "${KV_READ_HOST:-}" || "${KV_READ_HOST}" == "vecfluxkv" ]]; then
      echo "replication 模式请在 .env 设置 KV_READ_HOST 为从库 IP" >&2
      exit 1
    fi
    KV_CHECK_HOST="127.0.0.1"
    ;;
  *)
    echo "未知模式: ${MODE}（可用: default | replication）" >&2
    exit 1
    ;;
esac

kv_port_open() {
  local host="$1" port="$2"
  timeout 1 bash -c "echo > /dev/tcp/${host}/${port}" 2>/dev/null
}

echo "[docker-up] 检查 VecFluxkv 主库 ${KV_CHECK_HOST}:${KV_PORT} ..."
if ! kv_port_open "${KV_CHECK_HOST}" "${KV_PORT}"; then
  cat >&2 <<EOF
VecFluxkv 未在宿主机 :${KV_PORT} 监听。

请先在另一个终端编译并启动主库（VecFluxkv 仓库根目录）：
  cmake -S . -B build && cmake --build build
  ./bin/kvstore -c config/kv.conf

replication 从库见 ai-chat-stack/configs/vecfluxkv/kv-slave-remote.conf.example
EOF
  exit 1
fi

if [[ "${MODE}" == "replication" ]]; then
  echo "[docker-up] replication：写 ${KV_WRITE_HOST}:${KV_PORT}，读 ${KV_READ_HOST}:${KV_READ_PORT}"
fi

bash "${STACK_DIR}/scripts/render-docker-config.sh"

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose -f docker-compose.yml)
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE=(docker-compose -f docker-compose.yml)
else
  echo "未找到 Docker Compose。Ubuntu: sudo apt install docker-compose-v2" >&2
  exit 1
fi

if docker buildx version >/dev/null 2>&1; then
  export DOCKER_BUILDKIT=1
  export COMPOSE_DOCKER_CLI_BUILD=1
else
  unset DOCKER_BUILDKIT COMPOSE_DOCKER_CLI_BUILD
fi

echo "[docker-up] build 应用镜像（不含 VecFluxkv）..."
"${COMPOSE[@]}" build

echo "[docker-up] 启动容器..."
"${COMPOSE[@]}" up -d

if [[ "${OLLAMA_PULL_ON_START:-1}" == "1" ]]; then
  echo "[docker-up] 等待 Ollama 并拉取 ${OLLAMA_MODEL:-bge-m3} ..."
  for _ in $(seq 1 60); do
    if "${COMPOSE[@]}" exec -T ollama ollama list >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  "${COMPOSE[@]}" exec -T ollama ollama pull "${OLLAMA_MODEL:-bge-m3}" || true
fi

echo ""
echo "栈已启动。HTTP: http://localhost:7080"
echo "VecFluxkv 由宿主机进程提供（不在 Docker 内）。"
echo "查看: ${COMPOSE[*]} ps"
echo "停止应用: bash scripts/docker-down.sh"
