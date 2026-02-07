package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Versifine/locus/internal/config"
)

// newTestClient 创建一个指向 mock server 的 Client
func newTestClient(serverURL string) *Client {
	cfg := &config.LLMConfig{
		Model:       "deepseek-chat",
		APIKey:      "test-key",
		Endpoint:    serverURL,
		MaxTokens:   256,
		Temperature: 0.7,
		Timeout:     5,
	}
	return NewLLMClient(cfg)
}

func TestChat(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		wantText   string
		errContain string
	}{
		{
			name: "正常返回",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// 验证请求头
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("Authorization header = %q, 期望 %q", r.Header.Get("Authorization"), "Bearer test-key")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type header = %q, 期望 %q", r.Header.Get("Content-Type"), "application/json")
				}
				// 验证请求体
				var reqBody ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("解析请求体失败: %v", err)
				}
				if reqBody.Model != "deepseek-chat" {
					t.Errorf("Model = %q, 期望 %q", reqBody.Model, "deepseek-chat")
				}
				if len(reqBody.Messages) != 1 || reqBody.Messages[0].Content != "你好" {
					t.Errorf("Messages 内容不符预期: %+v", reqBody.Messages)
				}

				resp := ChatResponse{
					ID:      "chatcmpl-123",
					Object:  "chat.completion",
					Created: 1700000000,
					Choices: []Choice{
						{
							Index:        0,
							Message:      Message{Role: "assistant", Content: "你好！有什么可以帮你的吗？"},
							FinishReason: "stop",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:  false,
			wantText: "你好！有什么可以帮你的吗？",
		},
		{
			name: "多轮对话",
			handler: func(w http.ResponseWriter, r *http.Request) {
				var reqBody ChatRequest
				json.NewDecoder(r.Body).Decode(&reqBody)
				if len(reqBody.Messages) != 3 {
					t.Errorf("期望 3 条消息，实际 %d 条", len(reqBody.Messages))
				}
				resp := ChatResponse{
					Choices: []Choice{
						{Message: Message{Role: "assistant", Content: "钻石在Y=-59最多"}},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:  false,
			wantText: "钻石在Y=-59最多",
		},
		{
			name: "API返回500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal server error"}`))
			},
			wantErr:    true,
			errContain: "status 500",
		},
		{
			name: "API返回401未授权",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid api key"}`))
			},
			wantErr:    true,
			errContain: "status 401",
		},
		{
			name: "返回空choices",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := ChatResponse{
					ID:      "chatcmpl-456",
					Choices: []Choice{},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			errContain: "no choices",
		},
		{
			name: "返回非法JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{invalid json`))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := newTestClient(server.URL)

			var messages []Message
			if tt.name == "多轮对话" {
				messages = []Message{
					{Role: "system", Content: "你是MC助手"},
					{Role: "user", Content: "钻石在哪挖？"},
					{Role: "assistant", Content: "钻石一般在深层"},
				}
			} else {
				messages = []Message{{Role: "user", Content: "你好"}}
			}

			result, err := client.Chat(context.Background(), messages)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望返回错误，实际成功，result=%q", result)
				}
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("错误信息 %q 不包含 %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("期望成功，实际错误: %v", err)
			}
			if result != tt.wantText {
				t.Errorf("回复 = %q, 期望 %q", result, tt.wantText)
			}
		})
	}
}

func TestNewLLMClient_NilConfig(t *testing.T) {
	client := NewLLMClient(nil)
	if client == nil {
		t.Fatal("NewLLMClient(nil) 返回了 nil")
	}
	if client.config.Model != "deepseek-chat" {
		t.Errorf("默认 Model = %q, 期望 %q", client.config.Model, "deepseek-chat")
	}
	if client.config.Timeout != 30 {
		t.Errorf("默认 Timeout = %d, 期望 %d", client.config.Timeout, 30)
	}
}

func TestChat_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不返回，模拟慢响应（但 context 会先取消）
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立刻取消

	_, err := client.Chat(ctx, []Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Fatal("期望 context cancelled 错误，实际成功")
	}
}
