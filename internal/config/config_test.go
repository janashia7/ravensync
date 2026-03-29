package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LLMProvider != "openai" {
		t.Fatalf("default provider = %q, want openai", cfg.LLMProvider)
	}
	if cfg.LLMModel != "gpt-4o-mini" {
		t.Fatalf("default model = %q, want gpt-4o-mini", cfg.LLMModel)
	}
	if cfg.EmbeddingModel != "text-embedding-3-small" {
		t.Fatalf("default embedding = %q, want text-embedding-3-small", cfg.EmbeddingModel)
	}
	if cfg.DataDir == "" {
		t.Fatal("default data dir should not be empty")
	}
}

func TestDefaultDataDir(t *testing.T) {
	dir := DefaultDataDir()
	if dir == "" {
		t.Fatal("data dir should not be empty")
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".ravensync")
	if dir != expected {
		t.Fatalf("data dir = %q, want %q", dir, expected)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		DataDir:        tmpDir,
		OwnerID:        "test-owner-123",
		LLMProvider:    "ollama",
		LLMModel:       "llama3",
		EmbeddingModel: "nomic-embed-text",
		TelegramToken:  "tok_test",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.yaml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("config permissions = %o, want 0600", perm)
	}

	loaded := DefaultConfig()
	loaded.DataDir = tmpDir
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if err := yaml.Unmarshal(data, loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.OwnerID != "test-owner-123" {
		t.Fatalf("owner_id = %q, want test-owner-123", loaded.OwnerID)
	}
	if loaded.LLMProvider != "ollama" {
		t.Fatalf("provider = %q, want ollama", loaded.LLMProvider)
	}
	if loaded.TelegramToken != "tok_test" {
		t.Fatalf("token = %q, want tok_test", loaded.TelegramToken)
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatal("config file should not exist yet")
	}
}

func TestConfigPath(t *testing.T) {
	cfg := &Config{DataDir: "/tmp/ravensync-test"}
	expected := "/tmp/ravensync-test/config.yaml"
	if got := ConfigPath(cfg); got != expected {
		t.Fatalf("ConfigPath = %q, want %q", got, expected)
	}
}
