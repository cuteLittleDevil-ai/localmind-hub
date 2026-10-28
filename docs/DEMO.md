# LocalMind Hub — Demo 包（面试 / 本地一键）

## 端到端流程（M1 → M3）

1. **M1 / L1**：虚拟灯连 EMQX 发 `up` → Hub `mqttx` 消费 → SQLite 台账 + Redis 在线/影子 → HTTP 查设备、MQTT `down/ack` 控灯。  
2. **M2 / L2**：`POST /api/v1/agent/chat` → Ollama + 白名单三工具（`device.query|control|diagnose`）→ 内部仍走 device 域与 MQTT 闭环 → 会话与 tool 轨迹入 Mongo。  
3. **M3 / L3**：依赖仅 Compose 内网 + 本机 Ollama；断外网仍可 L2；重启后 SQLite/Mongo 持久；`device.diagnose` 与控灯失败写入 Mongo `events`。

详细步骤见 [RUNBOOK.md](./RUNBOOK.md)；架构取舍见 [architecture.md](./architecture.md)。

## 一键启动

```bash
# 仓库根目录
docker compose -f deploy/docker-compose.yml up -d redis mongo emqx ollama
docker compose -f deploy/docker-compose.yml exec ollama ollama pull qwen2.5:3b   # 首次，见 README 模型名

cd apps/hub && go run . -f etc/hub.yaml
# 新终端
go run ./demo/virtual-light -broker tcp://127.0.0.1:1883 -device light-living-01 -room living
```

验收脚本：

```bash
chmod +x demo/scripts/l1-smoke.sh demo/scripts/l2-smoke.sh demo/scripts/l3-smoke.sh
./demo/scripts/l1-smoke.sh    # L1
./demo/scripts/l2-smoke.sh    # L2（需 Ollama）
./demo/scripts/l3-smoke.sh    # L3 本地优先抽检
```

Demo Token：`localmind-demo-token`（`apps/hub/etc/hub.yaml`）。

## 示例对话

| 用户说法 | 预期 |
|----------|------|
| 把客厅灯调到 30% | `device.control` → 影子 `brightness=30` |
| 客厅灯现在多亮？ | `device.query` → 返回影子 |
| 客厅灯怎么离线了？帮我诊断 | `device.diagnose` → hints + Mongo `events`（`offline_timeout` 或 `diagnose`） |

手动 curl：

```bash
export HUB_URL=http://127.0.0.1:8888 TOKEN=localmind-demo-token
curl -s -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -X POST "$HUB_URL/api/v1/agent/chat" -d '{"message":"把客厅灯调到 50%"}' | jq .
```

## 面试口述要点（链到 architecture）

- **单进程 Hub**：go-zero 一个 API，四域 `device / mqttx / agent / memory` 包边界清晰（见 [architecture.md](./architecture.md) §Things 对照）。  
- **控制面 MQTT、元数据 SQLite、会话/事件 Mongo、在线 Redis**：本地优先，无 Kafka、无离线控制队列（offline 直接失败）。  
- **Agent 安全**：Tool 白名单 + schema；控制可走 `confirmDangerous`（配置项）。  
- **L3 差异**：断外网仍可演示；`device.diagnose` 写 `events` 便于排障叙事。

可选录屏分镜：[DEMO-STORYBOARD.md](./DEMO-STORYBOARD.md)。
