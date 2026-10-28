package mqttx

import (
	"encoding/json"
	"testing"
)

func TestDownAckRoundTrip(t *testing.T) {
	down := &DownMessage{
		CmdID:          "cmd-1",
		IdempotencyKey: "key-1",
		Action:         "set",
		Params:         map[string]any{"brightness": float64(30)},
		TimeoutMs:      5000,
	}
	b, err := EncodeDown(down)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeDown(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.CmdID != down.CmdID || got.Action != "set" {
		t.Fatalf("decode down: %+v", got)
	}
}

func TestUpDecodeSpecShape(t *testing.T) {
	raw := `{"deviceId":"light-living-01","ts":1730000000000,"type":"shadow","payload":{"online":true,"brightness":30,"room":"living"}}`
	up, err := DecodeUp([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if up.DeviceID != "light-living-01" {
		t.Fatal(up)
	}
	if up.Payload["brightness"].(float64) != 30 {
		t.Fatal(up.Payload)
	}
}

func TestAckJSONFields(t *testing.T) {
	ack := &AckMessage{
		CmdID: "u", IdempotencyKey: "k", OK: true, Code: 0, Message: "ok",
		Shadow: map[string]any{"brightness": 30},
	}
	b, _ := EncodeAck(ack)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["ok"] != true {
		t.Fatal(m)
	}
}

func TestParseDeviceFromTopic(t *testing.T) {
	id, kind, ok := ParseDeviceFromTopic("localmind/light-living-01/up")
	if !ok || id != "light-living-01" || kind != "up" {
		t.Fatal(id, kind, ok)
	}
}
