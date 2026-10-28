# LocalMind Hub — 业务代码实现计划

> **状态：已确认，经 PR 进仓**  
> 对齐：`REQUIREMENTS.md` / `docs/PRD.md` / `docs/SPEC.md` / `docs/architecture.md`  
> **闸门**：你明确说「可以开工」之前，**禁止**写业务代码、`go.mod`、Compose 业务服务、demo 可执行实现。本文只排期与拆任务。

---

## 1. 目标与原则

**目标**：按 M1→M3 打通 L1/L2/L3；M4 可选加分。仓库保持 monorepo **单进程** `apps/hub`，域边界按 architecture 切包。

**原则**

1. **竖切验收**：每一里程碑结束都能演示对应链路，不做「全空壳再填肉」。  
2. **先通路后花样**：MQTT up/down/ack → HTTP 设备 → Agent tools → 断网与诊断。  
3. **契约先行**：API / 主题 / 表结构以 SPEC 为准；改契约先改文档再改代码。  
4. **中文 commit / PR 描述**；重大架构变更先问你。  
5. Matter 仅 `pkg/matter` 空接口；Kafka/k8s 不进 M1–M3。

---

## 2. 里程碑总览

| 阶段 | 主题 | 验收 | 建议产出 |
|------|------|------|----------|
| **M0** | 文档基线 | 仓 + PRD/SPEC/architecture | **已完成**（PR #1 已合；#2 architecture 待合） |
| **M1** | 连接 + 设备台账 | **L1** | Compose 依赖 + hub 骨架 + mqttx + device API + demo 灯（**M1 已合 PR #4+#5**） |
| **M2** | Agent 控灯 | **L2** | Ollama + `pkg/llm` + tools + chat API + Mongo 会话（**进行中 / 本 PR**） |
| **M3** | 本地优先与讲述 | **L3** | 断网脚本/说明、diagnose、Demo 录屏清单、README 一键跑通 |
| **M4** | 加分（可选） | 非阻断 | Kafka 事件总线占位/一页、k8s 精简、简单压测笔记 |

依赖关系：`M1 → M2 → M3`；`M4` 可与 M3 文档并行，不挡面试主线。

---

## 3. M1 详细任务（L1：设备上线）

**DoD**：`docker compose up` 后，虚拟灯连 EMQX 发 `up` → `GET /api/v1/devices` 可见、online、影子字段正确；`POST .../control` 能 down→ack→影子更新（可用 curl，不经 Agent）。

| # | 任务 | 说明 | 依赖 |
|---|------|------|------|
| M1-1 | 仓库骨架 | `go.mod`、`apps/hub` go-zero 单服务、目录按 SPEC §1.3；`pkg/matter` 空接口 | 「可以开工」 |
| M1-2 | Compose 依赖 | redis / mongo / emqx（+ 卷挂 `data/`）；**暂不强制** ollama | M1-1 |
| M1-3 | 配置与鉴权 | `etc/*.yaml`；Bearer 固定 demo token（或简单 JWT）中间件；`GET /health` | M1-1 |
| M1-4 | SQLite 元数据 | `devices`（及极简 `users`）；迁移或启动建表 | M1-1 |
| M1-5 | `pkg/mqttx` | 连 EMQX；订阅 `localmind/+/up`、`.../ack`；发布 `.../down`；JSON 编解码按 SPEC §3 | M1-2 |
| M1-6 | device 域 | 注册、列表、详情+影子；up 合并影子；Redis 在线；控制幂等（Redis NX）+ 等 ack | M1-4, M1-5 |
| M1-7 | demo 虚拟灯 | `demo/`：连 MQTT，处理 down，回 ack，周期/变更发 up（brightness） | M1-5 |
| M1-8 | L1 验收脚本 | README 或 `demo/scripts`：注册→模拟上线→curl 断言 | M1-6, M1-7 |

**刻意不做（M1）**：Agent、Ollama、Mongo 会话、Kafka、多进程拆分。

---

## 4. M2 详细任务（L2：自然语言控灯）

**DoD**：`POST /api/v1/agent/chat` 发「把客厅灯调到 30%」→ 调用 `device.control` → MQTT 闭环 → 回复含结果；`brightness=30` 可查；消息入 Mongo。

