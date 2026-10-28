# LocalMind Hub — 产品需求文档（PRD）

> **状态：M0 定稿（§8 已拍板）**  
> **仍禁止写业务代码**，直到用户明确说「可以开工」；与 `REQUIREMENTS.md` / `SPEC.md` 同步。  
> 目标岗位：[Anker PSP-后端开发类专项](https://anker-in.jobs.feishu.cn/index/position/7638511716222142726/detail)  
> 仓库：https://github.com/cuteLittleDevil-ai/localmind-hub（仓名 `localmind-hub`）

---

## 1. 一句话目标

用 **go-zero** 做**本地优先**的 AIoT Agent 中枢：设备 MQTT 入网与影子 + 端侧 LLM 工具调用控设备 + 本地台账/会话记忆；三条核心链路可离线演示，对齐面试作品集与 Anker AIoT/AI 工程化能力。

---

## 2. 背景（JD + MindBase 对齐）

| 本项目能力 | 对齐 JD | 对齐 MindBase |
|------------|---------|----------------|
| 设备入网、影子、生命周期 API | AIoT 入网 / 全生命周期 | EasyOmni Link（连接层） |
| MQTT 上下行、状态上报 | MQTT 通讯 | 本地协议网关（MVP 用 MQTT 模拟） |
| go-zero 模块化 / 可拆多服务 | 微服务化 | 中枢内模块拆分 |
| Agent：意图 → Tool → 设备动作 | AI 主动服务 / 智能诊断 | TrueSmart Agent |
| SQLite + Redis + Mongo（Kafka 加分；JD 的 MySQL 口述可换） | 本地一体机优先 | EverSafe 本地记忆（简化） |
| Docker Compose / 精简 k8s | 云原生 | 一体机部署形态 |
| Cursor 工程化 + 文档取舍 | AI 工具优先 | — |

产品对标 Anker MindBase（本地单中枢 + 连接 + 端侧 Agent + 本地记忆），**不做** HearthNet 多机租约协议。

---

## 3. 用户与场景

### 3.1 使用者

- 开发者本人（本地 demo、录屏）
- 面试官（观看录屏 / 现场演示）

### 3.2 主场景

1. **设备上线**：虚拟设备经 MQTT 上线 → 台账可见、影子同步  
2. **自然语言控灯**：「把客厅灯调到 30%」→ Agent 选 tool → MQTT 下发 → 状态回读  
3. **断网仍可用**：断开外网仍能完成场景 2；进程重启后设备台账与近期会话仍在  
4. （加分）设备长时间离线 → 简单诊断 / 告警事件  

---

## 4. 目标 / 非目标

### 4.1 目标（MVP）

- 本地一键 Compose 起全栈，三条主链路可演示  
- 设备域：入网、在线、影子、控制回执  
- Agent：Ollama + Tool 白名单控设备  
- 记忆：设备/用户元数据 + 会话/事件持久化  
- 文档与架构可讲述（幂等、超时、离线队列取舍）  

### 4.2 非目标（明确不做）

- **HearthNet 多机租约**协议  
- **社区 / 打印 / 商城**进 MVP（JD 里有，口述同构即可）  
- **完整 Matter 深集成**（connectedhomeip / 完整 Controller；仅适配接口占位）  
- 第二个 Home Assistant（功能广度）  
- 完整用户体系 / 云端账号  

---

## 5. MVP 功能（按域）

### 5.1 设备域 `device`

- 注册/入网：deviceId、类型、房间、能力模型  
- 在线状态（Redis）与设备影子（上报合并）  
- 生命周期：online / offline / unbind（可先简化）  
- 指令下发：幂等 key、超时、结果回执  
- HTTP API：列表 / 详情 / 控制（供 Agent 与调试）  

### 5.2 连接域 `mqtt`（mqttx）

- 对接 EMQX（Compose 一键起；Mosquitto 可替换）  
- 主题约定：`up` / `down` / `ack`（详见 SPEC）  
- 虚拟设备模拟器：灯、温湿度  

### 5.3 Agent 域 `agent`

- OpenAI-compatible 调本地 Ollama（默认可换模型）  
- Tool 白名单：`device.query` / `device.control` / `device.diagnose`  
- 编排：Thought–Action–Observation（ReAct）；危险操作可配置「需确认」  
- HTTP：一轮对话 / 多轮会话 id（WS 可选）  

### 5.4 记忆域 `memory`

- 设备与用户元数据 → **SQLite**（**已确认**：不用 MySQL）  
- 会话与事件 → **Mongo**  
- 不做家庭 NAS 云盘；只做元数据 + 事件流  
- 面试口述：生产/云侧可换 MySQL，本地 demo 用 SQLite 更贴一体机  

### 5.5 网关 `gateway`

- 统一鉴权入口，路由到 device / agent / memory  
- MVP 可与业务同进程（见 §8 拆分建议）  

### 5.6 部署

- `docker-compose`：redis、mongo、emqx、ollama、API（SQLite 文件挂卷，无独立 mysql 容器）  
- （加分 M4）精简 k8s、Kafka 设备事件总线  

---

## 6. MVP 三链路验收标准

| # | 链路 | 验收标准 |
|---|------|----------|
| L1 | 设备上线 | 虚拟设备连 EMQX → 发 up → 台账有该设备、在线、影子字段正确 |
| L2 | 自然语言控灯 | 对话「把客厅灯调到 30%」→ Agent 调 `device.control` → MQTT down → ack → 影子 brightness=30 |
| L3 | 断网仍可用 | 断外网后 L2 仍可完成（Ollama 本机）；重启进程后台账与近期会话可查 |

加分：离线超时诊断事件可写入事件流并可查询。

---

## 7. 非功能

- **本地优先**：停外网，控设备与查影子仍可用（LLM 依赖本机 Ollama）  
- **可演示**：一条 README 命令跑通三条主链路  
- **可讲述**：架构图 + 与云控差异 + 幂等/离线队列取舍  
- **安全底线**：Tool 白名单、无任意代码执行、Prompt 注入基本防护（系统提示约束 + 参数校验）  
- **语言**：代码与提交信息中文或中英混合；对外 README 中文优先  

---

## 8. 里程碑 M0–M4

| 阶段 | 交付 | 备注 |
|------|------|------|
| **M0** | GitHub 仓 + 需求/PRD/SPEC 确认 | **§8 已拍板**；用户说「可以开工」前仍不写业务代码 |
| **M1** | Compose + device 入网/影子 API + 虚拟设备 | L1 验收 |
| **M2** | Agent + Ollama + 2～3 tools + 自然语言控灯 | L2 验收 |
| **M3** | 断网演示、简单诊断、架构文档与 Demo 视频 | L3 验收 |
| **M4** | Kafka / k8s / 压测一页（可选加分） | 非 MVP 阻断项 |

---

## 9. §8 拍板结果（已定稿）

| # | 议题 | 决定 | 状态 |
|---|------|------|------|
| 1 | 仓名与可见性 | 仓名 `localmind-hub`；可见性 **public** | **已确认** |
| 2 | 服务拆分 | **monorepo 先单 API**，四域划界，**M2 后再拆多进程** | **已确认** |
| 3 | 记忆存储 | 元数据 **SQLite**；会话与事件 **Mongo**（不用 MySQL） | **已确认** |
| 4 | Matter | **仅适配接口占位**，不接 HA Matter Server | **已确认** |
| 5 | 鉴权 | 本地 demo **固定 token / 简单 JWT**，不做完整用户体系 | **已确认** |
| 6 | Kafka | **不进 MVP**，放 **M4 加分** | **已确认** |
| 7 | License | **MIT** + README「**非 Anker 官方、与 Anker 无关**」声明 | **已确认** |

---

## 10. 面试一句话

> 按 MindBase 本地中枢思路，用 go-zero 做 LocalMind Hub：MQTT 设备生命周期与影子，端侧 Agent 工具调设备，台账与会话本地持久化；对齐 AIoT 入网/MQTT/微服务与 AI 工程化，Matter 留适配层避免 demo 被协议栈拖死。

---

## 11. 文档关系

- `REQUIREMENTS.md`：需求整理（上游）  
- `docs/PRD.md`：本文，产品范围与验收  
- `docs/SPEC.md`：技术规格讨论稿（接口 / MQTT / 数据模型）  

**本文档与 SPEC 确认前禁止实现业务代码。**
