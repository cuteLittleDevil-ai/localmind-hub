# LocalMind Hub — 需求整理（讨论稿，未开工）

> 状态：**§8 已拍板、PRD/SPEC 定稿**；用户说「可以开工」前仍**不写业务代码**。  
> 目标岗位：[Anker PSP-后端开发类专项](https://anker-in.jobs.feishu.cn/index/position/7638511716222142726/detail)  
> 产品对标：Anker MindBase（本地单中枢 + 连接 + 端侧 Agent + 本地记忆），**不做** HearthNet 多机租约协议。

---

## 1. 一句话目标

用 **go-zero** 做一个**本地优先**的 AIoT Agent 中枢：**设备 MQTT 入网与影子** + **端侧 LLM 工具调用控设备** + **本地台账/会话记忆**；核心链路可离线。用于面试展示与 Anker AIoT/AI 工程化能力对齐。

---

## 2. 为什么做（对齐 JD + Anker）

| 能力 | 对齐 JD | 对齐 MindBase |
|------|---------|----------------|
| 设备入网、影子、生命周期 API | AIoT 入网 / 全生命周期 | EasyOmni Link（连接层） |
| MQTT 上下行、状态上报 | MQTT 通讯 | 本地协议网关（MVP 用 MQTT 模拟） |
| go-zero 多服务 | 微服务化 | 中枢内模块拆分 |
| Agent：意图 → Tool → 设备动作 | AI 主动服务 / 智能诊断 | TrueSmart Agent |
| SQLite + Redis + Mongo（可选 Kafka；JD MySQL 口述可换） | 本地一体机优先 | EverSafe 本地记忆（简化） |
| Docker Compose / 精简 k8s | 云原生 | 一体机部署形态 |
| Cursor 工程化 + 文档取舍 | AI 工具优先 | — |

**明确不做（MVP / 面试范围外）**

- 用户社区、云盘商城、订阅支付、定制打印（JD 里有，口述同构即可）
- 完整 Matter Controller / connectedhomeip 深集成（只留适配接口）
- 第二个 Home Assistant（功能广度）
- HearthNet 式多 Agent 租约协议

---

## 3. 用户与场景

- **使用者**：开发者本人（本地 demo）+ 面试官观看录屏/现场
- **主场景**
  1. 虚拟设备经 MQTT 上线 → 台账可见、影子同步
  2. 自然语言：「把客厅灯调到 30%」→ Agent 选 tool → MQTT 下发 → 状态回读
  3. 断开外网仍能完成 2；进程重启后设备台账与近期会话仍在
  4.（加分）设备长时间离线 → 简单诊断/告警事件

---

## 4. 功能需求（MVP）

### 4.1 设备域 `device`
- 注册/入网（deviceId、类型、房间、能力模型）
- 在线状态（Redis）与设备影子（上报合并）
- 生命周期：online / offline / unbind（可先简化）
- 指令下发：幂等 key、超时、结果回执
- HTTP API 列表/详情/控制（供 Agent 与调试）

### 4.2 连接域 `mqttx`
- 对接 EMQX 或 Mosquitto（Compose 一键起）
- 主题约定（草案，待讨论锁定）：
  - `localmind/{deviceId}/up` — 状态/事件
  - `localmind/{deviceId}/down` — 控制指令
  - `localmind/{deviceId}/ack` — 执行回执
- 虚拟设备模拟器（demo 脚本）：灯、温湿度

### 4.3 Agent 域 `agent`
- OpenAI-compatible 调用本地 Ollama（默认可换模型）
- Tool 白名单：`device.query` / `device.control` / `device.diagnose`（名称可改）
- 编排：Thought–Action–Observation（ReAct 风格），危险操作可配置「需确认」
- HTTP/WS：一轮对话 / 多轮会话 id

### 4.4 记忆域 `memory`（EverSafe 简化）
- 会话消息落 Mongo（**已确认**）
- 设备与用户元数据 **SQLite**（**已确认**：不用 MySQL；面试可述生产换 MySQL）
- 不强调「家庭 NAS 云盘」，只做元数据 + 事件流即可

### 4.5 网关 `gateway`
- 统一鉴权（本地 demo 可用固定 token / 简单 JWT）
- 路由到 device / agent / memory

### 4.6 部署
- `docker-compose`：redis、mongo、emqx、ollama、各服务（SQLite 文件挂卷）
- （加分）`deploy/k8s` 精简清单
- （加分）Kafka 设备事件总线

---

## 5. 非功能

- **本地优先**：停外网，控设备与查影子仍可用（LLM 依赖本机 Ollama）
- **可演示**：一条 README 命令跑通三条主链路
- **可讲述**：架构图 + 与云控差异 + 幂等/离线队列取舍
- **安全底线**：Tool 白名单、无任意代码执行、Prompt 注入基本防护（系统提示约束 + 参数校验）
- **语言**：代码与 Git 提交信息中文或中英混合（团队偏好可再对齐）；对外 README 中文优先便于面试

---

## 6. 建议仓库结构（待讨论）

```text
localmind-hub/
├── apps/gateway|device|agent|memory
├── pkg/mqttx|llm|tools
├── deploy/docker-compose.yml | k8s/
├── demo/          # 虚拟设备 + 对话脚本
└── docs/architecture.md
```

技术栈锁定草案：Go + go-zero、SQLite、Redis、MongoDB、EMQX、Ollama；Kafka/k8s 为加分项（不用 MySQL）。

---

## 7. 里程碑（确认需求后再排期）

| 阶段 | 交付 | 备注 |
|------|------|------|
| M0 | GitHub 空仓 + 本需求/PRD/SPEC 确认 | **§8 已拍板**；待「可以开工」 |
| M1 | Compose + device 入网/影子 API + 虚拟设备 | |
| M2 | Agent + Ollama + 2～3 tools + 自然语言控灯 | |
| M3 | 断网演示、简单诊断、架构文档与 Demo 视频 | |
| M4 | Kafka / k8s / 压测一页（可选） | |

---

## 8. 拍板结果（已定稿）

1. 仓名 `localmind-hub`；可见性 **public** — **已确认**
2. monorepo **先单 API**，四域划界，M2 后再拆 — **已确认**
3. **SQLite** 元数据 + **Mongo** 会话（不用 MySQL） — **已确认**
4. Matter **仅接口占位** — **已确认**
5. 鉴权：**固定 token / 简单 JWT** — **已确认**
6. Kafka **不进 MVP**（M4） — **已确认**
7. License：**MIT** +「非 Anker 官方、与 Anker 无关」 — **已确认**

---

## 9. 面试一句话（确认后可微调）

> 按 MindBase 本地中枢思路，用 go-zero 做 LocalMind Hub：MQTT 设备生命周期与影子，端侧 Agent 工具调设备，台账与会话本地持久化；对齐 AIoT 入网/MQTT/微服务与 AI 工程化，Matter 留适配层避免 demo 被协议栈拖死。

---

## 10. 参考（只读，不引入多租约方案）

- 岗位 JD：飞书链接见文首
- 开源对照：HomeAgent、home-llm、HA + Matter、matter-mcp
- 论文对照：IoTaIS 2024 Matter Hub；On-Device LLMs for Home Assistant；Harmony
