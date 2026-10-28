package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/device"
	mem "github.com/cuteLittleDevil-ai/localmind-hub/pkg/memory/mongo"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
)

type mockDevices struct {
	list []struct {
		id, room, status string
	}
	controlCalls int
	lastParams   map[string]any
	online       bool
}

func (m *mockDevices) ListDevices(ctx context.Context, f sqlite.ListDevicesFilter) ([]sqlite.Device, error) {
	var out []sqlite.Device
	for _, d := range m.list {
		if f.Room != "" && d.room != f.Room {
			continue
		}
		out = append(out, sqlite.Device{DeviceID: d.id, Room: d.room, Status: d.status})
	}
	return out, nil
}

func (m *mockDevices) GetDevice(ctx context.Context, deviceID string) (*sqlite.Device, map[string]any, error) {
	for _, d := range m.list {
		if d.id == deviceID {
			st := d.status
			if m.online {
				st = "online"
			}
			return &sqlite.Device{DeviceID: d.id, Room: d.room, Status: st},
				map[string]any{"brightness": float64(10)}, nil
		}
	}
	return nil, nil, device.ErrDeviceNotFound
}

func (m *mockDevices) Control(ctx context.Context, deviceID string, req *device.ControlRequest) (*device.ControlResult, error) {
	m.controlCalls++
	m.lastParams = req.Params
	if !m.online {
		return &device.ControlResult{OK: false, Message: "offline"}, device.ErrDeviceOffline
	}
	return &device.ControlResult{OK: true, Code: 0, Message: "ok", Shadow: req.Params}, nil
}

func (m *mockDevices) IsOnline(ctx context.Context, deviceID string) bool { return m.online }
func (m *mockDevices) GetShadow(ctx context.Context, deviceID string) (map[string]any, error) {
	return map[string]any{"brightness": float64(10)}, nil
}
func (m *mockDevices) LastSeen(ctx context.Context, deviceID string) (time.Time, bool) {
	if !m.online {
		return time.Now().Add(-2 * time.Minute), true
	}
	return time.Now(), true
}

type mockEvents struct {
	lastDeviceID string
	lastType     string
	lastPayload  map[string]any
}

func (m *mockEvents) InsertEvent(ctx context.Context, deviceID, eventType string, payload map[string]any) (string, error) {
	m.lastDeviceID = deviceID
	m.lastType = eventType
	m.lastPayload = payload
	return "507f1f77bcf86cd799439011", nil
}

func TestRejectUnknownTool(t *testing.T) {
	r := NewRegistry(&mockDevices{online: true}, nil)
	_, err := r.Execute(context.Background(), "shell.exec", `{}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeviceControlSchemaValidation(t *testing.T) {
	m := &mockDevices{online: true, list: []struct{ id, room, status string }{
		{"light-living-01", "living", "online"},
	}}
	r := NewRegistry(m, nil)
	args, _ := json.Marshal(map[string]any{
		"deviceId": "light-living-01",
		"action":   "set",
		"params":   map[string]any{"brightness": 30},
	})
	inv, err := r.Execute(context.Background(), DeviceControl, string(args))
	if err != nil {
		t.Fatal(err)
	}
	if inv.Error != "" {
		t.Fatal(inv)
	}
	if !inv.Result["ok"].(bool) {
		t.Fatal(inv.Result)
	}
	if m.controlCalls != 1 {
		t.Fatal("expected one control")
	}
	b, ok := ParseBrightnessFromParams(m.lastParams)
	if !ok || b != 30 {
		t.Fatalf("params %v", m.lastParams)
	}
}

func TestDeviceControlOfflineReadable(t *testing.T) {
	m := &mockDevices{online: false}
	r := NewRegistry(m, nil)
	args := `{"deviceId":"light-living-01","params":{"brightness":30}}`
	inv, _ := r.Execute(context.Background(), DeviceControl, args)
	if inv.Error == "" || inv.Result["ok"].(bool) {
		t.Fatalf("%+v", inv)
	}
}

func TestValidateToolName(t *testing.T) {
	if ValidateToolName(DeviceQuery) != nil {
		t.Fatal()
	}
	if ValidateToolName("evil") == nil {
		t.Fatal()
	}
}

func TestDeviceDiagnoseWritesOfflineEvent(t *testing.T) {
	ev := &mockEvents{}
	m := &mockDevices{online: false}
	r := NewRegistry(m, ev)
	inv, err := r.Execute(context.Background(), DeviceDiagnose, `{"deviceId":"light-living-01"}`)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Error != "" {
		t.Fatal(inv)
	}
	if ev.lastType != mem.EventTypeOfflineTimeout {
		t.Fatalf("event type %q", ev.lastType)
	}
	if ev.lastDeviceID != "light-living-01" {
		t.Fatal(ev.lastDeviceID)
	}
	if inv.Result["eventId"] == nil || inv.Result["eventId"] == "" {
		t.Fatalf("result %+v", inv.Result)
	}
}

func TestDeviceDiagnoseOnlineWritesDiagnoseEvent(t *testing.T) {
	ev := &mockEvents{}
	r := NewRegistry(&mockDevices{online: true}, ev)
	inv, err := r.Execute(context.Background(), DeviceDiagnose, `{"deviceId":"light-living-01"}`)
	if err != nil {
		t.Fatal(err)
	}
	if ev.lastType != mem.EventTypeDiagnose {
		t.Fatalf("event type %q", ev.lastType)
	}
	if inv.Result["status"] != "online" {
		t.Fatal(inv.Result)
	}
}
