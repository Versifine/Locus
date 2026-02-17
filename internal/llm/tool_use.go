package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolMessage struct {
	Role       string `json:"role"`
	Content    any    `json:"content,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

type ToolContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type ToolResponse struct {
	Content    []ToolContentBlock
	StopReason string
}

type toolChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ToolMessage    `json:"messages"`
	Tools       []map[string]any `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
}

type toolChatResponse struct {
	Choices []toolChoice `json:"choices"`
}

type toolChoice struct {
	Message      toolResponseMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
	StopReason   string              `json:"stop_reason"`
}

type toolResponseMessage struct {
	Role      string         `json:"role"`
	Content   any            `json:"content"`
	ToolCalls []toolCallItem `json:"tool_calls"`
}

type toolCallItem struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (c *Client) CallWithTools(ctx context.Context, messages []ToolMessage, tools []ToolDefinition) (ToolResponse, error) {
	if c == nil {
		return ToolResponse{}, fmt.Errorf("llm client is nil")
	}
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
		defer cancel()
	}

	request := toolChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Tools:       toOpenAIToolDefs(tools),
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return ToolResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, bytes.NewBuffer(body))
	if err != nil {
		return ToolResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return ToolResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ToolResponse{}, err
	}
	if resp.StatusCode != 200 {
		slog.Error("LLM tool-use API request failed", "status", resp.StatusCode, "body", string(respBody))
		return ToolResponse{}, fmt.Errorf("LLM API request failed with status %d", resp.StatusCode)
	}

	var parsed toolChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ToolResponse{}, err
	}
	if len(parsed.Choices) == 0 {
		return ToolResponse{}, fmt.Errorf("LLM API response has no choices")
	}

	choice := parsed.Choices[0]
	blocks, err := parseToolContent(choice.Message)
	if err != nil {
		return ToolResponse{}, err
	}

	return ToolResponse{
		Content:    blocks,
		StopReason: normalizeStopReason(choice.StopReason, choice.FinishReason),
	}, nil
}

func toOpenAIToolDefs(tools []ToolDefinition) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}
	return out
}

func parseToolContent(msg toolResponseMessage) ([]ToolContentBlock, error) {
	out := make([]ToolContentBlock, 0, 8)

	for _, call := range msg.ToolCalls {
		input := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &input); err != nil {
				return nil, fmt.Errorf("parse tool arguments: %w", err)
			}
		}
		out = append(out, ToolContentBlock{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: input,
		})
	}

	if msg.Content != nil {
		blocks, err := parseMessageContent(msg.Content)
		if err != nil {
			return nil, err
		}
		out = append(out, blocks...)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("LLM tool-use response has no content")
	}

	return out, nil
}

func parseMessageContent(content any) ([]ToolContentBlock, error) {
	switch v := content.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []ToolContentBlock{{Type: "text", Text: v}}, nil
	case []any:
		out := make([]ToolContentBlock, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			block := ToolContentBlock{Type: asString(m["type"])}
			switch block.Type {
			case "text":
				block.Text = asString(m["text"])
			case "tool_use":
				block.ID = asString(m["id"])
				block.Name = asString(m["name"])
				if input, ok := m["input"].(map[string]any); ok {
					block.Input = input
				}
			case "tool_result":
				block.ToolUseID = asString(m["tool_use_id"])
				block.Content = asString(m["content"])
			default:
				buf := bytes.Buffer{}
				_ = json.NewEncoder(&buf).Encode(m)
				block.Type = "text"
				block.Text = strings.TrimSpace(buf.String())
			}
			if block.Type == "text" && strings.TrimSpace(block.Text) == "" {
				continue
			}
			out = append(out, block)
		}
		return out, nil
	case map[string]any:
		if text := asString(v["text"]); text != "" {
			return []ToolContentBlock{{Type: "text", Text: text}}, nil
		}
		if s, err := json.Marshal(v); err == nil {
			return []ToolContentBlock{{Type: "text", Text: string(s)}}, nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

func normalizeStopReason(stopReason, finishReason string) string {
	if stopReason != "" {
		return stopReason
	}
	switch finishReason {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		if finishReason == "" {
			return "end_turn"
		}
		return finishReason
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
