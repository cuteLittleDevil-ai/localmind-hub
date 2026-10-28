# LocalMind Hub — 架构说明（Things 对照稿）

> **状态：已确认，经 PR 进仓**  
> 参考：[unitedrhino/things](https://github.com/unitedrhino/things)（联犀开源 go-zero IoT 模块，AGPL-3.0）  
> 对齐：`REQUIREMENTS.md` / `docs/PRD.md` / `docs/SPEC.md`（§8 已拍板）  
> **禁止写业务代码**，直到明确说「可以开工」。

---

## 1. 一句话定位

| 项目 | 定位 |
|------|------|
| **Things** | 云侧 / SaaS 化的 go-zero **设备中台**（产品·设备·物模型·协议网关·OTA…） |
| **LocalMind Hub** | 同一套「接入 → 台账 → 控制」骨架的 **本地单中枢精简版**，再挂端侧 Agent + 本地记忆 |

Things 不整仓搬迁：学**边界与数据流**，不学多租户 / 完整协议栈 / AGPL 依赖。

---

## 2. Things 本仓结构（参考源）

联犀整体是「SaaS + IoT + AI」；**things** 仓主要是 IoT 设备域，账号权限 / 多应用等在 **core / share** 等模块。

| Things 服务 | 角色 | 典型职责 |
|-------------|------|----------|
| **apisvr** | IoT HTTP 网关 | REST / Swagger；鉴权后转发设备业务 |
| **dmsvr** | 设备管理 RPC | 产品、设备、物模型、影子/状态、OTA |
| **dgsvr** | 设备网关 RPC | 接入鉴权、报文路由、对接 MQTT 等协议 |

- 调用形态：同进程 **direct**（`dmdirect` / `dgdirect`）或远程 **gRPC** → 接口不绑死部署。  
- 消息面：Broker 侧 **MQTT**；内部还有 **NATS** 等。  
- 存储偏 MySQL/Postgres + Redis 等；时序常配 TDengine（本仓/周边）。

### 2.1 Things 数据流（抽象）

```text
设备 ──协议──► dgsvr（接入/转换/鉴权）
                 │
                 ▼
              dmsvr（台账、物模型、影子/状态）
                 │
                 ▼
         平台其它能力（历史 / 自动化 / 告警…）

控制：业务/API ──► dmsvr/dgsvr ──下行──► 设备（含回执）
```

AI 不在 things 内写死：角色 + 上下文选工具，经 HTTP / MCP / 脚本调**已授权**的设备接口。

---

## 3. LocalMind Hub 目标形态（已拍板）

```text
[面试官/开发者]
       │ HTTP Bearer（固定 token / 简单 JWT）
       ▼
┌──────────────────────────────────────┐
│  apps/hub（单进程 go-zero API）       │
│  gateway 路由 + device|agent|memory  │
└───┬──────────┬──────────┬────────────┘
    │          │          │
    ▼          ▼          ▼
 mqttx      Ollama     SQLite / Redis / Mongo
    │
    ▼
  EMQX ◄──► demo 虚拟设备
```

- **部署**：MVP Docker Compose；单 API monorepo；M2 后再按需拆进程。  
- **存储**：设备/用户元数据 **SQLite**；会话/消息/事件 **Mongo**；在线/幂等/热影子 **Redis**。  
- **不做**：HearthNet 多机租约；社区/打印/商城；Kafka（M4）；Matter 仅适配占位。

---

## 4. Things 域 → LocalMind 模块对照

| Things | LocalMind | 对齐点 | LocalMind 刻意简化 |
|--------|-----------|--------|-------------------|
| **apisvr** | `apps/hub` handler + 鉴权中间件 | 统一 HTTP 入口、Bearer | 无多租户 / 无完整用户体系；demo token 或简单 JWT |
| **dmsvr** | **device** 域（logic + SQLite/Redis） | 入网、列表/详情、影子合并、控制幂等 | 无产品线/多级物模型/OTA；灯+传感器即可 |
| **dgsvr** | **mqttx**（`pkg/mqttx`） | 主题编解码、up/down/ack、连 EMQX | 仅 MQTT；无 CoAP/TCP/云云对接 |
| 物模型（属性/服务/事件） | `devices.capabilities` + shadow JSON | 「能查什么、能控什么」可描述 | MVP：**扁平 JSON**，不做完整 TSL 引擎 |
| 影子 / 设备状态 | Redis 热缓存 + SQLite `shadow` | up 合并、读优先缓存 | 无云端 desired/reported 双缓冲全家桶；单合并影子够演示 |
| 协议插件 / 多协议 | `pkg/matter` 占位 | 适配接口可演进 | MVP 空实现，不接 HA Matter Server |
| AI / Agent（core 等） | **agent** + `pkg/llm` + `pkg/tools` | Tool 调已授权设备 API | 仅 3 个 tool：`device.query` / `control` / `diagnose` |
| 会话 / 审计（平台侧） | **memory**（Mongo） | 对话与事件可追溯 | 无 SaaS 审计中台 |
| NATS / 内部总线 | （MVP 省略） | — | 进程内调用；Kafka 仅 M4 加分 |
| MySQL 中心库 | SQLite（元数据） | 台账持久化 | 面试可述生产换 MySQL |

**目录映射（概念）**

```text
Things                         LocalMind Hub
──────                         ─────────────
service/apisvr          ≈      apps/hub（HTTP）
service/dmsvr           ≈      apps/hub/.../device* + SQLite
service/dgsvr           ≈      pkg/mqttx + EMQX
（core AI / MCP）       ≈      pkg/llm + pkg/tools + agent
（平台会话/日志）       ≈      memory → Mongo
```

---

## 5. 四域职责与依赖（LocalMind）

| 域 | 职责 | 依赖 | 对外 |
|----|------|------|------|
| **device** | 注册/解绑、影子、在线、控制幂等与等 ack | mqttx、SQLite、Redis、events→memory | HTTP 设备 API；可被 tools 内部调用 |
| **mqttx** | EMQX 客户端、主题、发布 down、订阅 up/ack | EMQX | **仅内部包** |
| **agent** | 会话编排、Tool 白名单、调 Ollama | llm、tools→device、memory | `POST /api/v1/agent/chat` |
| **memory** | sessions / messages / events | Mongo；（元数据读写可经 device） | 内部 + 少量查询 API |

依赖方向（禁止反向）：

```text
agent ──► device ──► mqttx
  │          │
  └──► memory ◄── device（写 events）
```

---

## 6. 三条验收链路 ↔ 架构路径

| PRD | 路径 |
|-----|------|
| **L1 设备上线** | demo → EMQX `localmind/{id}/up` → mqttx → device 更新 SQLite+Redis → `GET /api/v1/devices` |
| **L2 自然语言控灯** | `POST /agent/chat` → Ollama + `device.control` → down → ack → 影子合并 → 自然语言回复 |
| **L3 断网仍可用** | Compose 内网；Ollama 本地；无外网强依赖；重启后 SQLite/Mongo 仍可查 |

控制时序（对齐 Things「下行 + 回执」）：

```text
HTTP/Tool → device（幂等 key）→ mqttx publish down
                ↓
         等待 ack（timeoutMs）
                ↓
         合并 shadow → 返回 / 写 events
```

MQTT 主题（与 SPEC §3 一致）：`.../up` · `.../down` · `.../ack`，前缀 `localmind`。

---

## 7. 物模型粒度（建议默认，可再拍板）

| 方案 | 说明 | 建议 |
|------|------|------|
| **A. 扁平 capabilities + shadow（推荐 MVP）** | JSON 字段即属性；action 名写在 control 里 | **默认采用**：实现快、够讲清「模型→控制→影子」 |
| B. 精简 TSL（属性/服务/事件三表） | 更像 Things | M2+ 若面试官深挖物模型再加 |
| C. 完整联犀式物模型引擎 | 校验、版本、产品模板 | **不做** |

MVP 设备类型建议：`light`（brightness / on）、可选 `sensor`（只读上报）。

---

## 8. 明确不搬清单（面试可主动说）

- 多租户、多应用、项目范围、SaaS 开通流  
- 完整协议栈（CoAP / 自定义 TCP / 云云）与组态 / 3D 孪生 / 场景联动全家桶  
- AGPL 整仓依赖与企业版 AI/RAG（本地 Ollama + 白名单 tool）  
- MySQL 中心化与 NATS 总线（MVP）  
- OTA、大规模设备分片、离线指令持久队列（MVP 离线直接失败，见 SPEC §7）

---

## 9. 与 SPEC 的关系

- **SPEC**：接口、主题字段、表结构、Compose 清单、§8 拍板 — **实现契约**。  
- **本文**：Why 这样切、与 Things 的映射、物模型粒度取舍 — **架构叙事**。  
- 冲突时以已拍板的 SPEC / PRD 为准；本文若改边界须先问你。

---

## 10. 后续（仍不写业务代码）

1. 你确认本文无异议后，用中文 commit 推进仓库（或挂到现有文档 PR）。  
2. 若要收紧物模型 / MQTT 字段，只改 SPEC §3–§5 + 本文 §7。  
3. 你说「可以开工」后再按 SPEC 落 `apps/hub` 骨架与 demo。
