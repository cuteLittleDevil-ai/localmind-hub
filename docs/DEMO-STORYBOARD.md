# Demo 录屏分镜（约 3 分钟，L1 → L2 → L3）

| 时间 | 画面 | 旁白要点 |
|------|------|----------|
| 0:00–0:30 | 终端 `docker compose up` + Hub + 虚拟灯 | 本地栈：EMQX / Redis / Mongo / Ollama，单进程 Hub |
| 0:30–1:00 | `./demo/scripts/l1-smoke.sh` 或 curl 列表/控灯 | L1：MQTT up → 台账 online；control → ack → 影子 |
| 1:00–1:45 | `./demo/scripts/l2-smoke.sh` 或 chat「调到 30%」 | L2：自然语言 → tool → MQTT 闭环；Mongo messages |
| 1:45–2:15 | 停虚拟灯 → chat 失败 → 再 diagnose | 离线直接失败；diagnose 写 events |
| 2:15–2:45 | 文档/RUNBOOK：断外网步骤（或防火墙示意） | L3：仅内网依赖，无外网 API |
| 2:45–3:00 | 重启 Hub → GET 设备与会话仍在 | SQLite + Mongo 持久；architecture 一页 |

参考：[DEMO.md](./DEMO.md)、[architecture.md](./architecture.md)。
