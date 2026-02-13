package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

func TestNewAgent(t *testing.T) {
	bus := event.NewBus()
	a := NewAgent(bus, nil, nil, nil)

	if a == nil {
		t.Fatal("NewAgent() returned nil")
	}
	if a.bus != bus {
		t.Error("agent bus should match injected bus")
	}
	if a.maxHistory != defaultMaxHistory {
		t.Fatalf("maxHistory = %d, want %d", a.maxHistory, defaultMaxHistory)
	}
}

func TestNewAgentSubscribesChat(t *testing.T) {
	bus := event.NewBus()
	_ = NewAgent(bus, nil, nil, nil)

	var called atomic.Bool
	bus.Subscribe(event.EventChat, func(any) {
		called.Store(true)
	})

	bus.Publish(event.EventChat, "test-data")
	time.Sleep(100 * time.Millisecond)

	if !called.Load() {
		t.Fatal("chat event should trigger handlers")
	}
}

type mockSender struct {
	sentMessages []string
	mu           sync.Mutex
}

func (m *mockSender) SendMsgToServer(message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, message)
	return nil
}

func (m *mockSender) messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.sentMessages))
	copy(out, m.sentMessages)
	return out
}

type mockStateProvider struct {
	snapshot world.Snapshot
}

func (m *mockStateProvider) GetState() world.Snapshot {
	return m.snapshot
}

func TestSplitByRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected []string
	}{
		{"simple", "1234567890", 3, []string{"123", "456", "789", "0"}},
		{"chinese", "你好世界", 2, []string{"你好", "世界"}},
		{"emoji", "😀😀😀", 1, []string{"😀", "😀", "😀"}},
		{"limit_greater", "hello", 10, []string{"hello"}},
		{"empty", "", 5, nil},
		{"limit_zero", "hello", 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := SplitByRunes(tt.input, tt.limit)
			if len(chunks) != len(tt.expected) {
				t.Fatalf("len = %d, want %d", len(chunks), len(tt.expected))
			}
			for i := range chunks {
				if chunks[i] != tt.expected[i] {
					t.Fatalf("chunks[%d] = %q, want %q", i, chunks[i], tt.expected[i])
				}
			}
		})
	}
}

func TestExtractMemory(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantReply  string
		wantMemory string
	}{
		{
			name:       "normal_extract",
			response:   "我在主城。\n<memory>聊了当前位置</memory>",
			wantReply:  "我在主城。",
			wantMemory: "聊了当前位置",
		},
		{
			name:       "missing_tag",
			response:   "我看不到附近玩家。",
			wantReply:  "我看不到附近玩家。",
			wantMemory: "",
		},
		{
			name:       "multiple_tags_use_last",
			response:   "答复\n<memory>旧摘要</memory>\n<memory>新摘要</memory>",
			wantReply:  "答复",
			wantMemory: "新摘要",
		},
		{
			name:       "memory_with_newline",
			response:   "收到。<memory>第一行\n第二行</memory>",
			wantReply:  "收到。",
			wantMemory: "第一行 第二行",
		},
		{
			name:       "broken_open_tag",
			response:   "收到<memory>不完整摘要",
			wantReply:  "收到",
			wantMemory: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply, memory := extractMemory(tt.response)
			if reply != tt.wantReply {
				t.Fatalf("reply = %q, want %q", reply, tt.wantReply)
			}
			if memory != tt.wantMemory {
				t.Fatalf("memory = %q, want %q", memory, tt.wantMemory)
			}
		})
	}
}

