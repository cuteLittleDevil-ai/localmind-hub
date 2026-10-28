package types

type HealthResp struct {
	Status string           `json:"status"`
	Checks map[string]Check `json:"checks"`
}

type Check struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Device API types (SPEC §4.2)

type RegisterDeviceReq struct {
	DeviceID     string         `json:"deviceId"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	Room         string         `json:"room"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

type DeviceSummary struct {
	DeviceID string         `json:"deviceId"`
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Room     string         `json:"room"`
	Status   string         `json:"status"`
	Shadow   map[string]any `json:"shadow,omitempty"`
}

type DeviceDetailResp struct {
	DeviceID     string         `json:"deviceId"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	Room         string         `json:"room"`
	Status       string         `json:"status"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Shadow       map[string]any `json:"shadow"`
}

type ControlReq struct {
	IdempotencyKey string         `json:"idempotencyKey"`
	Action         string         `json:"action"`
	Params         map[string]any `json:"params"`
	TimeoutMs      int            `json:"timeoutMs"`
}

type ControlResp struct {
	CmdID          string         `json:"cmdId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	OK             bool           `json:"ok"`
	Code           int            `json:"code"`
	Message        string         `json:"message"`
	Shadow         map[string]any `json:"shadow,omitempty"`
}

type ListDevicesResp struct {
	Items []DeviceSummary `json:"items"`
}

type MessageResp struct {
	Message string `json:"message"`
}
