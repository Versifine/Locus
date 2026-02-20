package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/world"
)

type fakeThinkerClient struct {
	mu        sync.Mutex
	responses []llm.ToolResponse
	errors    []error
	calls     int
	config    config.LLMConfig
	blockFn   func(ctx context.Context) error
	inspectFn func(call int, messages []llm.ToolMessage, tools []llm.ToolDefinition) error
}

func (f *fakeThinkerClient) CallWithTools(ctx context.Context, messages []llm.ToolMessage, tools []llm.ToolDefinition) (llm.ToolResponse, error) {
	f.mu.Lock()
	f.calls++
	idx := f.calls - 1
	blockFn := f.blockFn
	inspectFn := f.inspectFn
	f.mu.Unlock()

	if inspectFn != nil {
		if err := inspectFn(idx+1, messages, tools); err != nil {
			return llm.ToolResponse{}, err
		}
	}

	if blockFn != nil {
		if err := blockFn(ctx); err != nil {
			return llm.ToolResponse{}, err
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if idx < len(f.errors) && f.errors[idx] != nil {
		return llm.ToolResponse{}, f.errors[idx]
	}
	if idx < len(f.responses) {
		return f.responses[idx], nil
	}
	return llm.ToolResponse{StopReason: "end_turn", Content: []llm.ToolContentBlock{{Type: "text", Text: "done"}}}, nil
}

func (f *fakeThinkerClient) Config() config.LLMConfig {
	return f.config
}

func TestThinkerToolUseLoop(t *testing.T) {
	client := &fakeThinkerClient{
		config: config.LLMConfig{SystemPrompt: "test"},
		responses: []llm.ToolResponse{
			{
				StopReason: "tool_use",
				Content: []llm.ToolContentBlock{{
					Type:  "tool_use",
					ID:    "call_1",
					Name:  "speak",
					Input: map[string]any{"message": "hello"},
				}},
			},
			{
				StopReason: "end_turn",
				Content:    []llm.ToolContentBlock{{Type: "text", Text: "done"}},
			},
		},
	}

	speakCh := make(chan string, 1)
	exec := ToolExecutor{SpeakChan: speakCh}
	th := newThinker(client, ToLLMTools(AllTools()), exec, nil)

	_, err := th.think(context.Background(), worldSnapshotForTest(), nil, "")
	if err != nil {
		t.Fatalf("think error: %v", err)
	}

	if client.calls < 2 {
		t.Fatalf("calls=%d want >=2", client.calls)
	}
	select {
	case msg := <-speakCh:
		if msg != "hello" {
			t.Fatalf("speak msg=%q want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting speak tool action")
	}
}

func TestThinkerInterruptedByContext(t *testing.T) {
	client := &fakeThinkerClient{
		config: config.LLMConfig{SystemPrompt: "test"},
		blockFn: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	th := newThinker(client, ToLLMTools(AllTools()), ToolExecutor{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := th.think(ctx, worldSnapshotForTest(), nil, "")
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("thinker did not stop after cancellation")
	}
}

func TestThinkerSendsOpenAIToolCallHistory(t *testing.T) {
	client := &fakeThinkerClient{
		config: config.LLMConfig{SystemPrompt: "test"},
		responses: []llm.ToolResponse{
			{
				StopReason: "tool_use",
				Content: []llm.ToolContentBlock{
					{Type: "text", Text: "先看看"},
					{Type: "tool_use", ID: "call_1", Name: "speak", Input: map[string]any{"message": "hi"}},
				},
			},
			{
				StopReason: "end_turn",
				Content:    []llm.ToolContentBlock{{Type: "text", Text: "done"}},
			},
		},
		inspectFn: func(call int, messages []llm.ToolMessage, tools []llm.ToolDefinition) error {
			if call != 2 {
				return nil
			}
			if len(messages) < 4 {
				return fmt.Errorf("messages len=%d want >=4", len(messages))
			}

			assistant := messages[len(messages)-2]
			if assistant.Role != "assistant" {
				return fmt.Errorf("assistant role=%q", assistant.Role)
			}
			if _, ok := assistant.Content.(string); !ok {
				return fmt.Errorf("assistant content should be string, got %T", assistant.Content)
			}
			if len(assistant.ToolCalls) != 1 {
				return fmt.Errorf("assistant tool_calls=%d want 1", len(assistant.ToolCalls))
			}
			if assistant.ToolCalls[0].Function.Name != "speak" {
				return fmt.Errorf("assistant tool name=%q", assistant.ToolCalls[0].Function.Name)
			}

			toolMsg := messages[len(messages)-1]
			if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call_1" {
				return fmt.Errorf("tool msg malformed role=%q id=%q", toolMsg.Role, toolMsg.ToolCallID)
			}
			return nil
		},
	}

	speakCh := make(chan string, 1)
	exec := ToolExecutor{SpeakChan: speakCh}
	th := newThinker(client, ToLLMTools(AllTools()), exec, nil)

	if _, err := th.think(context.Background(), worldSnapshotForTest(), nil, ""); err != nil {
		t.Fatalf("think error: %v", err)
	}
}

func worldSnapshotForTest() world.Snapshot {
	return world.Snapshot{Position: world.Position{X: 0, Y: 64, Z: 0}, Health: 20, Food: 20}
}

func TestFormatBufferedEventDetails(t *testing.T) {
	chat := formatBufferedEvent(BufferedEvent{
		Name:   event.EventChat,
		TickID: 7,
		Payload: &event.ChatEvent{
			Username: "Steve",
			Message:  "hello",
			Source:   event.SourcePlayer,
		},
	})
	if chat == "" || !containsAll(chat, []string{"chat@tick=7", "Steve", "hello", "Player"}) {
		t.Fatalf("chat formatted=%q", chat)
	}

	behaviorEnd := formatBufferedEvent(BufferedEvent{
		Name:    event.EventBehaviorEnd,
		TickID:  11,
		Payload: event.BehaviorEndEvent{Name: "go_to", RunID: 3, Reason: "completed"},
	})
	if !containsAll(behaviorEnd, []string{"behavior.end@tick=11", "go_to", "run_id=3", "completed"}) {
		t.Fatalf("behavior formatted=%q", behaviorEnd)
	}
}

func TestThinkerInitialInputIncludesFormattedEvents(t *testing.T) {
	text := thinkerInitialInput(
		worldSnapshotForTest(),
		[]BufferedEvent{
			{Name: event.EventDamage, TickID: 21, Payload: event.DamageEvent{Amount: 2, NewHP: 18}},
		},
		nil,
		"- [closed] id=ep-1 tick=10 trigger=chat decision=go_to outcome=behavior_end",
		"Nearby entities (last 30s): none\nRecent blocks: none",
	)
	if !containsAll(text, []string{"[Basic Status]", "[Short-term Memory]", "[Spatial Context]", "[Events]", "damage@tick=21", "amount=2.0", "hp=18.0"}) {
		t.Fatalf("initial input=%q", text)
	}
}

func TestThinkerSpatialContextFromMemory(t *testing.T) {
	memory := NewSpatialMemory()
	memory.UpdateEntities([]world.Entity{{EntityID: 42, Type: 150, X: 2, Y: 64, Z: 3}}, 7)

	ctx := thinkerSpatialContext(world.Snapshot{Position: world.Position{X: 0, Y: 64, Z: 0}}, memory)
	if !containsAll(ctx, []string{"Nearby entities", "id=42"}) {
		t.Fatalf("spatial context=%q", ctx)
	}
}

func containsAll(text string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
