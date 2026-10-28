# LocalMind Hub — Docker Compose

提供 **Redis、MongoDB、EMQX、Ollama（M2）** 与可选 **Hub** 容器。SQLite 元数据文件挂载在仓库根目录 `data/`。

## 用法

在仓库根目录：

```bash
# 仅依赖
docker compose -f deploy/docker-compose.yml up -d redis mongo emqx ollama

# 含 Hub 容器
docker compose -f deploy/docker-compose.yml up -d --build hub
```

首次 L2 需拉模型（与 `hub.yaml` 中 `Ollama.Model` 一致，默认 `qwen2.5:3b`）：

```bash
docker compose -f deploy/docker-compose.yml exec ollama ollama pull qwen2.5:3b
```

## 服务与端口

| 服务 | 端口 | 说明 |
|------|------|------|
| redis | 6379 | 在线状态 / 幂等 / 缓存 |
| mongo | 27017 | Agent 会话与消息 |
| emqx | 1883, 18083 | MQTT |
| ollama | 11434 | OpenAI 兼容 LLM API |
| hub（可选） | 8888 | 见 compose；开发推荐本地 `go run` |

Hub 容器配置：`apps/hub/etc/hub-docker.yaml`（`redis` / `mongo` / `emqx` / `ollama` 服务名）。

更完整启动与验收步骤见 [docs/RUNBOOK.md](../docs/RUNBOOK.md)。
