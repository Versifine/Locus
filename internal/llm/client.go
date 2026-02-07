package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Versifine/locus/internal/config"
)

type Client struct {
	client http.Client
	config config.LLMConfig
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	}
}

var defaultConfig = config.LLMConfig{
	Model:       "deepseek-chat",
	APIKey:      "",
	Endpoint:    "https://api.deepseek.com/v1/chat/completions",
	Timeout:     30,
	MaxTokens:   64,
	Temperature: 0.7,
}

func (c *Client) Config() config.LLMConfig {
	return c.config
}
func NewLLMClient(cfg *config.LLMConfig) *Client {
	if cfg == nil {
		cfg = &defaultConfig
	}
	return &Client{
		client: http.Client{},
		config: *cfg,
	}
}

func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
		defer cancel()
	}
	var responseText string
	reqData := ChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("LLM API request failed", "status", resp.StatusCode, "body", string(body))
		return "", fmt.Errorf("LLM API request failed with status %d", resp.StatusCode)
	}

	var chatResp ChatResponse
	err = json.Unmarshal(body, &chatResp)
	if err != nil {
		return "", err
	}
	if len(chatResp.Choices) == 0 {
		slog.Error("LLM API response has no choices", "body", string(body))
		return "", fmt.Errorf("LLM API response has no choices")
	}

	responseText = chatResp.Choices[0].Message.Content
	return responseText, nil
}
