#!/usr/bin/env bash
# L2 smoke: agent chat dims living room light (requires L1 stack + Ollama model).
set -euo pipefail

HUB_URL="${HUB_URL:-http://127.0.0.1:8888}"
TOKEN="${TOKEN:-localmind-demo-token}"
DEVICE_ID="${DEVICE_ID:-light-living-01}"
AUTH=(-H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json")

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SCRIPT_DIR}/l1-smoke.sh"

echo "== agent chat: 把客厅灯调到 30% =="
CHAT_JSON=$(curl -sf "${AUTH[@]}" -X POST "${HUB_URL}/api/v1/agent/chat" \
  -d '{"message":"把客厅灯调到 30%"}')
echo "${CHAT_JSON}" | jq .

SESSION_ID=$(echo "${CHAT_JSON}" | jq -r .sessionId)
REPLY=$(echo "${CHAT_JSON}" | jq -r .reply)
if [[ -z "${SESSION_ID}" || "${SESSION_ID}" == "null" ]]; then
  echo "FAIL: no sessionId" >&2
  exit 1
fi
if [[ -z "${REPLY}" || "${REPLY}" == "null" ]]; then
  echo "FAIL: empty reply" >&2
  exit 1
fi

echo "== shadow brightness after agent =="
BR=$(curl -sf "${AUTH[@]}" "${HUB_URL}/api/v1/devices/${DEVICE_ID}" | jq -r '.shadow.brightness')
echo "brightness=${BR}"
if [[ "${BR}" != "30" && "${BR}" != "30.0" ]]; then
  echo "FAIL: expected brightness 30, got ${BR}" >&2
  exit 1
fi

echo "== session messages =="
curl -sf "${AUTH[@]}" "${HUB_URL}/api/v1/agent/sessions/${SESSION_ID}" | jq '.messages | length'

echo "L2 smoke OK"
