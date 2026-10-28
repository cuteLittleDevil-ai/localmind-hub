package handler

import (
	"net/http"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/logic"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func healthHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewHealthLogic(r.Context(), svcCtx)
		resp, err := l.Health()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		code := http.StatusOK
		if resp.Status != "ok" {
			code = http.StatusServiceUnavailable
		}
		httpx.WriteJson(w, code, resp)
	}
}
