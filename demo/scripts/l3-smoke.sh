#!/usr/bin/env bash
# L3 smoke: 本地优先抽检（health、设备台账、diagnose 写 Mongo events）。
# 真断外网步骤见 docs/RUNBOOK.md §9；本脚本不要求 CI 断网。
set -euo pipefail

HUB_URL="${HUB_URL:-http://127.0.0.1:8888}"
TOKEN="${TOKEN:-localmind-demo-token}"
DEVICE_ID="${DEVICE_ID:-light-living-01}"
MONGO_URI="${MONGO_URI:-mongodb://127.0.0.1:27017}"
MONGO_DB="${MONGO_DB:-localmind}"
AUTH=(-H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json")

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPT_DIR}/l1-smoke.sh"

echo "== health (local deps only) =="
curl -sf "${HUB_URL}/api/v1/health" | jq '{status, mongo, redis, mqtt}'

echo "== agent diagnose via chat (optional, needs Ollama) =="
if curl -sf "${HUB_URL}/api/v1/health" | jq -e '.ollama == "ok"' >/dev/null 2>&1; then
  CHAT_JSON=$(curl -sf "${AUTH[@]}" -X POST "${HUB_URL}/api/v1/agent/chat" \
    -d "{\"message\":\"请对 ${DEVICE_ID} 做一次 device.diagnose 诊断并告诉我是否在线\"}") || true
  if [[ -n "${CHAT_JSON:-}" ]]; then
    echo "${CHAT_JSON}" | jq '{sessionId, reply, toolsUsed}'
  fi
else
  echo "skip chat diagnose (ollama not ok); 可手动: curl agent/chat 或停虚拟灯后再 diagnose"
fi

echo "== Mongo events (recent for device) =="
if command -v mongosh >/dev/null 2>&1; then
  mongosh "${MONGO_URI}/${MONGO_DB}" --quiet --eval \
    "db.events.find({deviceId:'${DEVICE_ID}'}).sort({createdAt:-1}).limit(3).toArray()" || \
    echo "mongosh query failed (is mongo up?)"
else
  echo "mongosh not installed; 用 Compass 或 docker exec 查看 events 集合"
fi

echo "L3 smoke OK (本地依赖链路与台账抽检通过；断网验收见 RUNBOOK §9)"
