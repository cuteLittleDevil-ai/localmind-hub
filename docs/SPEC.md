# LocalMind Hub — 技术规格讨论稿（SPEC）

> **状态：M0 定稿（§8 已拍板）**  
> **仍禁止写业务代码**，直到用户明确说「可以开工」；与 `REQUIREMENTS.md` / `docs/PRD.md` 同步。  
> 仓库：https://github.com/cuteLittleDevil-ai/localmind-hub  
> 路径风格：go-zero 惯例；鉴权：**固定 token / 简单 JWT**（已确认）。

**明确：用户说「可以开工」前禁止实现业务代码。**

---

## 1. 系统上下文与模块边界

### 1.1 上下文

```text
[面试官/开发者]
       │ HTTP (Bearer token)
       ▼
┌──────────────────┐     MQTT      ┌─────────────────┐
│  LocalMind Hub   │◄─────────────►│ EMQX            │
│  (go-zero API)   │               │ + 虚拟设备模拟器 │
└────────┬─────────┘               └─────────────────┘
         │
    ┌────┼────┬────────┐
    ▼    ▼    ▼        ▼
 SQLite Redis Mongo  Ollama
```

本地优先：外网断开后，Hub ↔ EMQX ↔ 虚拟设备、Hub ↔ Ollama 仍可用。

### 1.2 四域边界

| 域 | 职责 | 对外 |
|----|------|------|
| **device** | 入网、影子、生命周期、控制幂等与回执 | HTTP + 消费 MQTT up/ack |
| **mqttx** | EMQX 客户端、主题编解码、发布 down | 内部包，非独立 HTTP |
| **agent** | 会话编排、Tool 白名单、调 Ollama | HTTP 对话 |
| **memory** | 会话/消息/事件持久化；元数据读写封装 | 内部 + 少量查询 API |

Matter：仅 `pkg/matter`（或等价）**适配接口占位**，MVP 不接 HA Matter Server — **已确认**。

### 1.3 目录结构（monorepo 单 API — **已确认**）

```text
localmind-hub/
├── apps/
│   └── hub/                 # 单进程 API（gateway + 四域 handler）
│       ├── etc/
│       ├── internal/
│       │   ├── handler/
│       │   ├── logic/
│       │   ├── svc/
│       │   └── types/
│       └── hub.go
├── pkg/
│   ├── mqttx/               # 主题与客户端
│   ├── llm/                 # Ollama OpenAI-compatible
│   ├── tools/               # device.query|control|diagnose
│   └── matter/              # 适配接口占位（空实现）
├── demo/                    # 虚拟设备 + 对话脚本
├── deploy/
│   ├── docker-compose.yml
│   └── k8s/                 # M4 加分
├── docs/
│   ├── PRD.md
│   ├── SPEC.md
│   ├── architecture.md
│   └── IMPLEMENTATION.md
└── REQUIREMENTS.md
```

M2 后再按需拆 `apps/device|agent|memory` 多进程 — **已确认**。

---

## 2. 技术选型表

| 层级 | 选型 | 备注 |
|------|------|------|
| 语言 / 框架 | Go + go-zero | 对齐 JD |
| 元数据库 | SQLite | 设备/用户元数据 — **已确认**（不用 MySQL；面试可述生产换 MySQL） |
| 缓存 | Redis | 在线状态、幂等 key、短期影子缓存 |
| 文档库 | MongoDB | 会话、消息、事件 — **已确认** |
| 消息 | MQTT / EMQX | MVP 必选 |
| 事件总线 | Kafka | **不进 MVP，M4 加分** — **已确认** |
| LLM | Ollama（OpenAI-compatible） | 本地模型可换 |
| 部署 | Docker Compose | MVP；k8s M4 加分 |
| 鉴权 | 固定 token / 简单 JWT | **不做完整用户体系** — **已确认** |
| License | MIT + 非官方声明 | **已确认** |

---

## 3. MQTT 主题约定草案

前缀：`localmind`

| 方向 | Topic | 说明 |
|------|-------|------|
| 上行 | `localmind/{deviceId}/up` | 状态/事件上报 |
| 下行 | `localmind/{deviceId}/down` | 控制指令 |
| 回执 | `localmind/{deviceId}/ack` | 执行结果 |

### 3.1 消息字段草案（JSON）

**up（状态/事件）**

```json
{
  "deviceId": "light-living-01",
  "ts": 1730000000000,
  "type": "shadow|event",
  "payload": {
    "online": true,
    "brightness": 30,
    "room": "living"
  }
}
```

**down（控制）**

```json
{
  "cmdId": "uuid",
  "idempotencyKey": "client-key-xxx",
  "action": "set",
  "params": { "brightness": 30 },
  "timeoutMs": 5000
}
```

**ack（回执）**

```json
{
  "cmdId": "uuid",
  "idempotencyKey": "client-key-xxx",
  "ok": true,
  "code": 0,
  "message": "ok",
  "shadow": { "brightness": 30 }
}
```

主题与字段锁定前可微调，与 PRD L1/L2 对齐即可。

---

## 4. HTTP API 草案

基路径建议：`/api/v1`（go-zero rest 惯例）。  
鉴权：`Authorization: Bearer <token>` — **待用户确认**（默认固定 demo token 或简单 JWT）。

### 4.1 健康检查

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/health` | liveness；可附带 mysql/redis/mongo/mqtt/ollama 探测摘要 |

### 4.2 设备

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/devices` | 列表（房间/类型/在线过滤可选） |
| GET | `/api/v1/devices/:deviceId` | 详情 + 影子 |
| POST | `/api/v1/devices` | 注册/入网 |
| POST | `/api/v1/devices/:deviceId/control` | 控制（写幂等 key；内部发 MQTT down，等 ack） |
| POST | `/api/v1/devices/:deviceId/unbind` | 解绑（可简化） |

