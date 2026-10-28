# LocalMind Hub

本地优先的 AIoT Agent 中枢（面试作品集）。设备 MQTT 入网与影子、端侧 LLM 工具控设备、本地台账与会话记忆；详见定稿文档。

当前代码进度：**M2（L2）** — 在 M1 基础上增加 Ollama、`pkg/llm`、`pkg/tools`、Agent 编排、Mongo 会话，以及 `POST /api/v1/agent/chat`。

## 文档

- [需求整理（REQUIREMENTS.md）](./REQUIREMENTS.md)
- [产品需求（docs/PRD.md）](./docs/PRD.md)
- [技术规格（docs/SPEC.md）](./docs/SPEC.md)
- [架构说明（docs/architecture.md）](./docs/architecture.md) — Things 对照与模块边界
- [业务代码实现计划（docs/IMPLEMENTATION.md）](./docs/IMPLEMENTATION.md) — 里程碑任务拆解
- **[运行与验收手册（docs/RUNBOOK.md）](./docs/RUNBOOK.md)** — 启动顺序、预期日志、409、L1/L2 验收

## 快速开始（M1 + M2）

### 1. 启动依赖（Redis / Mongo / EMQX / Ollama）

在仓库根目录：

```bash
docker compose -f deploy/docker-compose.yml up -d redis mongo emqx
# M2 对话需要 Ollama（可与上一条一起 up）
docker compose -f deploy/docker-compose.yml up -d ollama
```

**默认模型**：`qwen2.5:3b`（配置见 `apps/hub/etc/hub.yaml` → `Ollama.Model`）。首次使用需拉取（体积约 2GB 量级，视网络而定）：

```bash
docker compose -f deploy/docker-compose.yml exec ollama ollama pull qwen2.5:3b
```

本地安装 Ollama 时也可：`ollama pull qwen2.5:3b`，Hub 默认连 `http://127.0.0.1:11434/v1`。

说明见 [deploy/README.md](./deploy/README.md)。SQLite 使用本机目录 `data/`。

### 2. 本地运行 Hub（推荐开发）

```bash
cd apps/hub
go run . -f etc/hub.yaml
```

无 Ollama 时 Hub **仍可启动**；`GET /api/v1/health` 中 `ollama` 检查为失败、整体 `degraded`；`POST /api/v1/agent/chat` 返回 **503** 与明确错误，不会 panic。

### 3. Demo Token 与 Health

固定 Bearer Token：

```text
localmind-demo-token
```

**Health 无需鉴权**：

```bash
curl -s http://127.0.0.1:8888/api/v1/health | jq .
```

### 4. 虚拟灯（L1）

```bash
go run ./demo/virtual-light -broker tcp://127.0.0.1:1883 -device light-living-01 -room living
```

### 5. L1 验收

```bash
chmod +x demo/scripts/l1-smoke.sh
./demo/scripts/l1-smoke.sh
```

重复注册同一 `deviceId` 会 **409**，脚本视为已存在并继续。详见 [RUNBOOK](./docs/RUNBOOK.md)。

### 6. L2 验收（自然语言控灯）

Ollama 与默认模型就绪后：

```bash
chmod +x demo/scripts/l2-smoke.sh
./demo/scripts/l2-smoke.sh
```

或：

```bash
curl -s -H "Authorization: Bearer localmind-demo-token" -H "Content-Type: application/json" \
  -X POST http://127.0.0.1:8888/api/v1/agent/chat \
  -d '{"message":"把客厅灯调到 30%"}' | jq .
```

### 7. 可选：Compose 一并启动 Hub 容器

```bash
docker compose -f deploy/docker-compose.yml up -d --build hub
```

Hub 容器使用 `etc/hub-docker.yaml`（服务名 `redis` / `mongo` / `emqx` / `ollama`）。

## 构建与测试

```bash
go build -o bin/hub ./apps/hub
go build -o bin/virtual-light ./demo/virtual-light
go test ./...
```

## 声明

本项目为个人学习与面试展示用途，**非 Anker 官方产品，与 Anker 无关**。

## License

[MIT](./LICENSE)
