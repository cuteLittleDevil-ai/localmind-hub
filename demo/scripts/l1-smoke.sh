#!/usr/bin/env bash
# L1 smoke: register device, list, control brightness (requires compose deps + hub + virtual-light).
set -euo pipefail

HUB_URL="${HUB_URL:-http://127.0.0.1:8888}"
TOKEN="${TOKEN:-localmind-demo-token}"
DEVICE_ID="${DEVICE_ID:-light-living-01}"
AUTH=(-H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json")

echo "== register device =="
REG_HTTP=$(curl -sS -w "%{http_code}" -o /tmp/l1-register.json "${AUTH[@]}" -X POST "${HUB_URL}/api/v1/devices" -d "{
  \"deviceId\": \"${DEVICE_ID}\",
  \"type\": \"light\",
  \"name\": \"客厅灯\",
  \"room\": \"living\"
}")
if [[ "${REG_HTTP}" == "409" ]]; then
  echo "device already registered (409), continuing"
  jq . /tmp/l1-register.json 2>/dev/null || true
elif [[ "${REG_HTTP}" =~ ^2 ]]; then
  jq . /tmp/l1-register.json
else
  echo "register failed HTTP ${REG_HTTP}" >&2
  cat /tmp/l1-register.json >&2
  exit 1
fi

echo "== wait for online uplink =="
for i in $(seq 1 20); do
  STATUS=$(curl -sf "${AUTH[@]}" "${HUB_URL}/api/v1/devices/${DEVICE_ID}" | jq -r .status)
  if [[ "${STATUS}" == "online" ]]; then
    break
  fi
  sleep 1
done

echo "== list devices (room=living) =="
curl -sf "${AUTH[@]}" "${HUB_URL}/api/v1/devices?room=living" | jq .

BR=$(curl -sf "${AUTH[@]}" "${HUB_URL}/api/v1/devices/${DEVICE_ID}" | jq -r '.shadow.brightness')
echo "current brightness=${BR}"

echo "== control brightness=30 =="
curl -sf "${AUTH[@]}" -X POST "${HUB_URL}/api/v1/devices/${DEVICE_ID}/control" -d '{
  "idempotencyKey": "l1-demo-1",
  "action": "set",
  "params": { "brightness": 30 },
  "timeoutMs": 5000
}' | jq .

BR2=$(curl -sf "${AUTH[@]}" "${HUB_URL}/api/v1/devices/${DEVICE_ID}" | jq -r '.shadow.brightness')
echo "shadow brightness after control=${BR2}"
if [[ "${BR2}" != "30" && "${BR2}" != "30.0" ]]; then
  echo "FAIL: expected brightness 30, got ${BR2}" >&2
  exit 1
fi
echo "L1 smoke OK"
