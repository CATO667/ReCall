# ReCallRag Backend

ReCallRag backend gateway built with [Gin](https://github.com/gin-gonic/gin). It serves the web UI, exposes HTTP APIs, and calls `ai-chat-service` over gRPC.

## Local development

```bash
go build -o ai-chat-backend cmd/*.go
./ai-chat-backend --config=dev.config.yaml
```

Default listen address is `:7080`. Static assets are served from `www/`.

## Docker deployment

Use docker-compose in [`ai-chat-stack`](../ai-chat-stack/) for full-stack deployment.
