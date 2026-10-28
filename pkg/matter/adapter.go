package matter

import "context"

// DeviceRef is a minimal Matter-side device reference (placeholder).
type DeviceRef struct {
	ID   string
	Name string
}

// Adapter bridges Home Assistant Matter Server or other backends (MVP: noop).
type Adapter interface {
	ListDevices(ctx context.Context) ([]DeviceRef, error)
}

type noopAdapter struct{}

// NewNoopAdapter returns an empty Matter adapter for MVP.
func NewNoopAdapter() Adapter {
	return &noopAdapter{}
}

func (noopAdapter) ListDevices(context.Context) ([]DeviceRef, error) {
	return nil, nil
}
