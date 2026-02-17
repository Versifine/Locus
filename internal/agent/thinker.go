package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

type thinkerClient interface {
	CallWithTools(ctx context.Context, messages []llm.ToolMessage, tools []llm.ToolDefinition) (llm.ToolResponse, error)
	Config() config.LLMConfig
}

type thinker struct {
	llmClient thinkerClient
	tools     []llm.ToolDefinition
	executor  ToolExecutor
	runner    *skill.BehaviorRunner
}

type ThinkerTrace struct {
	Thoughts   []string
	ToolCalls  []llm.ToolContentBlock
	StopReason string
}

func newThinker(client thinkerClient, tools []llm.ToolDefinition, executor ToolExecutor, runner *skill.BehaviorRunner) *thinker {
	return &thinker{
		llmClient: client,
		tools:     tools,
		executor:  executor,
		runner:    runner,
	}
}

func (t *thinker) think(ctx context.Context, snap world.Snapshot, events []BufferedEvent, shortTerm string) (ThinkerTrace, error) {
	trace := ThinkerTrace{}
	if t == nil || t.llmClient == nil {
		return trace, nil
	}

	messages := []llm.ToolMessage{
		{Role: "system", Content: thinkerSystemPrompt(t.llmClient.Config().SystemPrompt)},
		{Role: "user", Content: thinkerInitialInput(snap, events, t.runner, shortTerm)},
	}

	for {
		select {
		case <-ctx.Done():
			trace.StopReason = "interrupted"
			return trace, ctx.Err()
		default:
		}

		response, err := t.llmClient.CallWithTools(ctx, messages, t.tools)
		if err != nil {
			trace.StopReason = "error"
			return trace, err
		}
		trace.StopReason = response.StopReason

		thoughts, toolCalls := splitResponseBlocks(response.Content)
		if len(thoughts) > 0 {
			trace.Thoughts = append(trace.Thoughts, thoughts...)
		}
		if len(toolCalls) > 0 {
			trace.ToolCalls = append(trace.ToolCalls, toolCalls...)
		}

		assistantText := strings.Join(thoughts, "\n")
		if assistantText != "" || len(toolCalls) > 0 {
			assistantMsg := llm.ToolMessage{Role: "assistant", Content: assistantText}
			if len(toolCalls) > 0 {
				assistantMsg.ToolCalls = toOpenAIToolCalls(toolCalls)
			}
			messages = append(messages, assistantMsg)
		}

		if len(toolCalls) == 0 {
			if response.StopReason == "end_turn" || response.StopReason == "" {
				if trace.StopReason == "" {
					trace.StopReason = "end_turn"
				}
				return trace, nil
			}
			continue
		}

		for _, call := range toolCalls {
			result, execErr := t.executor.ExecuteTool(ctx, call.Name, call.Input)
			if execErr != nil {
				slog.Warn("tool execution failed", "tool", call.Name, "error", execErr)
				result = fmt.Sprintf(`{"status":"error","error":%q}`, execErr.Error())
			}
			messages = append(messages, llm.ToolMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    result,
			})
		}
	}
}

func splitResponseBlocks(blocks []llm.ToolContentBlock) ([]string, []llm.ToolContentBlock) {
	textParts := make([]string, 0, len(blocks))
	toolCalls := make([]llm.ToolContentBlock, 0, len(blocks))

	for _, block := range blocks {
		switch block.Type {
		case "tool_use":
			toolCalls = append(toolCalls, block)
		case "text":
			text := strings.TrimSpace(block.Text)
			if text == "" {
				continue
			}
			slog.Info("thought", "text", text)
			textParts = append(textParts, text)
		}
	}

	return textParts, toolCalls
}

func toOpenAIToolCalls(calls []llm.ToolContentBlock) []llm.ToolCall {
	out := make([]llm.ToolCall, 0, len(calls))
	for _, call := range calls {
		args := "{}"
		if call.Input != nil {
			if raw, err := json.Marshal(call.Input); err == nil {
				args = string(raw)
			}
		}
		out = append(out, llm.ToolCall{
			ID:   call.ID,
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      call.Name,
				Arguments: args,
			},
		})
	}
	return out
}

