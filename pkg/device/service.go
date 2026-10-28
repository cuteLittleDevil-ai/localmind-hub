package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/mqttx"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// device 域：SQLite 台账 + Redis 在线/影子 + MQTT 控制闭环（up 合并影子，down 等 ack）。
const (
	redisKeyOnlineFmt      = "device:online:%s"
	redisKeyShadowFmt      = "device:shadow:%s"
	redisKeyLastSeenFmt    = "device:lastseen:%s"
	redisKeyIdempotencyFmt = "idempotency:%s"
	idempotencyTTL         = 10 * time.Minute
	onlineKeyTTL           = 90 * time.Second
	defaultControlTimeout  = 5 * time.Second
)

var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrDeviceOffline  = errors.New("device offline")
	ErrAckTimeout     = errors.New("control ack timeout")
	ErrAckFailed      = errors.New("control failed")
)

// EventRecorder 可选：控制失败等侧效应写入 Mongo events（M3）。
type EventRecorder interface {
	InsertEvent(ctx context.Context, deviceID, eventType string, payload map[string]any) (string, error)
}

// Service implements device domain logic (M1-6).
type Service struct {
	store *sqlite.Store
	rdb   *redis.Client
	mqtt  *mqttx.Client
	events EventRecorder

	mu      sync.Mutex
	pending map[string]chan *mqttx.AckMessage
}

// SetEventRecorder 注入事件写入（Hub 启动 Mongo 就绪后调用）。
func (s *Service) SetEventRecorder(rec EventRecorder) {
	s.events = rec
}

func NewService(store *sqlite.Store, rdb *redis.Client, mqtt *mqttx.Client) *Service {
	s := &Service{
		store:   store,
		rdb:     rdb,
		mqtt:    mqtt,
		pending: make(map[string]chan *mqttx.AckMessage),
	}
	return s
}

// StartMQTTHandlers wires uplink processing.
// 注册 MQTT 上行：up 合并影子写 SQLite/Redis，ack 唤醒控制等待。
func (s *Service) StartMQTTHandlers() {
	s.mqtt.SetHandlers(s.handleUp, s.handleAck)
}

func (s *Service) handleUp(deviceID string, msg *mqttx.UpMessage) {
	ctx := context.Background()
	id := msg.DeviceID
	if id == "" {
		id = deviceID
	}

	merge := map[string]any{}
	for k, v := range msg.Payload {
		merge[k] = v
	}
	status := "online"
	if online, ok := msg.Payload["online"].(bool); ok && !online {
		status = "offline"
	}

	_, err := s.store.GetDeviceByID(id)
	if err != nil {
		// Auto-register unknown devices from uplink (demo path).
		room, _ := msg.Payload["room"].(string)
		_ = s.store.InsertDevice(&sqlite.Device{
			DeviceID: id,
			Type:     "light",
			Name:     id,
			Room:     room,
			Status:   status,
		})
	}

	_ = s.store.UpdateShadow(id, merge, status)

	shJSON, _ := json.Marshal(merge)
	_ = s.rdb.Set(ctx, fmt.Sprintf(redisKeyShadowFmt, id), shJSON, 24*time.Hour).Err()
	_ = s.rdb.Set(ctx, fmt.Sprintf(redisKeyOnlineFmt, id), "1", onlineKeyTTL).Err()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_ = s.rdb.Set(ctx, fmt.Sprintf(redisKeyLastSeenFmt, id), now, 7*24*time.Hour).Err()
}

