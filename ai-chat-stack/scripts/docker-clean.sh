#!/usr/bin/env bash
# 清理 Docker 构建缓存与未使用镜像（不删运行中容器）
set -euo pipefail

echo "清理 build 缓存..."
docker builder prune -af 2>/dev/null || true

echo "清理未使用的镜像/容器/网络..."
docker system prune -af 2>/dev/null || true

docker system df
