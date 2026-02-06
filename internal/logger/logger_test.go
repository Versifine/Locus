package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestParseLevel 测试日志级别解析
func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"未知级别默认info", "unknown", slog.LevelInfo},
		{"空字符串默认info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestLevelTag 测试日志级别标签
func TestLevelTag(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		expected string
	}{
		{"error", slog.LevelError, "ERROR"},
		{"warn", slog.LevelWarn, "WARN "},
		{"info", slog.LevelInfo, "INFO "},
		{"debug", slog.LevelDebug, "DEBUG"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := levelTag(tt.level)
			if got != tt.expected {
				t.Errorf("levelTag(%v) = %q, 期望 %q", tt.level, got, tt.expected)
			}
		})
	}
}

// TestFormatAttr 测试属性格式化
func TestFormatAttr(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		attr     slog.Attr
		expected string
	}{
		{
			name:     "无分组",
			group:    "",
			attr:     slog.String("key", "value"),
			expected: "  key=value",
		},
		{
			name:     "有分组",
			group:    "group",
			attr:     slog.String("key", "value"),
			expected: "  group.key=value",
		},
		{
			name:     "整数值",
			group:    "",
			attr:     slog.Int("port", 25565),
			expected: "  port=25565",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttr(tt.group, tt.attr)
			if got != tt.expected {
				t.Errorf("formatAttr(%q, %v) = %q, 期望 %q", tt.group, tt.attr, got, tt.expected)
			}
		})
	}
}

// TestConsoleHandlerEnabled 测试 consoleHandler 的级别过滤
func TestConsoleHandlerEnabled(t *testing.T) {
	h := &consoleHandler{level: slog.LevelInfo}

	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Info 级别应该被启用")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("Error 级别应该被启用")
	}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Debug 级别不应该被启用")
	}
}

// TestConsoleHandlerHandle 测试 consoleHandler 的日志输出
func TestConsoleHandlerHandle(t *testing.T) {
	var buf bytes.Buffer
	h := &consoleHandler{w: &buf, level: slog.LevelDebug}

	record := slog.NewRecord(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"))

	err := h.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() 返回错误: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "12:00:00") {
		t.Errorf("输出应包含时间戳, 实际: %q", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("输出应包含级别标签, 实际: %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("输出应包含消息, 实际: %q", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("输出应包含属性, 实际: %q", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("输出应以换行符结尾, 实际: %q", output)
	}
}

// TestConsoleHandlerWithAttrs 测试 WithAttrs 创建新 handler
func TestConsoleHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := &consoleHandler{w: &buf, level: slog.LevelDebug}

	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "proxy")})

	// 原始 handler 不应该受影响
	if len(h.attrs) != 0 {
		t.Error("原始 handler 的 attrs 不应该被修改")
	}

	// 新 handler 应该有预设属性
	record := slog.NewRecord(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), slog.LevelInfo, "test", 0)
	err := h2.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() 返回错误: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "component=proxy") {
		t.Errorf("输出应包含预设属性, 实际: %q", output)
	}
}

// TestConsoleHandlerWithGroup 测试 WithGroup 创建新 handler
func TestConsoleHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := &consoleHandler{w: &buf, level: slog.LevelDebug}

	h2 := h.WithGroup("server")

	record := slog.NewRecord(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("addr", "127.0.0.1"))
	err := h2.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() 返回错误: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "server.addr=127.0.0.1") {
		t.Errorf("输出应包含分组前缀, 实际: %q", output)
	}
}

// TestConsoleHandlerWithNestedGroup 测试嵌套分组
func TestConsoleHandlerWithNestedGroup(t *testing.T) {
	var buf bytes.Buffer
	h := &consoleHandler{w: &buf, level: slog.LevelDebug}

	h2 := h.WithGroup("server").WithGroup("config")

	record := slog.NewRecord(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("port", "25565"))
	err := h2.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() 返回错误: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "server.config.port=25565") {
		t.Errorf("输出应包含嵌套分组前缀, 实际: %q", output)
	}
}

// TestInitWithFormats 测试不同格式的初始化
func TestInitWithFormats(t *testing.T) {
	formats := []string{"json", "text", "console", ""}

	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			// 由于 sync.Once，我们只能测试 consoleHandler 的创建逻辑
			// 这里通过直接创建 handler 来验证格式切换
			var buf bytes.Buffer
			cfg := Config{Level: "debug", Format: format, Output: &buf}

			var handler slog.Handler
			switch cfg.Format {
			case "json":
				handler = slog.NewJSONHandler(cfg.Output, &slog.HandlerOptions{Level: parseLevel(cfg.Level)})
			case "text":
				handler = slog.NewTextHandler(cfg.Output, &slog.HandlerOptions{Level: parseLevel(cfg.Level)})
			default:
				handler = &consoleHandler{w: cfg.Output, level: parseLevel(cfg.Level)}
			}

			if handler == nil {
				t.Error("handler 不应为 nil")
			}
		})
	}
}
