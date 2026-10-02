#!/usr/bin/env bash
# 从 configs/docker/*.tpl 渲染 runtime/docker/configs（读取 .env）
set -euo pipefail

STACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${STACK_DIR}/.env"
TEMPLATE_DIR="${STACK_DIR}/configs/docker"
OUT_DIR="${STACK_DIR}/runtime/docker/configs"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "缺少 ${ENV_FILE}，请先: cp .env.example .env" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
source "${ENV_FILE}"
set +a

: "${DEEPSEEK_API_KEY:?请在 .env 中设置 DEEPSEEK_API_KEY}"
: "${ACCESS_TOKEN:=dev-local-access-token}"
: "${OLLAMA_MODEL:=bge-m3}"
: "${KV_WRITE_HOST:=host.docker.internal}"
: "${KV_READ_HOST:=host.docker.internal}"
: "${KV_PORT:=2000}"
: "${KV_READ_PORT:=2000}"

mkdir -p "${OUT_DIR}"

render_tpl() {
  local tpl="$1"
  local out="$2"
  envsubst '${DEEPSEEK_API_KEY} ${ACCESS_TOKEN} ${OLLAMA_MODEL} ${KV_WRITE_HOST} ${KV_READ_HOST} ${KV_PORT} ${KV_READ_PORT}' \
    < "${tpl}" > "${out}"
  echo "[ok] ${out#${STACK_DIR}/}"
}

render_tpl "${TEMPLATE_DIR}/ai-chat-service.yaml.tpl" "${OUT_DIR}/ai-chat-service.yaml"
render_tpl "${TEMPLATE_DIR}/vecfluxapi.yaml.tpl" "${OUT_DIR}/vecfluxapi.yaml"
render_tpl "${TEMPLATE_DIR}/ai-chat-backend.yaml.tpl" "${OUT_DIR}/ai-chat-backend.yaml"

for f in sensitive.yaml keywords.yaml; do
  cp "${TEMPLATE_DIR}/${f}" "${OUT_DIR}/${f}"
  echo "[ok] runtime/docker/configs/${f}"
done

echo ""
echo "配置已渲染到 runtime/docker/configs/"
