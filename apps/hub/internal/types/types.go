package types

type HealthResp struct {
	Status string           `json:"status"`
	Checks map[string]Check `json:"checks"`
}

type Check struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}
