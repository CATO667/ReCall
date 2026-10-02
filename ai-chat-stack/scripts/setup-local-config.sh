#!/usr/bin/env bash
# 从 *.example 生成本地 dev 配置（已存在则跳过）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

copy_if_missing() {
  local example="$1"
  local target="$2"
  if [[ -f "${target}" ]]; then
    echo "[skip] 已存在: ${target#${ROOT}/}"
    return
  fi
  if [[ ! -f "${example}" ]]; then
    echo "[warn] 缺少模板: ${example#${ROOT}/}" >&2
    return
  fi
  cp "${example}" "${target}"
  echo "[ok]   已创建: ${target#${ROOT}/}"
}

echo "生成本地配置文件（不会覆盖已有文件）"
echo ""

copy_if_missing "${ROOT}/ai-chat-service/dev.config.yaml.example" "${ROOT}/ai-chat-service/dev.config.yaml"
copy_if_missing "${ROOT}/ai-chat-service/vecfluxapi.dev.config.yaml.example" "${ROOT}/ai-chat-service/vecfluxapi.dev.config.yaml"
copy_if_missing "${ROOT}/ai-chat-backend/dev.config.yaml.example" "${ROOT}/ai-chat-backend/dev.config.yaml"
copy_if_missing "${ROOT}/keywords-filter/dev.config.yaml.example" "${ROOT}/keywords-filter/dev.config.yaml"
copy_if_missing "${ROOT}/keywords-filter/dev.kw.config.yaml.example" "${ROOT}/keywords-filter/dev.kw.config.yaml"

copy_if_missing "${ROOT}/ai-chat-stack/configs/ai-chat-service.yaml.example" "${ROOT}/ai-chat-stack/configs/ai-chat-service.yaml"
copy_if_missing "${ROOT}/ai-chat-stack/configs/ai-chat-backend.yaml.example" "${ROOT}/ai-chat-stack/configs/ai-chat-backend.yaml"
copy_if_missing "${ROOT}/ai-chat-stack/configs/sensitive.yaml.example" "${ROOT}/ai-chat-stack/configs/sensitive.yaml"
copy_if_missing "${ROOT}/ai-chat-stack/configs/keywords.yaml.example" "${ROOT}/ai-chat-stack/configs/keywords.yaml"

echo ""
echo "下一步：编辑以下两个文件中的 LLM API Key（其余 token / 地址模板已对齐）："
echo ""
echo "  ai-chat-service/dev.config.yaml"
echo "  ai-chat-service/vecfluxapi.dev.config.yaml"
echo ""
echo "  需替换："
echo "    YOUR_DEEPSEEK_API_KEY      → chat.api_key"
echo ""
echo "  Embedding 默认走本地 Ollama（bge-m3 @ http://127.0.0.1:11434/v1），先执行："
echo "    bash ai-chat-stack/scripts/ollama.sh start"
echo "    bash ai-chat-stack/scripts/ollama.sh pull"
echo ""
echo "  可选调整："
echo "    kvstore.write_host / read_host（单机 dev 可都用 127.0.0.1；主从读写分离见 README）"
echo "    vector.provider 固定为 kvstore（vec-fluxkv HNSW）"
echo "    embedder 改回远程 SiliconFlow 见 dev.config.yaml.example 注释"
echo ""
echo "accessToken 已在 dev.config 与 vecfluxapi.dev.config 中对齐为 dev-local-access-token，一般无需改。"
echo ""
echo "完成后："
echo "  cd ai-chat-stack && bash dev-start.sh"
