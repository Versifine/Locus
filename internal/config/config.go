package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen  ListenConfig  `yaml:"listen"`
	Backend BackendConfig `yaml:"backend"`
	Logging LoggingConfig `yaml:"logging"`
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
