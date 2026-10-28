package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/device"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/llm"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
)

const (
	DeviceQuery    = "device.query"
	DeviceControl  = "device.control"
	DeviceDiagnose = "device.diagnose"
)

var allowedTools = map[string]bool{
	DeviceQuery: true, DeviceControl: true, DeviceDiagnose: true,
}

// DeviceAPI is the subset of device.Service used by tools (for tests).
type DeviceAPI interface {
	ListDevices(ctx context.Context, f sqlite.ListDevicesFilter) ([]sqlite.Device, error)
	GetDevice(ctx context.Context, deviceID string) (*sqlite.Device, map[string]any, error)
	Control(ctx context.Context, deviceID string, req *device.ControlRequest) (*device.ControlResult, error)
	IsOnline(ctx context.Context, deviceID string) bool
	GetShadow(ctx context.Context, deviceID string) (map[string]any, error)
}

// Registry executes whitelisted tools via device domain (no direct MQTT).
type Registry struct {
	devices DeviceAPI
}

func NewRegistry(devices DeviceAPI) *Registry {
	return &Registry{devices: devices}
}

// LLMToolDefinitions returns OpenAI tool schemas.
func (r *Registry) LLMToolDefinitions() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Type: "function",
			Function: llm.FunctionSpec{
				Name:        DeviceQuery,
				Description: "查询设备列表或单个设备详情与影子。只读。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"deviceId": map[string]any{"type": "string", "description": "设备 ID，如 light-living-01"},
						"room":     map[string]any{"type": "string", "description": "按房间过滤，如 living"},
						"type":     map[string]any{"type": "string", "description": "设备类型，如 light"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: llm.FunctionSpec{
				Name:        DeviceControl,
				Description: "控制设备，例如设置亮度。会通过 Hub 下发 MQTT 并等待 ack。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"deviceId":       map[string]any{"type": "string"},
						"action":         map[string]any{"type": "string", "description": "默认 set"},
						"params":         map[string]any{"type": "object"},
						"idempotencyKey": map[string]any{"type": "string"},
					},
					"required": []string{"deviceId", "params"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.FunctionSpec{
				Name:        DeviceDiagnose,
				Description: "诊断设备在线状态与影子摘要。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"deviceId": map[string]any{"type": "string"},
					},
					"required": []string{"deviceId"},
				},
			},
		},
	}
}

// ToolInvocation records one tool run for API response.
type ToolInvocation struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
	Error  string         `json:"error,omitempty"`
}

// Execute runs a whitelisted tool after schema validation.
func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (*ToolInvocation, error) {
	if !allowedTools[name] {
		return nil, fmt.Errorf("tool not allowed: %s", name)
	}
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("invalid tool args json: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	inv := &ToolInvocation{Name: name, Args: args}
	var result map[string]any
	var err error

	switch name {
	case DeviceQuery:
		result, err = r.runQuery(ctx, args)
	case DeviceControl:
		result, err = r.runControl(ctx, args)
	case DeviceDiagnose:
		result, err = r.runDiagnose(ctx, args)
	default:
		err = fmt.Errorf("unknown tool")
	}

	if err != nil {
		inv.Error = err.Error()
		inv.Result = map[string]any{"ok": false, "message": err.Error()}
	} else {
		inv.Result = result
	}
	return inv, nil
}

func (r *Registry) runQuery(ctx context.Context, args map[string]any) (map[string]any, error) {
	deviceID, _ := args["deviceId"].(string)
	if deviceID != "" {
		d, shadow, err := r.devices.GetDevice(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"ok": true,
			"device": map[string]any{
				"deviceId": d.DeviceID, "name": d.Name, "room": d.Room,
				"type": d.Type, "status": d.Status, "shadow": shadow,
			},
		}, nil
	}
	room, _ := args["room"].(string)
	typ, _ := args["type"].(string)
	devices, err := r.devices.ListDevices(ctx, sqlite.ListDevicesFilter{Room: room, Type: typ})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(devices))
	for _, d := range devices {
		sh, _ := r.devices.GetShadow(ctx, d.DeviceID)
		items = append(items, map[string]any{
			"deviceId": d.DeviceID, "name": d.Name, "room": d.Room,
			"status": d.Status, "shadow": sh,
		})
	}
	return map[string]any{"ok": true, "items": items}, nil
}

func (r *Registry) runControl(ctx context.Context, args map[string]any) (map[string]any, error) {
	deviceID, _ := args["deviceId"].(string)
	if deviceID == "" {
		return nil, errors.New("deviceId required")
	}
	params, ok := args["params"].(map[string]any)
	if !ok || len(params) == 0 {
		return nil, errors.New("params required")
	}
	action, _ := args["action"].(string)
	idem, _ := args["idempotencyKey"].(string)

	res, err := r.devices.Control(ctx, deviceID, &device.ControlRequest{
		IdempotencyKey: idem,
		Action:         action,
		Params:         params,
		TimeoutMs:      5000,
	})
	out := map[string]any{
		"ok": false, "deviceId": deviceID,
	}
	if res != nil {
		out["cmdId"] = res.CmdID
		out["ok"] = res.OK
		out["code"] = res.Code
		out["message"] = res.Message
		out["shadow"] = res.Shadow
	}
	if err != nil {
		out["message"] = err.Error()
		if errors.Is(err, device.ErrDeviceOffline) {
			return out, fmt.Errorf("device offline: %s", deviceID)
		}
		if res != nil && res.Message != "" {
			return out, fmt.Errorf("%s", res.Message)
		}
		return out, err
	}
	return out, nil
}

func (r *Registry) runDiagnose(ctx context.Context, args map[string]any) (map[string]any, error) {
	deviceID, _ := args["deviceId"].(string)
	if deviceID == "" {
		return nil, errors.New("deviceId required")
	}
	online := r.devices.IsOnline(ctx, deviceID)
	sh, err := r.devices.GetShadow(ctx, deviceID)
	if err != nil && !errors.Is(err, device.ErrDeviceNotFound) {
		return nil, err
	}
	status := "offline"
	if online {
		status = "online"
	}
	hints := []string{}
	if !online {
		hints = append(hints, "设备当前不在线，请确认虚拟灯或真实设备已连接 MQTT")
	}
	return map[string]any{
		"ok": true, "deviceId": deviceID, "online": online, "status": status,
		"shadow": sh, "hints": hints,
	}, nil
}

// ValidateToolName ensures LLM cannot call arbitrary tools.
func ValidateToolName(name string) error {
	if !allowedTools[name] {
		return fmt.Errorf("tool not in whitelist: %s", name)
	}
	return nil
}

// ParseBrightnessFromParams helper for tests.
func ParseBrightnessFromParams(params map[string]any) (int, bool) {
	v, ok := params["brightness"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, _ := n.Int64()
		return int(i), true
	default:
		return 0, false
	}
}

// RoomHint returns demo room name for system prompt.
func RoomHint() string {
	return strings.Join([]string{"living", "light-living-01", "客厅灯"}, ", ")
}
