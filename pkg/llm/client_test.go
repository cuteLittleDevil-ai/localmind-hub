package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatCompletionToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{
							map[string]any{
								"id": "call_1", "type": "function",
								"function": map[string]any{
									"name": "device.control",
									"arguments": `{"deviceId":"light-living-01","action":"set","params":{"brightness":30}}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL + "/v1", Model: "test"})
	msg, err := c.ChatCompletion(context.Background(), []Message{
		{Role: "user", Content: "把客厅灯调到30%"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "device.control" {
		t.Fatalf("%+v", msg)
	}
}
