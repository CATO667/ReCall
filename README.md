# ReCallRagService

基于 **vec-fluxdb** 的 RAG 对话服务：历史 QA 缓存 + 语义检索 + rerank + LLM；命中缓存时跳过 LLM，节省 Token。

存储与向量统一走 **vec-fluxdb**（正文 / meta / HNSW），不再依赖 MySQL 或其它向量库。

---

## 架构

```text
浏览器 / 客户端
      │  HTTP/SSE
      ▼
ai-chat-backend (:7080)
      │  gRPC
      ▼
ai-chat-service (:50055)
      ├─ 敏感词 gRPC ──► keywords-filter (:50053)
      └─ 对话编排 gRPC ──► vecfluxapi (:50057)
                              ├─ Embed  ──► 本地 Ollama bge-m3 (:11434)
                              ├─ KV     ──► vec-fluxkv (:2000)   QA 正文 / meta
                              ├─ 向量   ──► vec-fluxkv V*         HNSW 索引
                              ├─ rerank ──► 本地融合打分
                              └─ LLM    ──► DeepSeek 等 OpenAI 兼容 API
```

**一次提问（简化）**

1. 敏感词检查  
2. **精确 QA 命中**（相同问题）→ 读 KV 直返，**跳过 Embed / LLM**  
3. 否则 Embed + VSEARCH + rerank → 分数 ≥ 0.65 → **语义缓存直返**  
4. 否则拼 RAG 上下文 → LLM 流式生成  
5. 异步写入 QA（KV + 向量索引）

默认阈值：语义硬缓存 **0.65**，软 RAG **0.4**。

---

## 功能

| 能力 | 说明 |
|------|------|
| 精确 QA 缓存 | 相同问题 KV 直返 |
| 语义 QA 缓存 | 相似问题向量检索 + rerank 后直返 |
| 软 RAG | 中等相似度历史 QA 拼 Prompt |
| 知识库检索 | 文档切分入库 + 混合检索，数据落在 vec-fluxkv |
| 敏感词过滤 | 独立 gRPC 服务 |
| 流式对话 | LLM token 级流式；缓存命中整包分片返回 |

---

## 快速开始

VecFluxkv **不编进 Docker**，先在宿主机启动，再拉起应用栈。

### 1. 准备环境变量

```bash
cd ai-chat-stack
cp .env.example .env          # 填入 DEEPSEEK_API_KEY
```

`.env` 里主要项：

| 变量 | 说明 |
|------|------|
| `DEEPSEEK_API_KEY` | LLM API Key（必填） |
| `ACCESS_TOKEN` | 各 gRPC 服务共用 token（默认即可） |
| `KV_WRITE_HOST` / `KV_READ_HOST` | 容器访问宿主机 KV，默认 `host.docker.internal` |
| `OLLAMA_MODEL` | Embedding 模型，默认 `bge-m3` |

### 2. 启动 VecFluxkv（宿主机）

另开终端，在 **VecFluxkv** 仓库根目录编译并启动主库（监听 `:2000`）：

```bash
cd ../VecFluxkv          # 与 RecallragService 同级时
cmake -S . -B build && cmake --build build
./bin/kvstore -c config/kv.conf
```

详情见 VecFluxdb项目 README。

### 3. 启动应用栈

```bash
cd ai-chat-stack
bash scripts/docker-up.sh
```

脚本会检查宿主机 `:2000` → 渲染配置 → build/up 容器 →（可选）拉取 `bge-m3`。

打开：**http://localhost:7080**

首次 pull **bge-m3** 约 1.2 GB（仅一次，模型在 Docker volume 中）。

---

## 常用命令

在 `ai-chat-stack/` 下执行：

| 操作 | 命令 |
|------|------|
| 启动应用栈 | `bash scripts/docker-up.sh` |
| 停止应用栈 | `bash scripts/docker-down.sh`（VecFluxkv 需手动 Ctrl+C） |
| 查看状态 | `docker compose -f docker-compose.yml ps` |
| 清理 build 缓存 | `bash scripts/docker-clean.sh` |
| 主从读写分离 | `bash scripts/docker-up.sh replication`（`.env` 设 `KV_READ_HOST=从库IP`） |

