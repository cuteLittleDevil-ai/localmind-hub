# pkg/tools

Agent 白名单工具（M2+）：`device.query` / `device.control` / `device.diagnose`（M3：diagnose 写 Mongo events）。参数 schema 校验后调用 `pkg/device`，**禁止**旁路 MQTT。
