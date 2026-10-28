package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/llm"
	mem "github.com/cuteLittleDevil-ai/localmind-hub/pkg/memory/mongo"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/tools"
)

const maxToolRounds = 8

// Config for orchestrator behavior.
type Config struct {
	RequireConfirmForControl bool
}

// Orchestrator runs ReAct loop: LLM + whitelisted tools.
// 编排：系统提示 → Ollama → 白名单 tool loop → 会话与 tool 轨迹落 Mongo。
type Orchestrator struct {
	llm   *llm.Client
	tools *tools.Registry
	mem   *mem.Store
	cfg   Config
}

func NewOrchestrator(llmClient *llm.Client, reg *tools.Registry, memStore *mem.Store, cfg Config) *Orchestrator {
	return &Orchestrator{llm: llmClient, tools: reg, mem: memStore, cfg: cfg}
}

// ChatRequest is agent input.
type ChatRequest struct {
	SessionID        string
	UserID           string
	Message          string
	ConfirmDangerous bool
}

// ChatResult is agent output.
type ChatResult struct {
	SessionID string                `json:"sessionId"`
	Reply     string                `json:"reply"`
	ToolsUsed []tools.ToolInvocation `json:"toolsUsed"`
}

var ErrLLMUnavailable = errors.New("ollama unavailable")

func (o *Orchestrator) Chat(ctx context.Context, req *ChatRequest) (*ChatResult, error) {
	if o.llm == nil {
		return nil, ErrLLMUnavailable
	}
	if err := o.llm.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
	}

	sessionID, err := o.mem.GetOrCreateSession(ctx, req.SessionID, req.UserID)
	if err != nil {
		return nil, err
	}

	_ = o.mem.AppendMessage(ctx, &mem.Message{
		SessionID: sessionID, Role: "user", Content: req.Message,
	})

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt()},
		{Role: "user", Content: req.Message},
	}
	toolDefs := o.tools.LLMToolDefinitions()
	var toolsUsed []tools.ToolInvocation

	for round := 0; round < maxToolRounds; round++ {
		assistant, err := o.llm.ChatCompletion(ctx, messages, toolDefs)
		if err != nil {
			return nil, err
		}

		if len(assistant.ToolCalls) == 0 {
			reply := strings.TrimSpace(assistant.Content)
			if reply == "" {
				reply = "已完成。"
			}
			_ = o.mem.AppendMessage(ctx, &mem.Message{
				SessionID: sessionID, Role: "assistant", Content: reply,
			})
			_ = o.mem.TouchSession(ctx, sessionID)
			return &ChatResult{SessionID: sessionID, Reply: reply, ToolsUsed: toolsUsed}, nil
		}

		messages = append(messages, *assistant)
		_ = o.mem.AppendMessage(ctx, &mem.Message{
			SessionID: sessionID, Role: "assistant", Content: assistant.Content,
		})

		for _, tc := range assistant.ToolCalls {
			if err := tools.ValidateToolName(tc.Function.Name); err != nil {
				return nil, err
			}
			if tc.Function.Name == tools.DeviceControl && o.cfg.RequireConfirmForControl && !req.ConfirmDangerous {
				msg := "控制设备需要 confirmDangerous=true"
				messages = append(messages, llm.Message{
					Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name,
					Content: msg,
				})
				continue
			}

			inv, err := o.tools.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil && inv == nil {
				return nil, err
			}
			if inv != nil {
				toolsUsed = append(toolsUsed, *inv)
				b, _ := json.Marshal(inv.Result)
				toolContent := string(b)
				if inv.Error != "" {
					toolContent = inv.Error + " " + toolContent
				}
				messages = append(messages, llm.Message{
					Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name, Content: toolContent,
				})
				_ = o.mem.AppendMessage(ctx, &mem.Message{
					SessionID: sessionID, Role: "tool", Content: toolContent,
					ToolName: tc.Function.Name, ToolArgs: inv.Args,
				})
			}
		}
	}

	return nil, fmt.Errorf("tool loop exceeded %d rounds", maxToolRounds)
}

func systemPrompt() string {
	return fmt.Sprintf(`你是 LocalMind Hub 的家居助手，只能通过工具操作设备，禁止臆造设备状态。
Demo 设备：deviceId=light-living-01，room=living（客厅灯）。
用户说「客厅灯」时优先使用 light-living-01。
调亮度时使用 device.control，params 如 {"brightness":30}（0-100）。
查询状态用 device.query；设备离线、控灯失败或异常时优先 device.diagnose 并依据 hints 向用户说明。
回复使用简洁中文，并说明实际工具执行结果。相关上下文：%s`, tools.RoomHint())
}