func (s *Service) handleAck(_ string, msg *mqttx.AckMessage) {
	key := msg.CmdID
	if key == "" {
		key = msg.IdempotencyKey
	}
	s.mu.Lock()
	ch, ok := s.pending[key]
	if ok {
		delete(s.pending, key)
	}
	s.mu.Unlock()
	if ok && ch != nil {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *Service) IsOnline(ctx context.Context, deviceID string) bool {
	v, err := s.rdb.Get(ctx, fmt.Sprintf(redisKeyOnlineFmt, deviceID)).Result()
	return err == nil && v == "1"
}

// LastSeen 返回 Redis 记录的最近一次 MQTT up 时间（无记录则 ok=false）。
func (s *Service) LastSeen(ctx context.Context, deviceID string) (time.Time, bool) {
	v, err := s.rdb.Get(ctx, fmt.Sprintf(redisKeyLastSeenFmt, deviceID)).Result()
	if err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		t, err = time.Parse(time.RFC3339, v)
	}
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (s *Service) recordEvent(ctx context.Context, deviceID, eventType string, payload map[string]any) {
	if s.events == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	_, _ = s.events.InsertEvent(ctx, deviceID, eventType, payload)
}

func (s *Service) GetShadow(ctx context.Context, deviceID string) (map[string]any, error) {
	key := fmt.Sprintf(redisKeyShadowFmt, deviceID)
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			return m, nil
		}
	}
	d, err := s.store.GetDeviceByID(deviceID)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(d.Shadow, &m); err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}

// RegisterDevice creates a device record.
func (s *Service) RegisterDevice(d *sqlite.Device) error {
	return s.store.InsertDevice(d)
}

// ListDevices lists with optional filters; online filter uses Redis when status=online.
func (s *Service) ListDevices(ctx context.Context, f sqlite.ListDevicesFilter) ([]sqlite.Device, error) {
	onlineOnly := f.Status == "online"
	if onlineOnly {
		f.Status = ""
	}
	devices, err := s.store.ListDevices(f)
	if err != nil {
		return nil, err
	}
	if onlineOnly {
		var filtered []sqlite.Device
		for _, d := range devices {
			if s.IsOnline(ctx, d.DeviceID) {
				d.Status = "online"
				filtered = append(filtered, d)
			}
		}
		return filtered, nil
	}
	for i := range devices {
		if s.IsOnline(ctx, devices[i].DeviceID) {
			devices[i].Status = "online"
		}
	}
	return devices, nil
}

// GetDevice returns detail with merged shadow.
func (s *Service) GetDevice(ctx context.Context, deviceID string) (*sqlite.Device, map[string]any, error) {
	d, err := s.store.GetDeviceByID(deviceID)
	if err != nil {
		return nil, nil, ErrDeviceNotFound
	}
	if s.IsOnline(ctx, deviceID) {
		d.Status = "online"
	}
	shadow, _ := s.GetShadow(ctx, deviceID)
	return d, shadow, nil
}

// ControlRequest is HTTP control body (SPEC §4.2).
type ControlRequest struct {
	IdempotencyKey string         `json:"idempotencyKey"`
	Action         string         `json:"action"`
	Params         map[string]any `json:"params"`
	TimeoutMs      int            `json:"timeoutMs"`
}