| # | 任务 | 说明 | 依赖 |
|---|------|------|------|
| M2-1 | Compose 加 Ollama | 拉默认模型说明写入 README（体积/首次拉取） | M1 |
| M2-2 | `pkg/llm` | OpenAI-compatible 客户端（chat + tools） | M2-1 |
| M2-3 | `pkg/tools` | 白名单三工具；schema 校验；内部调 device 逻辑（禁止旁路 MQTT） | M1-6 |
| M2-4 | agent 编排 | ReAct：系统提示 + tool loop；可配置危险操作确认 | M2-2, M2-3 |
| M2-5 | memory（Mongo） | sessions / messages；chat 读写会话 | M1-2 |
| M2-6 | HTTP chat | `POST /agent/chat`、可选 `GET .../sessions/:id` | M2-4, M2-5 |
| M2-7 | L2 验收 | 对话脚本 + 影子断言；失败路径（设备离线）可读 | M2-6, M1-7 |

**刻意不做（M2）**：多进程拆分（可在 M2 **末**评估，默认仍单进程）；完整用户体系。

---

## 5. M3 详细任务（L3：断网仍可用）

**DoD**：断外网（或文档给出可复现步骤）下仍能完成 L2；重启 hub 后 SQLite 台账与 Mongo 近期会话可查；`device.diagnose` 可写离线超时类事件。

| # | 任务 | 说明 |
|---|------|------|
| M3-1 | 本地优先检查清单 | 依赖仅内网 Compose + 本机 Ollama；文档标明禁止的外网调用 |
| M3-2 | 重启持久化抽检 | 停/起 hub 与依赖后的验收步骤 |
| M3-3 | `device.diagnose` | 基于 Redis 在线/最后 up 时间的简单诊断；写 Mongo `events` |
| M3-4 | Demo 包 | 一键命令、示例对话、架构口述要点（链到 architecture） |
| M3-5 | （可选）录屏分镜 | L1→L2→L3 三分钟结构，便于面试 |

---

## 6. M4（可选加分）

| # | 任务 | 说明 |
|---|------|------|
| M4-1 | Kafka | 设备事件旁路总线；不替代 MQTT 控制面 |
| M4-2 | `deploy/k8s` | 精简 Deployment/Service 清单 |
| M4-3 | 压测一页 | 控制 QPS / 在线设备数量级与瓶颈口述 |

---

## 7. 建议实现顺序（开工后第一周）

```text
Day 1–2   M1-1～M1-3   骨架 + Compose + health/鉴权
Day 2–3   M1-4～M1-5   SQLite + mqttx
Day 3–4   M1-6～M1-8   device + demo 灯 + L1 绿
Day 5–6   M2-1～M2-4   Ollama + tools + 编排
Day 6–7   M2-5～M2-7   Mongo 会话 + chat + L2 绿
之后      M3           L3 + 诊断 + Demo 文档
按需      M4
```

节奏可按你面试时间压缩；**先保 L1，再 L2**。

---

## 8. 风险与取舍（提前说清）

| 风险 | 应对 |
|------|------|
| Ollama 模型大 / 首次慢 | README 写清模型名与预拉；demo 可提供「跳过 LLM、直调 control」旁路仅用于排障（不进面试主路径） |
| LLM 不稳选错 tool | 收紧系统提示 + 房间/设备名与 demo 一致；失败回显 tool 轨迹 |
| ack 超时难调 | M1 先把 timeout/幂等打日志；默认 5s（SPEC） |
| 范围膨胀 | 严格非目标清单；物模型保持扁平 JSON（architecture §7-A） |

---

## 9. 协作建议

- 默认由 **LocalMind Hub** 主责计划与契约；开工后实现可拉 **@golang开发**（须你同意）。  
- 架构级改动（拆进程、换存储、改 MQTT 契约）先问你，必要时拉 **@架构师**。  
- PR：小步中文 commit；你网页合并（与现有习惯一致）。

---

## 10. 开工闸门（核对清单）

在你说「可以开工」前确认：

- [x] PRD / SPEC / §8 定稿  
- [x] architecture Things 对照（PR #2）  
- [x] 本实现计划无异议（或你改过里程碑）  
- [ ] PR #2 已合并（建议，避免文档分叉）  
- [ ] 你明确说：**可以开工**

开工后第一刀只做 **M1-1～M1-3**，不并行铺 Agent。

---

## 11. 文档关系

| 文档 | 用途 |
|------|------|
| PRD | 做什么、验什么 |
| SPEC | 接口/主题/表怎么定 |
| architecture | 为什么这样切（Things 对照） |
| **本文** | 按什么顺序写代码 |

冲突时以已拍板的 PRD/SPEC 为准；改计划先改本文再动代码。
