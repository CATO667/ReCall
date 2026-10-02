#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${ROOT}/tiktoken_cache"
TARGET="${CACHE_DIR}/cl100k_base.tiktoken"
CACHE_KEY="9b5ad71b2ce5302211f9c61530b329a4922fc6a4"
CACHE_FILE="${CACHE_DIR}/${CACHE_KEY}"

mkdir -p "${CACHE_DIR}"

install_cache() {
  cp "${TARGET}" "${CACHE_FILE}"
  echo "已写入 tiktoken 缓存: ${CACHE_FILE}"
}

if [[ -f "${CACHE_FILE}" && -s "${CACHE_FILE}" ]]; then
  echo "tiktoken 缓存已就绪: ${CACHE_FILE} ($(du -h "${CACHE_FILE}" | awk '{print $1}'))"
  exit 0
fi

if [[ -f "${TARGET}" && -s "${TARGET}" ]]; then
  install_cache
  exit 0
fi

URLS=(
  "https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken"
  "https://raw.githubusercontent.com/niieani/gpt-tokenizer/main/data/cl100k_base.tiktoken"
  "https://ghproxy.net/https://raw.githubusercontent.com/niieani/gpt-tokenizer/main/data/cl100k_base.tiktoken"
)

for url in "${URLS[@]}"; do
  echo "尝试下载: ${url}"
  if curl -fsSL --max-time 120 -o "${TARGET}.tmp" "${url}"; then
    mv "${TARGET}.tmp" "${TARGET}"
    install_cache
    echo "下载成功: ${TARGET} ($(du -h "${TARGET}" | awk '{print $1}'))"
    exit 0
  fi
  rm -f "${TARGET}.tmp"
done

echo "下载失败，请检查网络或手动放置 cl100k_base.tiktoken 到 ${CACHE_DIR}/ 后重新运行本脚本"
exit 1
