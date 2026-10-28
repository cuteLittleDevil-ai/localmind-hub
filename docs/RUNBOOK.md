# LocalMind Hub — 运行与验收手册（RUNBOOK）

本文说明本地/Compose 启动顺序、端口、预期日志、常见 HTTP 行为，以及 **L1 / L2** 验收步骤。契约以 [SPEC.md](./SPEC.md) 为准。

## 1. 启动顺序

推荐顺序（开发机）：

1. **Compose 依赖**：Redis、Mongo、EMQX（M2 起可选 Ollama）
2. **Hub**：`apps/hub`（go-zero API，8888）
3. **虚拟灯**：`demo/virtual-light`（MQTT 上行/执行 down）
4. **验收**：`demo/scripts/l1-smoke.sh`（L1）或 `demo/scripts/l2-smoke.sh`（L2，需 Ollama）

```bash
# 仓库根目录
docker compose -f deploy/docker-compose.yml up -d redis mongo emqx

# M2：本地 LLM（首次会拉模型，体积较大，见 README）
docker compose -f deploy/docker-compose.yml up -d ollama

cd apps/hub && go run . -f etc/hub.yaml

# 另开终端
go run ./demo/virtual-light -broker tcp://127.0.0.1:1883 -device light-living-01 -room living
```

Hub 容器方式：`docker compose -f deploy/docker-compose.yml up -d --build hub`（使用 `etc/hub-docker.yaml` 内网地址）。

## 2. 端口与服务

| 端口 | 服务 | 作用 |
|------|------|------|
| **8888** | Hub HTTP | REST API（设备、Agent、health） |
| **1883** | EMQX | MQTT Broker（设备 up/down/ack） |
| **6379** | Redis | 在线状态、控制幂等、影子缓存 |
| **27017** | MongoDB | Agent 会话与消息（M2+） |
| **11434** | Ollama | OpenAI 兼容 LLM API（M2+，可选） |
| 18083 | EMQX Dashboard | 管理 UI（demo 可不用） |

## 3. 鉴权与 Health

- **Demo Token**：`localmind-demo-token`（与 `apps/hub/etc/hub.yaml` 中 `Auth.DemoToken` 一致）
- **`GET /api/v1/health`**：**无需** Bearer，用于探活
- 其它 `/api/v1/*`：需 `Authorization: Bearer localmind-demo-token`

依赖未就绪时 health 返回 JSON，`status` 为 `degraded`、HTTP **503**，进程**不会**因此退出。

## 4. 预期日志（正常 vs 误报）

### Hub

- 正常：`mqtt connected: tcp://...`、`Starting hub at 0.0.0.0:8888...`
- go-zero 空闲时每分钟可能出现的 **统计日志**（非错误）：
  - `CPU` 使用率
  - `shedding_stat`
  - `qps` 为 0 或很低  
  这些是框架监控输出，**不是**业务报错。

### 虚拟灯

- 连接 EMQX 后周期性上报 shadow（含 `brightness`、`room`）

### Ollama（M2）

- 首次 `docker compose up ollama` 后需拉模型，见 README「默认模型」；未启动 Ollama 时 Hub **仍可启动**，但 `POST /api/v1/agent/chat` 会返回明确错误（类似 health degraded）。

## 5. `POST /api/v1/devices` 返回 409

同一 `deviceId` 已在 SQLite（默认 `data/localmind.db`）注册过时，再次注册会 **409 Conflict**，消息形如 `device already registered`。这是**预期行为**，不是故障。

处理方式（任选）：

- 继续 L1：跳过注册，直接 `GET` / `control`（`l1-smoke.sh` 已把 409 视为已存在）
- 换一个新的 `deviceId`
- 删库重建：停止 Hub 后 `rm -f data/localmind.db`，再启动 Hub（会重新建表）

## 6. L1 手动 curl 与 smoke

环境变量（可选）：

```bash
export HUB_URL=http://127.0.0.1:8888
export TOKEN=localmind-demo-token
export DEVICE_ID=light-living-01
```

一键验收：

```bash
chmod +x demo/scripts/l1-smoke.sh
./demo/scripts/l1-smoke.sh
```

手动步骤见 [README](../README.md)「L1 验收」；设备名/房间与 demo 一致：`light-living-01` / `living`。

## 7. L2 自然语言控灯（M2）

前置：L1 链路通（虚拟灯 online），Mongo 可用，**Ollama 已启动且默认模型已拉取**。

```bash
chmod +x demo/scripts/l2-smoke.sh
./demo/scripts/l2-smoke.sh
```

或手动：

```bash
curl -s -H "Authorization: Bearer localmind-demo-token" -H "Content-Type: application/json" \
  -X POST "$HUB_URL/api/v1/agent/chat" \
  -d '{"message":"把客厅灯调到 30%"}' | jq .

curl -s -H "Authorization: Bearer localmind-demo-token" \
  "$HUB_URL/api/v1/devices/light-living-01" | jq '.shadow.brightness'
```

预期：回复中含控灯结果摘要；影子 `brightness` 为 30；Mongo `messages` 集合有 user/assistant/tool 记录。

**设备离线失败路径**：停止虚拟灯后再 chat，应得到可读错误（设备离线或控制失败），Hub 不 panic。

可选：`GET /api/v1/agent/sessions/:sessionId` 查看会话消息列表。

## 8. 当前未启动 / 未实现项

| 项 | 说明 |
|----|------|
| **Kafka** | M4 加分，不进 MVP |
| **k8s 清单** | M4 |
| **Matter 真接入** | 仅 `pkg/matter` 占位 |
| **完整用户体系** | 固定 demo token |
| **离线控制队列** | MVP 不做；offline 直接失败 |
| **M3 断网清单 / diagnose 写 events** | 下一阶段 |

Agent 与 Ollama 属于 **M2**；M1 仅设备 + MQTT + SQLite 台账。
