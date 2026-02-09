package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad 使用表驱动测试覆盖配置加载的核心场景
func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		createFile bool
		content    string
		wantErr    bool
		validate   func(t *testing.T, cfg *Config, err error)
	}{
		{
			name:       "正常加载有效YAML",
			createFile: true,
			content: `listen:
  host: "127.0.0.1"
  port: 25565
backend:
  host: "mc.example.com"
  port: 25566
logging:
  level: "info"
  file: "locus.log"
mode: "bot"
bot:
  username: "TestBot"
llm:
  model: "gpt-4"
  api_key: "secret"
  endpoint: "https://api.openai.com/v1"
  max_tokens: 100
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config, err error) {
				if cfg.Listen.Host != "127.0.0.1" {
					t.Errorf("Listen.Host = %q, 期望 %q", cfg.Listen.Host, "127.0.0.1")
				}
				if cfg.Listen.Port != 25565 {
					t.Errorf("Listen.Port = %d, 期望 %d", cfg.Listen.Port, 25565)
				}
				if cfg.Backend.Host != "mc.example.com" {
					t.Errorf("Backend.Host = %q, 期望 %q", cfg.Backend.Host, "mc.example.com")
				}
				if cfg.Backend.Port != 25566 {
					t.Errorf("Backend.Port = %d, 期望 %d", cfg.Backend.Port, 25566)
				}
				if cfg.Logging.Level != "info" {
					t.Errorf("Logging.Level = %q, 期望 %q", cfg.Logging.Level, "info")
				}
				if cfg.Logging.File != "locus.log" {
					t.Errorf("Logging.File = %q, 期望 %q", cfg.Logging.File, "locus.log")
				}
				if cfg.Mode != "bot" {
					t.Errorf("Mode = %q, 期望 %q", cfg.Mode, "bot")
				}
				if cfg.Bot.Username != "TestBot" {
					t.Errorf("Bot.Username = %q, 期望 %q", cfg.Bot.Username, "TestBot")
				}
				if cfg.LLM.Model != "gpt-4" {
					t.Errorf("LLM.Model = %q, 期望 %q", cfg.LLM.Model, "gpt-4")
				}
			},
		},
		{
			name:       "文件不存在",
			createFile: false,
			wantErr:    true,
			validate: func(t *testing.T, cfg *Config, err error) {
				if !os.IsNotExist(err) {
					t.Errorf("期望文件不存在错误，实际: %v", err)
				}
			},
		},
		{
			name:       "YAML格式错误",
			createFile: true,
			content: `listen:
  host: "127.0.0.1"
  port: [25565
backend:
  host: "mc.example.com"
  port: 25566
`,
			wantErr: true,
			validate: func(t *testing.T, cfg *Config, err error) {
				if err == nil || !strings.Contains(err.Error(), "yaml") {
					t.Errorf("期望返回YAML解析错误，实际: %v", err)
				}
			},
		},
		{
			name:       "空文件",
			createFile: true,
			content:    "",
			wantErr:    false,
			validate: func(t *testing.T, cfg *Config, err error) {
				// 当前实现下，空文件会解析为零值配置。
				if cfg.Listen.Host != "" || cfg.Listen.Port != 0 {
					t.Errorf("Listen 应为零值，实际 Host=%q Port=%d", cfg.Listen.Host, cfg.Listen.Port)
				}
				if cfg.Backend.Host != "" || cfg.Backend.Port != 0 {
					t.Errorf("Backend 应为零值，实际 Host=%q Port=%d", cfg.Backend.Host, cfg.Backend.Port)
				}
				if cfg.Logging.Level != "" || cfg.Logging.File != "" {
					t.Errorf("Logging 应为零值，实际 Level=%q File=%q", cfg.Logging.Level, cfg.Logging.File)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")

			if tt.createFile {
				if err := os.WriteFile(configPath, []byte(tt.content), 0o644); err != nil {
					t.Fatalf("创建测试配置文件失败: %v", err)
				}
			}

			cfg, err := Load(configPath)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && cfg == nil {
				t.Fatalf("Load() 返回了 nil 配置")
			}

			if tt.validate != nil {
				tt.validate(t, cfg, err)
			}
		})
	}
}
