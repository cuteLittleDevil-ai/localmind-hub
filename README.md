# LocalMind Hub

本地优先的 AIoT Agent 中枢（面试作品集）。设备 MQTT 入网与影子、端侧 LLM 工具控设备、本地台账与会话记忆；详见定稿文档。

当前代码进度：**M1（L1）** — Compose 依赖、hub、SQLite 设备台账、`pkg/mqttx`、设备 HTTP API、虚拟灯与验收脚本。

## 文档

- [需求整理（REQUIREMENTS.md）](./REQUIREMENTS.md)
- [产品需求（docs/PRD.md）](./docs/PRD.md)
- [技术规格（docs/SPEC.md）](./docs/SPEC.md)
- [架构说明（docs/architecture.md）](./docs/architecture.md) — Things 对照与模块边界
- [业务代码实现计划（docs/IMPLEMENTATION.md）](./docs/IMPLEMENTATION.md) — 里程碑任务拆解

## 快速开始（M1）

### 1. 启动依赖（Redis / Mongo / EMQX）

在仓库根目录：

```bash
docker compose -f deploy/docker-compose.yml up -d redis mongo emqx
```

说明见 [deploy/README.md](./deploy/README.md)。SQLite 使用本机目录 `data/`（已 `.gitignore` 数据库文件，保留 `.gitkeep`）。

### 2. 本地运行 Hub（推荐开发）

```bash
cd apps/hub
go run . -f etc/hub.yaml
```

默认监听 `http://0.0.0.0:8888`。

### 3. Demo Token 与 Health

固定 Bearer Token（见 `apps/hub/etc/hub.yaml` 中 `Auth.DemoToken`）：

```text
localmind-demo-token
```

**Health 无需鉴权**：

```bash
curl -s http://127.0.0.1:8888/api/v1/health | jq .
```

**其它 API 需带 Token**：

```bash
export TOKEN=localmind-demo-token
export HUB=http://127.0.0.1:8888
```

### 4. 虚拟灯（L1）

另开终端，在仓库根目录：

```bash
go run ./demo/virtual-light -broker tcp://127.0.0.1:1883 -device light-living-01 -room living
```

### 5. L1 验收（注册 → 在线 → 控灯）

确保依赖、hub、虚拟灯均已启动后：

```bash
chmod +x demo/scripts/l1-smoke.sh
./demo/scripts/l1-smoke.sh
```

或手动 curl：

```bash
# 注册
curl -s -H "Authorization: Bearer localmind-demo-token" -H "Content-Type: application/json" \
  -X POST "$HUB/api/v1/devices" -d '{"deviceId":"light-living-01","type":"light","name":"客厅灯","room":"living"}' | jq .

# 列表
curl -s -H "Authorization: Bearer localmind-demo-token" "$HUB/api/v1/devices?room=living" | jq .

# 控制亮度
curl -s -H "Authorization: Bearer localmind-demo-token" -H "Content-Type: application/json" \
  -X POST "$HUB/api/v1/devices/light-living-01/control" \
  -d '{"idempotencyKey":"demo-1","action":"set","params":{"brightness":30},"timeoutMs":5000}' | jq .

# 影子
curl -s -H "Authorization: Bearer localmind-demo-token" "$HUB/api/v1/devices/light-living-01" | jq .
```

依赖未就绪时 health 仍返回 JSON，整体 `status` 为 `degraded`、HTTP **503**，进程不会因此退出。

### 6. 可选：Compose 一并启动 Hub 容器

```bash
docker compose -f deploy/docker-compose.yml up -d --build hub
```

## 构建与测试

```bash
go build -o bin/hub ./apps/hub
go build -o bin/virtual-light ./demo/virtual-light
go test ./...
```

## 声明

本项目为个人学习与面试展示用途，**非 Anker 官方产品，与 Anker 无关**。产品思路对标公开信息中的 MindBase 概念，不构成任何关联或背书。

## License

[MIT](./LICENSE)
