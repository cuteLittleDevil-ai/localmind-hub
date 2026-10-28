# LocalMind Hub — Docker Compose（M1 依赖）

本文件提供 **M1** 阶段所需基础设施：**Redis、MongoDB、EMQX**。SQLite 元数据库文件挂载在仓库根目录 `data/`（无独立 DB 容器）。**Ollama 在 M2 再加入**。

## 用法

在仓库根目录：

```bash
docker compose -f deploy/docker-compose.yml up -d
```

停止：

```bash
docker compose -f deploy/docker-compose.yml down
```

## 服务与端口

| 服务 | 端口 | 说明 |
|------|------|------|
| redis | 6379 | 在线状态 / 幂等 / 缓存 |
| mongo | 27017 | 会话与事件（M2+） |
| emqx | 1883, 18083 | MQTT；Dashboard 默认 `admin/public` |
| hub（可选） | 8888 | 见 compose 中 `hub` 服务；也可本地 `go run` |

Hub 容器内配置见 `apps/hub/etc/hub-docker.yaml`（主机名 `redis` / `mongo` / `emqx`）。