func TestBuildMessages_UsesRecentMemoryAndState(t *testing.T) {
	memory := []string{"  问了我在哪  ", "\n我回答在村庄\n"}

	msgs := buildMessages("sys", memory, "state-snapshot", "现在几点")
	if len(msgs) != 6 {
		t.Fatalf("len(msgs) = %d, want 6", len(msgs))
	}

	if msgs[0].Role != "system" || msgs[0].Content != "sys" {
		t.Fatalf("msgs[0] = %+v, want system prompt", msgs[0])
	}
	if msgs[1].Role != "system" || msgs[1].Content != "[以下是历史对话记录，其中涉及的位置、实体、状态等信息均已过时，仅供了解对话上下文]" {
		t.Fatalf("msgs[1] = %+v", msgs[1])
	}
	if msgs[2].Role != "system" {
		t.Fatalf("msgs[2].Role = %q, want system", msgs[2].Role)
	}
	if msgs[2].Content != "[近期记忆]\n- 问了我在哪\n- 我回答在村庄" {
		t.Fatalf("msgs[2].Content = %q", msgs[2].Content)
	}
	if msgs[3].Role != "system" || msgs[3].Content != "[历史记录结束。以下是当前真实状态，以此为准]" {
		t.Fatalf("msgs[3] = %+v", msgs[3])
	}
	if msgs[4].Role != "system" {
		t.Fatalf("msgs[4].Role = %q, want system", msgs[4].Role)
	}
	if want := "[当前状态（唯一可信数据源）]"; len(msgs[4].Content) < len(want) || msgs[4].Content[:len(want)] != want {
		t.Fatalf("msgs[4].Content should start with %q, got %q", want, msgs[4].Content)
	}
	if msgs[5].Role != "user" || msgs[5].Content != "现在几点" {
		t.Fatalf("msgs[5] = %+v, want user question", msgs[5])
	}
}

func TestBuildMessages_WithoutMemory(t *testing.T) {
	msgs := buildMessages("sys", nil, "state", "你好")
	if len(msgs) != 3 {
		t.Fatalf("len(msgs) = %d, want 3", len(msgs))
	}
	if msgs[1].Role != "system" {
		t.Fatalf("msgs[1].Role = %q, want system", msgs[1].Role)
	}
	if msgs[2].Role != "user" || msgs[2].Content != "你好" {
		t.Fatalf("msgs[2] = %+v, want user message", msgs[2])
	}
}

func TestAppendMemorySlidingWindow(t *testing.T) {
	a := &Agent{
		maxHistory: 2,
		memory:     make(map[protocol.UUID][]string),
	}
	uuid := protocol.GenerateOfflineUUID("tester")

	a.appendMemory(uuid, " 第一条 ")
	a.appendMemory(uuid, "第二条")
	a.appendMemory(uuid, "第三条")

	got := a.getMemory(uuid)
	want := []string{"第二条", "第三条"}
	if len(got) != len(want) {
		t.Fatalf("len(memory) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("memory[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHandleSourcePlayer_StripsMemoryTagAndStoresSummary(t *testing.T) {
	var requestPayload llm.ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		resp := llm.ChatResponse{
			Choices: []llm.Choice{{
				Message: llm.Message{Role: "assistant", Content: "你在主城。<memory>聊了玩家位置</memory>"},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := llm.NewLLMClient(&config.LLMConfig{
		Model:        "deepseek-chat",
		APIKey:       "test-key",
		Endpoint:     server.URL,
		SystemPrompt: "system",
		MaxTokens:    64,
		Temperature:  0.1,
		Timeout:      5,
		MaxHistory:   8,
	})

	sender := &mockSender{}
	provider := &mockStateProvider{snapshot: world.Snapshot{}}
	a := NewAgent(event.NewBus(), sender, provider, client)

	uuid := protocol.GenerateOfflineUUID("steve")
	evt := event.NewChatEvent(context.Background(), "Steve", uuid, "我在哪", event.SourcePlayer)
	a.handleSourcePlayer(evt)

	msgs := sender.messages()
	if len(msgs) != 1 {
		t.Fatalf("sender message count = %d, want 1", len(msgs))
	}
	if msgs[0] != "你在主城。" {
		t.Fatalf("sender message = %q, want %q", msgs[0], "你在主城。")
	}

	memory := a.getMemory(uuid)
	if len(memory) != 1 {
		t.Fatalf("memory count = %d, want 1", len(memory))
	}
	if memory[0] != "聊了玩家位置" {
		t.Fatalf("memory[0] = %q, want %q", memory[0], "聊了玩家位置")
	}

	if len(requestPayload.Messages) == 0 {
		t.Fatal("llm request messages should not be empty")
	}
	if requestPayload.Messages[len(requestPayload.Messages)-1].Role != "user" {
		t.Fatalf("last message role = %q, want user", requestPayload.Messages[len(requestPayload.Messages)-1].Role)
	}
}