// ControlResult is API response for control.
type ControlResult struct {
	CmdID          string         `json:"cmdId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	OK             bool           `json:"ok"`
	Code           int            `json:"code"`
	Message        string         `json:"message"`
	Shadow         map[string]any `json:"shadow,omitempty"`
}

type idempotencyRecord struct {
	Status string         `json:"status"`
	Result *ControlResult `json:"result,omitempty"`
}

func (s *Service) Control(ctx context.Context, deviceID string, req *ControlRequest) (*ControlResult, error) {
	if req.Action == "" {
		req.Action = "set"
	}
	timeout := defaultControlTimeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	if _, err := s.store.GetDeviceByID(deviceID); err != nil {
		return nil, ErrDeviceNotFound
	}
	if !s.IsOnline(ctx, deviceID) {
		lastSeen, hasLast := s.LastSeen(ctx, deviceID)
		payload := map[string]any{"reason": "device_offline", "action": req.Action}
		if hasLast {
			payload["lastSeenAt"] = lastSeen.UTC().Format(time.RFC3339)
		}
		s.recordEvent(ctx, deviceID, "control_fail", payload)
		return nil, ErrDeviceOffline
	}

	idemKey := req.IdempotencyKey
	if idemKey == "" {
		idemKey = uuid.NewString()
	}

	redisIdem := fmt.Sprintf(redisKeyIdempotencyFmt, idemKey)
	set, err := s.rdb.SetNX(ctx, redisIdem, `{"status":"processing"}`, idempotencyTTL).Result()
	if err != nil {
		return nil, err
	}
	if !set {
		cached, err := s.rdb.Get(ctx, redisIdem).Bytes()
		if err != nil {
			return nil, fmt.Errorf("idempotency in progress")
		}
		var rec idempotencyRecord
		if json.Unmarshal(cached, &rec) == nil && rec.Result != nil {
			return rec.Result, nil
		}
		return nil, fmt.Errorf("idempotency in progress")
	}

	cmdID := uuid.NewString()
	ch := make(chan *mqttx.AckMessage, 1)
	s.mu.Lock()
	s.pending[cmdID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, cmdID)
		s.mu.Unlock()
	}()

	down := &mqttx.DownMessage{
		CmdID:          cmdID,
		IdempotencyKey: idemKey,
		Action:         req.Action,
		Params:         req.Params,
		TimeoutMs:      int(timeout / time.Millisecond),
	}
	if err := s.mqtt.PublishDown(deviceID, down); err != nil {
		s.saveIdempotency(ctx, redisIdem, &ControlResult{
			CmdID: cmdID, IdempotencyKey: idemKey, OK: false, Code: 500, Message: err.Error(),
		})
		return nil, err
	}

	select {
	case ack := <-ch:
		res := s.ackToResult(ack, cmdID, idemKey)
		if ack.OK {
			_ = s.store.UpdateShadow(deviceID, ack.Shadow, "online")
			shJSON, _ := json.Marshal(ack.Shadow)
			_ = s.rdb.Set(ctx, fmt.Sprintf(redisKeyShadowFmt, deviceID), shJSON, 24*time.Hour).Err()
		} else {
			if res.Message == "" {
				res.Message = ErrAckFailed.Error()
			}
		}
		s.saveIdempotency(ctx, redisIdem, res)
		if !ack.OK {
			return res, ErrAckFailed
		}
		return res, nil
	case <-time.After(timeout):
		res := &ControlResult{
			CmdID: cmdID, IdempotencyKey: idemKey, OK: false, Code: 408, Message: ErrAckTimeout.Error(),
		}
		s.recordEvent(ctx, deviceID, "control_fail", map[string]any{
			"reason": "ack_timeout", "cmdId": cmdID, "timeoutMs": int(timeout / time.Millisecond),
		})
		s.saveIdempotency(ctx, redisIdem, res)
		return res, ErrAckTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) ackToResult(ack *mqttx.AckMessage, cmdID, idemKey string) *ControlResult {
	return &ControlResult{
		CmdID:          cmdID,
		IdempotencyKey: idemKey,
		OK:             ack.OK,
		Code:           ack.Code,
		Message:        ack.Message,
		Shadow:         ack.Shadow,
	}
}

func (s *Service) saveIdempotency(ctx context.Context, key string, res *ControlResult) {
	rec := idempotencyRecord{Status: "done", Result: res}
	b, _ := json.Marshal(rec)
	_ = s.rdb.Set(ctx, key, b, idempotencyTTL).Err()
}

// Unbind marks device unbound and clears online key.
func (s *Service) Unbind(ctx context.Context, deviceID string) error {
	if _, err := s.store.GetDeviceByID(deviceID); err != nil {
		return ErrDeviceNotFound
	}
	_ = s.rdb.Del(ctx, fmt.Sprintf(redisKeyOnlineFmt, deviceID)).Err()
	return s.store.SetUnbound(deviceID)
}
