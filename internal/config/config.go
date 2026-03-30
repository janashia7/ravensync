package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DataDir            string   `yaml:"data_dir"`
	AllowedUsers       []int64  `yaml:"allowed_users,omitempty"`
	AllowedUsernames   []string `yaml:"allowed_usernames,omitempty"` // without @, lowercase in file is fine
	TelegramToken      string   `yaml:"telegram_token,omitempty"`
	LLMProvider    string `yaml:"llm_provider"`
	LLMAPIKey      string `yaml:"llm_api_key,omitempty"`
	LLMModel       string `yaml:"llm_model"`
	EmbeddingModel string `yaml:"embedding_model"`
	EncryptionSalt []byte `yaml:"encryption_salt,omitempty"`
}

func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ravensync")
}

func DefaultConfig() *Config {
	return &Config{
		DataDir:        DefaultDataDir(),
		LLMProvider:    "openai",
		LLMModel:       "gpt-4o-mini",
		EmbeddingModel: "text-embedding-3-small",
	}
}

func Load() (*Config, error) {
	cfg := DefaultConfig()
	path := filepath.Join(cfg.DataDir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := filepath.Join(cfg.DataDir, "config.yaml")
	return os.WriteFile(path, data, 0600)
}

func ConfigPath(cfg *Config) string {
	return filepath.Join(cfg.DataDir, "config.yaml")
}