**控制请求体草案**

```json
{
  "idempotencyKey": "optional-or-server-gen",
  "action": "set",
  "params": { "brightness": 30 },
  "timeoutMs": 5000
}
```

### 4.3 Agent 对话

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent/chat` | 一轮或带 `sessionId` 多轮 |
| GET | `/api/v1/agent/sessions/:sessionId` | 会话摘要/消息列表（可选） |

**chat 请求体草案**

```json
{
  "sessionId": "",
  "message": "把客厅灯调到 30%",
  "confirmDangerous": false
}
```

**chat 响应草案**：回复文本、所用 tools、可选更新后的 sessionId、设备侧结果摘要。

---

## 5. 数据模型草案

存储分工 — **已确认**：元数据 **SQLite**；会话与事件 **Mongo**；**不用 MySQL**。

### 5.1 SQLite

文件路径建议：`data/localmind.db`（Compose 挂卷）。类型用 SQLite 习惯即可。

**devices**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增 |
| device_id | TEXT UNIQUE | 业务主键 |
| type | TEXT | light / sensor / … |
| name | TEXT | 展示名 |
| room | TEXT | 房间 |
| capabilities | TEXT(JSON) | 能力模型 |
| shadow | TEXT(JSON) | 最近合并影子（可选，热数据也可 Redis） |
| status | TEXT | online / offline / unbound |
| created_at / updated_at | TEXT(RFC3339) | |

**users**（极简，demo 可单用户）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| username | TEXT UNIQUE | |
| token_hint | TEXT | 可选；真实 token 不落明文或仅 hash |

### 5.2 Mongo 集合 — **已确认**

**sessions**

```json
{
  "_id": "ObjectId",
  "sessionId": "uuid",
  "userId": "demo",
  "createdAt": "ISODate",
  "updatedAt": "ISODate",
  "title": "optional"
}
```

**messages**

```json
{
  "_id": "ObjectId",
  "sessionId": "uuid",
  "role": "user|assistant|tool",
  "content": "...",
  "toolName": "device.control",
  "toolArgs": {},
  "createdAt": "ISODate"
}
```

**events**

```json
{
  "_id": "ObjectId",
  "deviceId": "light-living-01",
  "type": "offline_timeout|control_fail|diagnose",
  "payload": {},
  "createdAt": "ISODate"
}
```

---

## 6. Agent Tool 白名单

仅允许下列 tools（名称可微调，须与代码常量表一致）：

| Tool | 作用 | 侧效 |
|------|------|------|
| `device.query` | 查列表/详情/影子 | 只读 |
| `device.control` | 下发控制 | 写；可配置需确认 |
| `device.diagnose` | 简单离线/超时诊断 | 只读或写事件 |

禁止：任意 shell、任意 HTTP、未声明设备动作。系统提示约束 + 参数 schema 校验。

---

## 7. 幂等、超时、离线队列（取舍简述）

| 问题 | MVP 取舍 |
|------|----------|
| **幂等** | 控制带 `idempotencyKey`；Redis SET NX + TTL；相同 key 返回首次结果，不重复 down |
| **超时** | down 带 `timeoutMs`（默认 5s）；Hub 等待 ack，超时记失败并可写 events |
| **离线队列** | MVP **不做**持久离线指令队列；设备 offline 时 control 直接失败并提示；M3+ 可讨论 Redis List 有界队列 |
| **影子** | up 合并写入；读路径 Redis 优先，回源 SQLite |

面试可讲：云控常做 QoS+离线队列；本地 demo 优先可演示与语义清晰，避免半吊子队列掩盖设备状态错误。

---

## 8. 部署：docker-compose 组件清单

| 服务 | 用途 | MVP |
|------|------|-----|
| sqlite 卷 | 元数据文件 `data/localmind.db` | 是（无独立 mysql 容器） |
| redis | 在线/幂等/缓存 | 是 |
| mongo | 会话/事件 | 是 |
| emqx | MQTT Broker | 是 |
| ollama | 本地 LLM | 是 |
| hub（本服务） | go-zero API | 是 |
| demo-device（可选同 compose） | 虚拟灯/传感器 | 是 |
| kafka | 事件总线 | **否，M4** |
| k8s manifests | 精简部署 | **否，M4** |

---

## 9. 与 PRD 的追溯（三链路 → 接口 / MQTT）

| PRD 链路 | 路径 |
|----------|------|
| **L1 设备上线** | demo 连 EMQX → `.../up` → mqttx 消费 → device 更新 SQLite+Redis → `GET /api/v1/devices` 可见 |
| **L2 自然语言控灯** | `POST /api/v1/agent/chat` → Ollama + `device.control` → `POST .../control` 或内部等价逻辑 → `.../down` → `.../ack` → 影子更新 → 回复用户 |
| **L3 断网仍可用** | Compose 内网；无外网依赖；Ollama 本机；重启后 SQLite/Mongo 仍可查台账与会话 |

---

## 10. §8 拍板结果（与 PRD 对齐）

1. **可见性 public** — **已确认**  
2. **先单 API monorepo，M2 后再拆** — **已确认**  
3. **SQLite 元数据 + Mongo 会话/事件（不用 MySQL）** — **已确认**  
4. **Matter 仅占位** — **已确认**  
5. **固定 token / 简单 JWT** — **已确认**  
6. **Kafka 不进 MVP（M4）** — **已确认**  
7. **MIT + 非 Anker 官方声明** — **已确认**  

---

## 11. 实现禁令

- 用户明确说「**可以开工**」前**禁止实现**业务代码  
- 禁止以「先写个骨架」绕过确认  
- 文档进仓（README / docs）须用户另行同意后再 commit  

**未说「可以开工」前禁止实现。**
