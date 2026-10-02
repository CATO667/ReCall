#!/usr/bin/env bash
# shellcheck disable=SC2034
# 供 dev-start / dev-stop / dev-restart 共用

resolve_dev_paths() {
  STACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  CHAT_ROOT="$(cd "${STACK_DIR}/.." && pwd)"
  KEYWORDS_DIR="${CHAT_ROOT}/keywords-filter"
  TOKENIZER_DIR="${CHAT_ROOT}/tokenizer"
  SERVICE_DIR="${CHAT_ROOT}/ai-chat-service"
  BACKEND_DIR="${CHAT_ROOT}/ai-chat-backend"
  RUNTIME_DIR="${STACK_DIR}/runtime/dev"
  LOG_DIR="${RUNTIME_DIR}/logs"
  PID_DIR="${RUNTIME_DIR}/pids"
  mkdir -p "${LOG_DIR}" "${PID_DIR}"
}

port_listening() {
  local port="$1"
  ss -tln | grep -q ":${port} "
}

wait_for_port() {
  local port="$1"
  local name="$2"
  local timeout="${3:-60}"
  local i=0
  while ! port_listening "${port}"; do
    sleep 1
    i=$((i + 1))
    if [[ "${i}" -ge "${timeout}" ]]; then
      echo "[dev] ERROR: ${name} 未在 ${timeout}s 内监听 :${port}" >&2
      echo "[dev]        日志: ${LOG_DIR}/${name}.log" >&2
      return 1
    fi
  done
  echo "[dev] ${name} 已就绪 (:${port})"
}

require_port_free() {
  local port="$1"
  local name="$2"
  if port_listening "${port}"; then
    echo "[dev] ERROR: 端口 ${port} 已被占用，无法启动 ${name}" >&2
    echo "[dev]        请先执行: bash ${STACK_DIR}/dev-stop.sh 或 dev-restart.sh ${name}" >&2
    exit 1
  fi
}

warn_kvstore() {
  if port_listening 2000; then
    echo "[dev] mykvstore 已就绪 (:2000)"
    return
  fi
  echo "[dev] WARN: mykvstore 未监听 :2000，请自行启动并配置 dev.config.yaml 中 kvstore.write_host / read_host"
}

kill_port() {
  local port="$1"
  if ! port_listening "${port}"; then
    return
  fi
  if command -v fuser >/dev/null 2>&1; then
    fuser -k "${port}/tcp" >/dev/null 2>&1 || true
    sleep 1
  fi
}

stop_service() {
  local name="$1"
  local port="${2:-}"
  local pid_file="${PID_DIR}/${name}.pid"

  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}")"
    if kill -0 "${pid}" 2>/dev/null; then
      echo "[dev] 停止 ${name} (pid=${pid})"
      kill "${pid}" 2>/dev/null || true
      sleep 1
      kill -9 "${pid}" 2>/dev/null || true
    fi
    rm -f "${pid_file}"
  fi

  if [[ -n "${port}" ]]; then
    kill_port "${port}"
  fi
}

start_bg() {
  local name="$1"
  shift
  echo "[dev] 启动 ${name} ..."
  nohup "$@" >> "${LOG_DIR}/${name}.log" 2>&1 &
  echo "$!" > "${PID_DIR}/${name}.pid"
}

ensure_tokenizer_venv() {
  local venv="${TOKENIZER_DIR}/.venv"
  local nuxt_bin="${venv}/bin/nuxt"
  local need_recreate=0
  if [[ ! -d "${venv}" ]]; then
    need_recreate=1
  elif [[ ! -x "${nuxt_bin}" ]]; then
    need_recreate=1
  elif ! "${nuxt_bin}" --help >/dev/null 2>&1; then
    # venv 从其他目录拷贝后，entry script shebang 仍指向旧路径
    need_recreate=1
  elif ! "${venv}/bin/python" -c "import nuxt" >/dev/null 2>&1; then
    need_recreate=1
  fi
  if [[ "${need_recreate}" -eq 1 ]]; then
    echo "[dev] 创建/重建 tokenizer 虚拟环境 ..."
    rm -rf "${venv}"
    python3 -m venv "${venv}"
    echo "[dev] 安装 tokenizer 依赖 ..."
    "${venv}/bin/pip" install -q -r "${TOKENIZER_DIR}/requirements.txt"
  fi
}

start_service() {
  local name="$1"
  case "${name}" in
    sensitive)
      start_bg "sensitive" bash -lc "cd '${KEYWORDS_DIR}' && go run ./filter-server --config=dev.config.yaml --dict=dict.txt"
      wait_for_port 50053 "sensitive"
      ;;
    tokenizer)
      ensure_tokenizer_venv
      bash "${TOKENIZER_DIR}/download_tiktoken_cache.sh"
      start_bg "tokenizer" bash -lc "cd '${TOKENIZER_DIR}' && export TIKTOKEN_CACHE_DIR='${TOKENIZER_DIR}/tiktoken_cache' && '${TOKENIZER_DIR}/.venv/bin/nuxt' --port 3002 --module tokenizer.py --workers 1"
      wait_for_port 3002 "tokenizer"
      ;;
    vecfluxapi)
      start_bg "vecfluxapi" bash -lc "cd '${SERVICE_DIR}' && go run ./cmd/vecfluxapi --config=vecfluxapi.dev.config.yaml"
      wait_for_port 50057 "vecfluxapi"
      ;;
    ai-chat-service)
      start_bg "ai-chat-service" bash -lc "cd '${SERVICE_DIR}' && go run ./chat-server --config=dev.config.yaml"
      wait_for_port 50055 "ai-chat-service"
      ;;
    ai-chat-backend)
      start_bg "ai-chat-backend" bash -lc "cd '${BACKEND_DIR}' && go run cmd/*.go --config=dev.config.yaml"
      wait_for_port 7080 "ai-chat-backend"
      ;;
    *)
      echo "[dev] 未知服务: ${name}" >&2
      return 1
      ;;
  esac
}

restart_service() {
  local name="$1"
  local port=""
  case "${name}" in
    sensitive) port=50053 ;;
    tokenizer) port=3002 ;;
    vecfluxapi) port=50057 ;;
    ai-chat-service) port=50055 ;;
    ai-chat-backend) port=7080 ;;
  esac
  stop_service "${name}" "${port}"
  start_service "${name}"
}

ALL_DEV_SERVICES=(sensitive tokenizer vecfluxapi ai-chat-service ai-chat-backend)
