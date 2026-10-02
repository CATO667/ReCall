# ReCallRag Backend

ReCallRag 后端网关：基于 [Gin](https://github.com/gin-gonic/gin) 提供 HTTP API，托管前端静态资源，并通过 gRPC 调用 `ai-chat-service`。

## 本地开发

```bash
go build -o ai-chat-backend cmd/*.go
./ai-chat-backend --config=dev.config.yaml
```

默认监听 `:7080`，静态资源目录为 `www/`。

## Docker 部署

请使用 [`ai-chat-stack`](../ai-chat-stack/) 中的 docker-compose 进行全栈部署。
