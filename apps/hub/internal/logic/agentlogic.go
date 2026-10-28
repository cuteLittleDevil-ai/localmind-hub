package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/types"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/agent"
	"go.mongodb.org/mongo-driver/mongo"
)

type ChatLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatLogic {
	return &ChatLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatLogic) Chat(req *types.ChatReq) (*types.ChatResp, int, error) {
	if req.Message == "" {
		return nil, http.StatusBadRequest, errBadRequest("message required")
	}
	if l.svcCtx.Agent == nil {
		return nil, http.StatusServiceUnavailable, errServiceUnavailable("agent unavailable: mongo not connected")
	}

	res, err := l.svcCtx.Agent.Chat(l.ctx, &agent.ChatRequest{
		SessionID:        req.SessionID,
		UserID:           "demo",
		Message:          req.Message,
		ConfirmDangerous: req.ConfirmDangerous,
	})
	if err != nil {
		if errors.Is(err, agent.ErrLLMUnavailable) {
			return nil, http.StatusServiceUnavailable, errServiceUnavailable(err.Error())
		}
		return nil, http.StatusBadGateway, err
	}

	toolsUsed := make([]map[string]any, 0, len(res.ToolsUsed))
	for _, t := range res.ToolsUsed {
		toolsUsed = append(toolsUsed, map[string]any{
			"name": t.Name, "args": t.Args, "result": t.Result, "error": t.Error,
		})
	}
	return &types.ChatResp{
		SessionID: res.SessionID,
		Reply:     res.Reply,
		ToolsUsed: toolsUsed,
	}, http.StatusOK, nil
}

type GetSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSessionLogic {
	return &GetSessionLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *GetSessionLogic) Get(sessionID string) (*types.SessionResp, int, error) {
	if l.svcCtx.Memory == nil {
		return nil, http.StatusServiceUnavailable, errServiceUnavailable("memory unavailable")
	}
	sess, err := l.svcCtx.Memory.GetSession(l.ctx, sessionID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, http.StatusNotFound, errNotFound(err)
		}
		return nil, http.StatusInternalServerError, err
	}
	msgs, err := l.svcCtx.Memory.ListMessages(l.ctx, sessionID, 200)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	out := make([]types.SessionMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, types.SessionMessage{
			Role: m.Role, Content: m.Content, ToolName: m.ToolName,
			ToolArgs: m.ToolArgs, CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	return &types.SessionResp{
		SessionID: sess.SessionID, UserID: sess.UserID, Messages: out,
	}, http.StatusOK, nil
}

func errServiceUnavailable(msg string) error {
	return &apiError{status: http.StatusServiceUnavailable, message: msg}
}
