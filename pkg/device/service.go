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

const (
	redisKeyOnlineFmt      = "device:online:%s"
	redisKeyShadowFmt      = "device:shadow:%s"
	redisKeyIdempotencyFmt = "idempotency:%s"
	idempotencyTTL         = 10 * time.Minute
	defaultControlTimeout  = 5 * time.Second
)

var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrDeviceOffline  = errors.New("device offline")
	ErrAckTimeout     = errors.New("control ack timeout")
	ErrAckFailed      = errors.New("control failed")
)

// Service implements device domain logic (M1-6).
type Service struct {
	store *sqlite.Store
	rdb   *redis.Client
	mqtt  *mqttx.Client

	mu      sync.Mutex
	pending map[string]chan *mqttx.AckMessage
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
	_ = s.rdb.Set(ctx, fmt.Sprintf(redisKeyOnlineFmt, id), "1", 90*time.Second).Err()
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
