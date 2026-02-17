package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Versifine/locus/internal/config"
)

func newToolTestClient(serverURL string) *Client {
	return NewLLMClient(&config.LLMConfig{
		Model:       "deepseek-chat",
		APIKey:      "test-key",
		Endpoint:    serverURL,
		MaxTokens:   256,
		Temperature: 0.2,
		Timeout:     5,
	})
}

func TestCallWithToolsParsesToolCalls(t *testing.T) {
	var gotReq toolChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		resp := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "look",
							"arguments": `{"direction":"forward"}`,
						},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newToolTestClient(server.URL)
	result, err := client.CallWithTools(context.Background(), []ToolMessage{{Role: "user", Content: "hello"}}, []ToolDefinition{{
		Name:        "look",
		Description: "look around",
		Parameters: map[string]any{
			"type": "object",
		},
	}})
	if err != nil {
		t.Fatalf("CallWithTools error: %v", err)
	}

	if len(gotReq.Tools) != 1 {
		t.Fatalf("tools len=%d want 1", len(gotReq.Tools))
	}
	fn, _ := gotReq.Tools[0]["function"].(map[string]any)
	if fn["name"] != "look" {
		t.Fatalf("tool function name=%v want look", fn["name"])
	}

	if result.StopReason != "tool_use" {
		t.Fatalf("stop reason=%q want tool_use", result.StopReason)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len=%d want 1", len(result.Content))
	}
	if result.Content[0].Type != "tool_use" || result.Content[0].Name != "look" {
		t.Fatalf("unexpected content block: %+v", result.Content[0])
	}
	if result.Content[0].Input["direction"] != "forward" {
		t.Fatalf("tool input direction=%v want forward", result.Content[0].Input["direction"])
	}
}

func TestCallWithToolsParsesTextContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
				"finish_reason": "stop",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newToolTestClient(server.URL)
	result, err := client.CallWithTools(context.Background(), []ToolMessage{{Role: "user", Content: "hello"}}, nil)
	if err != nil {
		t.Fatalf("CallWithTools error: %v", err)
	}

	if result.StopReason != "end_turn" {
		t.Fatalf("stop reason=%q want end_turn", result.StopReason)
	}
	if len(result.Content) != 1 || result.Content[0].Type != "text" || result.Content[0].Text != "done" {
		t.Fatalf("unexpected content: %+v", result.Content)
	}
}

func TestCallWithToolsNoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	client := newToolTestClient(server.URL)
	_, err := client.CallWithTools(context.Background(), []ToolMessage{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected error when choices is empty")
	}
}
