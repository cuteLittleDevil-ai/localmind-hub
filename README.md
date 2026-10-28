# LocalMind Hub

本地优先的 AIoT Agent 中枢（面试作品集）。设备 MQTT 入网与影子、端侧 LLM 工具控设备、本地台账与会话记忆；详见定稿文档。

当前代码进度：**M1-1～M1-3**（hub 骨架、Compose 依赖、health / 鉴权）。设备域、mqttx、Agent 等在后续里程碑实现。

## 文档

- [需求整理（REQUIREMENTS.md）](./REQUIREMENTS.md)
- [产品需求（docs/PRD.md）](./docs/PRD.md)
- [技术规格（docs/SPEC.md）](./docs/SPEC.md)
- [架构说明（docs/architecture.md）](./docs/architecture.md) — Things 对照与模块边界

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

**其它 API 需带 Token**（M1 仅 health；后续设备/Agent 接口同样规则）：

```bash
curl -s -H "Authorization: Bearer localmind-demo-token" \
  http://127.0.0.1:8888/api/v1/health
```

依赖未就绪时 health 仍返回 JSON，整体 `status` 为 `degraded`、HTTP **503**，进程不会因此退出。

### 4. 可选：Compose 一并启动 Hub 容器

```bash
docker compose -f deploy/docker-compose.yml up -d --build hub
```

## 构建与测试

```bash
go build -o bin/hub ./apps/hub
go test ./...
```

## 声明

本项目为个人学习与面试展示用途，**非 Anker 官方产品，与 Anker 无关**。产品思路对标公开信息中的 MindBase 概念，不构成任何关联或背书。

## License

[MIT](./LICENSE)
