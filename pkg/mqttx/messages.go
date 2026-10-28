package mqttx

import "encoding/json"

// UpMessage is device state/event uplink (SPEC §3.1).
type UpMessage struct {
	DeviceID string         `json:"deviceId"`
	Ts       int64          `json:"ts"`
	Type     string         `json:"type"`
	Payload  map[string]any `json:"payload"`
}

// DownMessage is control downlink (SPEC §3.1).
type DownMessage struct {
	CmdID          string         `json:"cmdId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Action         string         `json:"action"`
	Params         map[string]any `json:"params"`
	TimeoutMs      int            `json:"timeoutMs"`
}

// AckMessage is command acknowledgement (SPEC §3.1).
type AckMessage struct {
	CmdID          string         `json:"cmdId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	OK             bool           `json:"ok"`
	Code           int            `json:"code"`
	Message        string         `json:"message"`
	Shadow         map[string]any `json:"shadow"`
}

func EncodeUp(m *UpMessage) ([]byte, error) {
	return json.Marshal(m)
}

func DecodeUp(data []byte) (*UpMessage, error) {
	var m UpMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func EncodeDown(m *DownMessage) ([]byte, error) {
	return json.Marshal(m)
}

func DecodeDown(data []byte) (*DownMessage, error) {
	var m DownMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func EncodeAck(m *AckMessage) ([]byte, error) {
	return json.Marshal(m)
}

func DecodeAck(data []byte) (*AckMessage, error) {
	var m AckMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
