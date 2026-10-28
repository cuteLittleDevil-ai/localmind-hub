# mqttx

EMQX 客户端与 `localmind/{deviceId}/{up|down|ack}` JSON 编解码（SPEC §3）。

Hub 在启动时连接 broker，订阅 `localmind/+/up` 与 `localmind/+/ack`，控制面通过 `PublishDown` 下发。
