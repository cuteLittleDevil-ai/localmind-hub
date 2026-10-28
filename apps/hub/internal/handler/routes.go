package handler

import (
	"net/http"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/health",
				Handler: healthHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/devices",
				Handler: listDevicesHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/devices/:deviceId",
				Handler: getDeviceHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/devices",
				Handler: registerDeviceHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/devices/:deviceId/control",
				Handler: controlDeviceHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/devices/:deviceId/unbind",
				Handler: unbindDeviceHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/agent/chat",
				Handler: chatHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/agent/sessions/:sessionId",
				Handler: getSessionHandler(serverCtx),
			},
		},
	)
}
