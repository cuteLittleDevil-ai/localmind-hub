package handler

import (
	"net/http"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/logic"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func chatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewChatLogic(r.Context(), svcCtx)
		resp, code, err := l.Chat(&req)
		if err != nil {
			httpx.WriteJson(w, code, map[string]string{"message": err.Error()})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func getSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := pathvar.Vars(r)["sessionId"]
		l := logic.NewGetSessionLogic(r.Context(), svcCtx)
		resp, code, err := l.Get(sessionID)
		if err != nil {
			httpx.WriteJson(w, code, map[string]string{"message": err.Error()})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
