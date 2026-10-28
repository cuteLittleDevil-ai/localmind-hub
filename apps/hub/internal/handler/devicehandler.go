package handler

import (
	"net/http"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/logic"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func listDevicesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewListDevicesLogic(r.Context(), svcCtx)
		resp, err := l.List(r.URL.Query().Get("room"), r.URL.Query().Get("type"), r.URL.Query().Get("status"))
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func getDeviceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := pathvar.Vars(r)["deviceId"]
		l := logic.NewGetDeviceLogic(r.Context(), svcCtx)
		resp, err := l.Get(deviceID)
		if err != nil {
			httpx.WriteJson(w, logic.StatusFromError(err), map[string]string{"message": err.Error()})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func registerDeviceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RegisterDeviceReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewRegisterDeviceLogic(r.Context(), svcCtx)
		resp, err := l.Register(&req)
		if err != nil {
			httpx.WriteJson(w, logic.StatusFromError(err), map[string]string{"message": err.Error()})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func controlDeviceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := pathvar.Vars(r)["deviceId"]
		var req types.ControlReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewControlDeviceLogic(r.Context(), svcCtx)
		resp, code, err := l.Control(deviceID, &req)
		if err != nil && resp == nil {
			httpx.WriteJson(w, code, map[string]string{"message": err.Error()})
			return
		}
		if code != http.StatusOK {
			httpx.WriteJson(w, code, resp)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func unbindDeviceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := pathvar.Vars(r)["deviceId"]
		l := logic.NewUnbindDeviceLogic(r.Context(), svcCtx)
		if err := l.Unbind(deviceID); err != nil {
			httpx.WriteJson(w, logic.StatusFromError(err), map[string]string{"message": err.Error()})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, types.MessageResp{Message: "unbound"})
	}
}
