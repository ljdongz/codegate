package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxSessions != 5 {
		t.Errorf("expected MaxSessions=5, got %d", cfg.MaxSessions)
	}
	if !cfg.SkipPermissions {
		t.Error("expected SkipPermissions=true")
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatal("expected file to not exist before load")
	}

	cfg, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom non-existent file should return error")
	}
	_ = cfg

	// Use Save/SaveTo to simulate Load creating default
	def := DefaultConfig()
	if err := def.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom after save: %v", err)
	}
	if loaded.MaxSessions != 5 {
		t.Errorf("expected MaxSessions=5, got %d", loaded.MaxSessions)
	}
	if !loaded.SkipPermissions {
		t.Error("expected SkipPermissions=true")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file should exist: %v", err)
	}
}

func TestLoadParsesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `telegram:
  token: "my-bot-token"
  allowed_users:
    - 111
    - 222
claude_bots:
  - token: "claude-token"
    id: 12345
    username: "testbot"
max_sessions: 10
skip_permissions: false
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Telegram.Token != "my-bot-token" {
		t.Errorf("expected token 'my-bot-token', got %q", cfg.Telegram.Token)
	}
	if len(cfg.Telegram.AllowedUsers) != 2 || cfg.Telegram.AllowedUsers[0] != 111 || cfg.Telegram.AllowedUsers[1] != 222 {
		t.Errorf("unexpected AllowedUsers: %v", cfg.Telegram.AllowedUsers)
	}
	if len(cfg.ClaudeBots) != 1 || cfg.ClaudeBots[0].Token != "claude-token" || cfg.ClaudeBots[0].ID != 12345 || cfg.ClaudeBots[0].UserName != "testbot" {
		t.Errorf("unexpected ClaudeBots: %+v", cfg.ClaudeBots)
	}
	if cfg.MaxSessions != 10 {
		t.Errorf("expected MaxSessions=10, got %d", cfg.MaxSessions)
	}
	if cfg.SkipPermissions {
		t.Error("expected SkipPermissions=false")
	}
}

func TestSavePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected file permissions 0600, got %04o", perm)
	}
}
