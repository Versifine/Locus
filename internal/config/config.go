package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen  ListenConfig  `yaml:"listen"`
	Backend BackendConfig `yaml:"backend"`
	Logging LoggingConfig `yaml:"logging"`
	LLM     LLMConfig     `yaml:"llm"`
	Mode    string        `yaml:"mode"`
	Bot     BotConfig     `yaml:"bot"`
}
type BotConfig struct {
	Username string `yaml:"username"`
}

type LLMConfig struct {
	Model        string  `yaml:"model"`
	APIKey       string  `yaml:"api_key"`
	Endpoint     string  `yaml:"endpoint"`
	SystemPrompt string  `yaml:"system_prompt"`
	MaxTokens    int     `yaml:"max_tokens"`
	Temperature  float64 `yaml:"temperature"`
	Timeout      int     `yaml:"timeout"`
	MaxHistory   int     `yaml:"max_history"`
}

type ListenConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}
type BackendConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