func thinkerSystemPrompt(base string) string {
	if strings.TrimSpace(base) == "" {
		base = "你是一个在 Minecraft 中自主行动的智能体。"
	}
	return base + "\n\n重要：你必须通过工具获取信息和执行动作。[Basic Status] 提供你的基础状态，[Events] 是最近发生的事件。根据这些信息决定下一步行动。如果不确定周围环境，先调用 look() 观察。"
}

func thinkerInitialInput(snap world.Snapshot, events []BufferedEvent, runner *skill.BehaviorRunner, shortTerm string) string {
	active := "none"
	if runner != nil {
		names := runner.Active()
		if len(names) > 0 {
			active = strings.Join(names, ",")
		}
	}

	shortTerm = strings.TrimSpace(shortTerm)
	if shortTerm == "" {
		shortTerm = "- none"
	}

	eventLines := make([]string, 0, len(events))
	for _, evt := range events {
		eventLines = append(eventLines, "- "+formatBufferedEvent(evt))
	}
	if len(eventLines) == 0 {
		eventLines = append(eventLines, "- none")
	}

	return fmt.Sprintf(
		"[Basic Status]\nposition=(%.2f, %.2f, %.2f) yaw=%.2f pitch=%.2f hp=%.1f food=%d active=%s\n\n[Short-term Memory]\n%s\n\n[Events]\n%s",
		snap.Position.X,
		snap.Position.Y,
		snap.Position.Z,
		snap.Position.Yaw,
		snap.Position.Pitch,
		snap.Health,
		snap.Food,
		active,
		shortTerm,
		strings.Join(eventLines, "\n"),
	)
}

func formatBufferedEvent(evt BufferedEvent) string {
	base := evt.Name
	if evt.TickID > 0 {
		base = fmt.Sprintf("%s@tick=%d", base, evt.TickID)
	}

	switch evt.Name {
	case event.EventChat:
		if chat, ok := asChatEvent(evt.Payload); ok {
			return fmt.Sprintf("%s user=%s source=%s msg=%q", base, chat.Username, chat.Source.String(), chat.Message)
		}
	case event.EventDamage:
		if dmg, ok := asDamageEvent(evt.Payload); ok {
			return fmt.Sprintf("%s amount=%.1f hp=%.1f", base, dmg.Amount, dmg.NewHP)
		}
	case event.EventBehaviorEnd:
		if done, ok := asBehaviorEndEvent(evt.Payload); ok {
			return fmt.Sprintf("%s name=%s run_id=%d reason=%s", base, done.Name, done.RunID, done.Reason)
		}
	case event.EventEntityAppear, event.EventEntityLeave:
		if e, ok := asEntityEvent(evt.Payload); ok {
			return fmt.Sprintf("%s entity_id=%d name=%s type=%d", base, e.EntityID, e.Name, e.Type)
		}
	}

	if evt.Payload == nil {
		return base
	}
	encoded, err := json.Marshal(evt.Payload)
	if err != nil {
		return base
	}
	return fmt.Sprintf("%s payload=%s", base, string(encoded))
}

func asChatEvent(raw any) (*event.ChatEvent, bool) {
	switch v := raw.(type) {
	case *event.ChatEvent:
		if v == nil {
			return nil, false
		}
		return v, true
	case event.ChatEvent:
		copy := v
		return &copy, true
	default:
		return nil, false
	}
}

func asDamageEvent(raw any) (event.DamageEvent, bool) {
	switch v := raw.(type) {
	case event.DamageEvent:
		return v, true
	case *event.DamageEvent:
		if v == nil {
			return event.DamageEvent{}, false
		}
		return *v, true
	default:
		return event.DamageEvent{}, false
	}
}

func asBehaviorEndEvent(raw any) (event.BehaviorEndEvent, bool) {
	switch v := raw.(type) {
	case event.BehaviorEndEvent:
		return v, true
	case *event.BehaviorEndEvent:
		if v == nil {
			return event.BehaviorEndEvent{}, false
		}
		return *v, true
	default:
		return event.BehaviorEndEvent{}, false
	}
}

func asEntityEvent(raw any) (event.EntityEvent, bool) {
	switch v := raw.(type) {
	case event.EntityEvent:
		return v, true
	case *event.EntityEvent:
		if v == nil {
			return event.EntityEvent{}, false
		}
		return *v, true
	default:
		return event.EntityEvent{}, false
	}
}
