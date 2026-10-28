# LocalMind Hub — 运行与验收手册（RUNBOOK）

本文说明本地/Compose 启动顺序、端口、预期日志、常见 HTTP 行为，以及 **L1 / L2 / L3** 验收步骤。契约以 [SPEC.md](./SPEC.md) 为准。

## 0. 端到端流程（面试可读）

| 阶段 | 链路 |
|------|------|
| **M1** | 虚拟灯 MQTT `up` → Hub 更新 SQLite 台账与 Redis 在线/影子 → HTTP 注册/列表/控制（`down/ack`） |
| **M2** | `POST /agent/chat` → Ollama + 白名单 tools → 内部仍经 device 域 MQTT 控灯 → Mongo sessions/messages |
| **M3** | 仅内网 Compose + 本机 Ollama；断外网仍可 L2；重启后 SQLite/Mongo 可查；`device.diagnose` 与控灯失败写 Mongo `events` |

Demo 一键命令与示例对话见 [DEMO.md](./DEMO.md)。

## 1. 启动顺序

推荐顺序（开发机）：

1. **Compose 依赖**：Redis、Mongo、EMQX（M2 起可选 Ollama）
2. **Hub**：`apps/hub`（go-zero API，8888）
3. **虚拟灯**：`demo/virtual-light`（MQTT 上行/执行 down）
4. **验收**：`demo/scripts/l1-smoke.sh`（L1）、`l2-smoke.sh`（L2）、`l3-smoke.sh`（L3 本地优先抽检）

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

## 8. L3 断网仍可用（M3）

**DoD**：断外网（或等价隔离）后仍能完成 L2；Hub 进程不依赖公网 API。

### 8.1 本地优先检查清单（允许 / 禁止）

| 允许（MVP） | 禁止（M3 不做） |
|-------------|-----------------|
| Docker Hub 已拉取的镜像：redis、mongo、emqx、ollama | 运行时访问 OpenAI / 云端 LLM |
| Compose 内网：`127.0.0.1` / `redis` / `mongo` / `emqx` | Kafka、远程配置中心 |
| 本机或 Compose 内 Ollama（`11434`） | Matter 真接入、完整用户体系 |
| Hub ↔ MQTT ↔ 虚拟灯闭环 | 持久离线控制队列（offline 仍直接失败） |

Hub 业务路径**不应**发起外网 HTTP；health 仅探测本地依赖。

### 8.2 模拟断外网（可复现）

任选其一（保留本机 `127.0.0.1` 与 Docker 网桥）：

1. **关 Wi‑Fi / 拔网线**（最简单）：Compose 与 Ollama 仍在 localhost 时，执行 `./demo/scripts/l2-smoke.sh` 或手动 chat。  
2. **防火墙挡默认路由外网**（Linux 示例，需 root，执行前请确认规则可回滚）：

```bash
# 示例：拒绝经默认网关的外出（保留 lo 与 docker0）
sudo iptables -A OUTPUT ! -o lo -d 127.0.0.0/8 -j REJECT
# 验收 L2 后删除规则或 reboot
```

3. **文档验收**：若环境不便真断网，按 [DEMO.md](./DEMO.md) 与 §8.1 清单确认无外网依赖，并跑 `./demo/scripts/l3-smoke.sh` 作本地抽检。

### 8.3 diagnose 与 events

停虚拟灯或等待 >90s 无 `up` 后，通过 Agent 触发诊断：

```bash
curl -s -H "Authorization: Bearer localmind-demo-token" -H "Content-Type: application/json" \
  -X POST "$HUB_URL/api/v1/agent/chat" \
  -d '{"message":"请诊断 light-living-01 是否在线"}' | jq .
```

Mongo `events` 集合应出现 `type` 为 `offline_timeout` 或 `diagnose` 的文档（字段见 SPEC §5.2）。控制离线/ack 超时会写 `control_fail`。

```bash
chmod +x demo/scripts/l3-smoke.sh
./demo/scripts/l3-smoke.sh
```

## 9. 重启持久化抽检（M3-2）

1. 完成 L1 或 L2，记下 `deviceId` 与 `sessionId`（若有）。  
2. **停止 Hub**（Ctrl+C），Compose 依赖可保持运行。  
3. 确认文件仍在：`data/localmind.db`（SQLite 设备台账）。  
4. 确认 Mongo：`mongosh mongodb://127.0.0.1:27017/localmind --eval 'db.sessions.countDocuments()'`  
5. **重新启动 Hub** + 虚拟灯。  
6. `GET /api/v1/devices/:deviceId` 台账仍在；`GET /api/v1/agent/sessions/:sessionId` 历史消息仍在（Mongo 卷未删）。

## 10. 当前未启动 / 未实现项

| 项 | 说明 |
|----|------|
| **Kafka** | M4 加分，不进 MVP |
| **k8s 清单** | M4 |
| **Matter 真接入** | 仅 `pkg/matter` 占位 |
| **完整用户体系** | 固定 demo token |
| **离线控制队列** | MVP 不做；offline 直接失败 |

Agent 与 Ollama 属于 **M2**；M3 增加断网说明、重启抽检、diagnose/events 与 Demo 文档。
