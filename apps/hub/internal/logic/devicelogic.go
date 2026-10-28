package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/types"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/device"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
)

type ListDevicesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDevicesLogic {
	return &ListDevicesLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ListDevicesLogic) List(room, typ, status string) (*types.ListDevicesResp, error) {
	devices, err := l.svcCtx.Devices.ListDevices(l.ctx, sqlite.ListDevicesFilter{
		Room: room, Type: typ, Status: status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DeviceSummary, 0, len(devices))
	for _, d := range devices {
		shadow, _ := l.svcCtx.Devices.GetShadow(l.ctx, d.DeviceID)
		items = append(items, types.DeviceSummary{
			DeviceID: d.DeviceID,
			Type:     d.Type,
			Name:     d.Name,
			Room:     d.Room,
			Status:   d.Status,
			Shadow:   shadow,
		})
	}
	return &types.ListDevicesResp{Items: items}, nil
}

type GetDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceLogic {
	return &GetDeviceLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *GetDeviceLogic) Get(deviceID string) (*types.DeviceDetailResp, error) {
	d, shadow, err := l.svcCtx.Devices.GetDevice(l.ctx, deviceID)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			return nil, errNotFound(err)
		}
		return nil, err
	}
	var caps map[string]any
	_ = json.Unmarshal(d.Capabilities, &caps)
	return &types.DeviceDetailResp{
		DeviceID: d.DeviceID, Type: d.Type, Name: d.Name, Room: d.Room,
		Status: d.Status, Capabilities: caps, Shadow: shadow,
	}, nil
}

type RegisterDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterDeviceLogic {
	return &RegisterDeviceLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *RegisterDeviceLogic) Register(req *types.RegisterDeviceReq) (*types.DeviceDetailResp, error) {
	if req.DeviceID == "" {
		return nil, errBadRequest("deviceId required")
	}
	capJSON, _ := json.Marshal(req.Capabilities)
	d := &sqlite.Device{
		DeviceID: req.DeviceID,
		Type:     req.Type,
		Name:     req.Name,
		Room:     req.Room,
		Capabilities: capJSON,
		Status:   "offline",
	}
	if d.Type == "" {
		d.Type = "light"
	}
	if d.Name == "" {
		d.Name = req.DeviceID
	}
	if err := l.svcCtx.Devices.RegisterDevice(d); err != nil {
		return nil, errConflict("device already registered")
	}
	shadow, _ := l.svcCtx.Devices.GetShadow(l.ctx, d.DeviceID)
	return &types.DeviceDetailResp{
		DeviceID: d.DeviceID, Type: d.Type, Name: d.Name, Room: d.Room,
		Status: d.Status, Shadow: shadow,
	}, nil
}

type ControlDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewControlDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ControlDeviceLogic {
	return &ControlDeviceLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ControlDeviceLogic) Control(deviceID string, req *types.ControlReq) (*types.ControlResp, int, error) {
	res, err := l.svcCtx.Devices.Control(l.ctx, deviceID, &device.ControlRequest{
		IdempotencyKey: req.IdempotencyKey,
		Action:         req.Action,
		Params:         req.Params,
		TimeoutMs:      req.TimeoutMs,
	})
	if res != nil {
		out := &types.ControlResp{
			CmdID: res.CmdID, IdempotencyKey: res.IdempotencyKey,
			OK: res.OK, Code: res.Code, Message: res.Message, Shadow: res.Shadow,
		}
		if err != nil {
			switch {
			case errors.Is(err, device.ErrDeviceNotFound):
				return out, http.StatusNotFound, errNotFound(err)
			case errors.Is(err, device.ErrDeviceOffline):
				return out, http.StatusConflict, err
			case errors.Is(err, device.ErrAckTimeout):
				return out, http.StatusGatewayTimeout, err
			case errors.Is(err, device.ErrAckFailed):
				return out, http.StatusBadGateway, err
			default:
				return out, http.StatusInternalServerError, err
			}
		}
		return out, http.StatusOK, nil
	}
	if errors.Is(err, device.ErrDeviceNotFound) {
		return nil, http.StatusNotFound, errNotFound(err)
	}
	if errors.Is(err, device.ErrDeviceOffline) {
		return nil, http.StatusConflict, err
	}
	return nil, http.StatusInternalServerError, err
}

type UnbindDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnbindDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindDeviceLogic {
	return &UnbindDeviceLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *UnbindDeviceLogic) Unbind(deviceID string) error {
	err := l.svcCtx.Devices.Unbind(l.ctx, deviceID)
	if errors.Is(err, device.ErrDeviceNotFound) {
		return errNotFound(err)
	}
	return err
}

type apiError struct {
	status  int
	message string
}

func (e *apiError) Error() string { return e.message }

func errNotFound(err error) error {
	return &apiError{status: http.StatusNotFound, message: err.Error()}
}

func errBadRequest(msg string) error {
	return &apiError{status: http.StatusBadRequest, message: msg}
}

func errConflict(msg string) error {
	return &apiError{status: http.StatusConflict, message: msg}
}

func StatusFromError(err error) int {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.status
	}
	if errors.Is(err, sql.ErrNoRows) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
