# ReCallRagService

基于 **vec-fluxkv** 的 RAG 对话服务：历史 QA 缓存 + 语义检索 + rerank + LLM；命中缓存时跳过 LLM，节省 Token。

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

默认阈值（`dev.config.yaml`）：语义硬缓存 **0.65**，软 RAG **0.4**。

---

## 功能

| 能力 | 说明 |
|------|------|
| 精确 QA 缓存 | 相同问题 KV 直返 |
| 语义 QA 缓存 | 相似问题向量检索 + rerank 后直返 |
| 软 RAG | 中等相似度历史 QA 拼 Prompt |
| 敏感词过滤 | 独立 gRPC 服务 |
| 流式对话 | LLM token 级流式；缓存命中整包分片返回 |

---

## 快速开始

### 1. 生成本地配置

```bash
bash ai-chat-stack/scripts/setup-local-config.sh
```

主要产出：`dev.config.yaml`、`vecfluxapi.dev.config.yaml`、`ai-chat-backend/dev.config.yaml` 等（不覆盖已有文件）。

### 2. 配置 API Key 与 Embedding

**LLM**（两处 yaml 保持一致）：将 `YOUR_DEEPSEEK_API_KEY` 填入 `chat.api_key`。

**Embedding 默认走本地 Ollama**（无需 SiliconFlow Key）：

```yaml
embedder:
  provider: "openai_compat"
  api_key: "ollama"
  base_url: "http://127.0.0.1:11434/v1"
  model: "bge-m3"          # 1024 维
```

```bash
bash ai-chat-stack/scripts/ollama.sh start
bash ai-chat-stack/scripts/ollama.sh pull
```

### 3. 启动 vec-fluxkv 与应用栈

**方式 A · 本地 dev（Go 直跑）**

vec-fluxkv 需自行编译启动（监听 `:2000`），详见 VecFluxkv 项目 README。

```bash
cd ai-chat-stack
bash dev-start.sh
```

**方式 B · Docker 应用栈（VecFlux 在容器外）**

VecFluxkv **不编进 Docker**；先宿主机起 KV，再 build/up RAG 应用。

```bash
cd ai-chat-stack
cp .env.example .env          # 填 DEEPSEEK_API_KEY

# 终端 1：VecFluxkv 自行编译并启动主库
cd ../VecFluxkv && ./bin/kvstore -c config/kv.conf

cd ~/test/AIagent/RecallragService/ai-chat-stack
bash scripts/docker-up.sh
bash scripts/docker-down.sh
```

| 操作 | 命令 |
|------|------|
| 启动 | `bash scripts/docker-up.sh` |
| 停止 | `bash scripts/docker-down.sh`（VecFluxkv 需手动 Ctrl+C） |
| 查看状态 | `docker compose -f docker-compose.yml ps` |
| 清理 build 缓存 | `bash scripts/docker-clean.sh` |

- HTTP：**http://localhost:7080**
- 首次会 pull **bge-m3**（约 1.2 GB，仅一次；模型在 volume 里，后续很快）
- 主从：`bash scripts/docker-up.sh replication`（`.env` 设 `KV_READ_HOST=从库IP`）；从库见 `configs/vecfluxkv/kv-slave-remote.conf.example`

| 服务 | 端口 |
|------|------|
| ai-chat-backend | 7080 |
| ai-chat-service | 50055 |
| vecfluxapi | 50057 |
| sensitive | 50053 |
| tokenizer | 3002 |
| vec-fluxkv | 2000 |
| Ollama（Embed） | 11434 |

```bash
bash dev-stop.sh
bash dev-restart.sh vecfluxapi    # 改 RAG / vecflux 代码后
bash dev-restart.sh ai-chat-service
```

---

## 向量后端

仅支持 **`vector.provider: kvstore`**：QA 向量走 vec-fluxkv 内置 HNSW（`VCREATE` / `VINSERT` / `VSEARCH`）。

---

## 知识库入库

```bash
bash ai-chat-stack/scripts/ingest-kbase.sh

cd ai-chat-service
go run ./chat-server/cmd/ingest-doc --config=dev.config.yaml --file=path/to/doc.md
go run ./chat-server/cmd/ingest-doc --config=dev.config.yaml --list
```

---

## 目录结构

```text
ReCallRagService/
├── ai-chat-service/     # RAG 核心、chat-server、vecfluxapi
├── ai-chat-backend/     # HTTP API + 静态前端
├── ai-chat-web/         # Vue 前端源码
├── ai-chat-stack/       # dev 启停、Docker 部署
├── keywords-filter/     # 敏感词 gRPC
├── tokenizer/           # 分词 HTTP
└── docs/sql/            # 数据库脚本
```

---

## 核心配置

| 文件 | 用途 |
|------|------|
| `ai-chat-service/dev.config.yaml` | chat-service |
| `ai-chat-service/vecfluxapi.dev.config.yaml` | vecfluxapi（dev-start 必需） |

| 配置段 | 说明 |
|--------|------|
| `embedder.*` | 默认本地 Ollama bge-m3 |
| `chat.*` | LLM 模型与参数；`qa_cache_ttl: 0` = QA 不过期 |
| `kvstore.*` | vec-fluxkv 地址；可读写分离 |
| `vector.*` | 向量集合、缓存阈值 0.65 / RAG 阈值 0.4 |
| `dependOn.vecfluxapi` | chat-service → vecfluxapi 地址与 token |

---

## 环境要求

| 依赖 | 说明 |
|------|------|
| Go 1.21+ | 后端 |
| Python 3 | tokenizer（dev 本地；Docker 内自动构建） |
| vec-fluxkv | 存储 + 向量，`:2000` |
| Ollama + bge-m3 | 本地 Embedding（默认） |
| Docker | 可选；全栈见 `ai-chat-stack/docker-compose.yml` |
