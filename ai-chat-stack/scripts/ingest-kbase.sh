#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}/ai-chat-service"

CONFIG="${1:-dev.config.yaml}"
TARGET="${2:-data/kbase/samples}"

echo "ingest docs from ${TARGET} with config ${CONFIG}"
for path in ${TARGET}; do
  if [ -f "${path}" ]; then
    go run ./chat-server/cmd/ingest-doc --config="${CONFIG}" --file="${path}"
  elif [ -d "${path}" ]; then
    go run ./chat-server/cmd/ingest-doc --config="${CONFIG}" --dir="${path}"
  fi
done