从库示例配置：`configs/vecfluxkv/kv-slave-remote.conf.example`。

| 服务 | 端口 |
|------|------|
| ai-chat-backend | 7080 |
| ai-chat-service | 50055 |
| vecfluxapi | 50057 |
| sensitive | 50053 |
| tokenizer | 3002 |
| vec-fluxkv（宿主机） | 2000 |
| Ollama（Embed） | 11434 |

---

## 向量与存储

仅支持 **`vector.provider: kvstore`**：

- QA / 知识库正文与 meta → vec-fluxkv KV
- 向量 → vec-fluxkv 内置 HNSW（`VCREATE` / `VINSERT` / `VSEARCH`）

---

## 知识库入库

应用栈与 VecFluxkv 就绪后：

```bash
bash ai-chat-stack/scripts/ingest-kbase.sh

cd ai-chat-service
go run ./chat-server/cmd/ingest-doc --config=dev.config.yaml --file=path/to/doc.md
go run ./chat-server/cmd/ingest-doc --config=dev.config.yaml --list
```

入库前需有本地 `ai-chat-service/dev.config.yaml`（可从 example 复制，并保证 `kvstore` 指向可达的 VecFluxkv）。

---

## 本地 Go 开发（可选）

改后端代码、不想每次 Docker build 时，可用 Go 直跑（仍须宿主机 VecFluxkv + Ollama）：

```bash
bash ai-chat-stack/scripts/setup-local-config.sh
# 编辑 ai-chat-service/dev.config.yaml 与 vecfluxapi.dev.config.yaml 中的 chat.api_key

bash ai-chat-stack/scripts/ollama.sh start
bash ai-chat-stack/scripts/ollama.sh pull

# 另开终端：./bin/kvstore -c config/kv.conf（VecFluxkv 仓库）

cd ai-chat-stack
bash dev-start.sh
bash dev-stop.sh
bash dev-restart.sh ai-chat-service   # 改 RAG 后重启
```

---

## 目录结构

```text
ReCallRagService/
├── ai-chat-service/     # RAG 核心：chat-server、vecfluxapi
├── ai-chat-backend/     # HTTP API + 静态前端（www/）
├── ai-chat-web/         # Vue 前端源码
├── ai-chat-stack/       # Docker 编排、dev 启停脚本、配置模板
├── keywords-filter/     # 敏感词 gRPC
└── tokenizer/           # 分词 HTTP（tiktoken）
```

---

## 核心配置

| 文件 | 用途 |
|------|------|
| `ai-chat-stack/.env` | Docker 栈：API Key、KV 地址、Ollama 模型 |
| `ai-chat-service/dev.config.yaml` | 本地 Go：chat-service |
| `ai-chat-service/vecfluxapi.dev.config.yaml` | 本地 Go：vecfluxapi |

| 配置段 | 说明 |
|--------|------|
| `embedder.*` | 默认本地 Ollama bge-m3 |
| `chat.*` | LLM；`qa_cache_ttl: 0` = QA 不过期 |
| `kvstore.*` | vec-fluxkv 地址；可读写分离 |
| `vector.*` | 固定 `provider: kvstore`；缓存阈值 0.65 / RAG 阈值 0.4 |
| `dependOn.vecfluxapi` | chat-service → vecfluxapi 地址与 token |

---

## 环境要求

| 依赖 | 说明 |
|------|------|
| Docker / Compose | 应用栈 |
| vec-fluxkv | 存储 + 向量，宿主机 `:2000`（独立仓库） |
| DeepSeek 等 API Key | LLM |
| Ollama + bge-m3 | Embedding（Docker 栈内自动拉；本地 Go 用 `scripts/ollama.sh`） |
| Go 1.21+ | 仅本地 Go 开发 / 知识库 CLI 需要 |
| Python 3 | 仅本地 Go 开发跑 tokenizer 需要 |
