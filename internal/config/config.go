package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ClaudeBot struct {
	Token    string `yaml:"token"`
	ID       int64  `yaml:"id"`
	UserName string `yaml:"username"`
}

type Config struct {
	Telegram struct {
		Token        string  `yaml:"token"`
		AllowedUsers []int64 `yaml:"allowed_users"`
	} `yaml:"telegram"`
	ClaudeBots      []ClaudeBot `yaml:"claude_bots"`
	SkipPermissions bool        `yaml:"skip_permissions"`
}

func DefaultConfig() Config {
	c := Config{}
	c.Telegram.Token = "YOUR_TELEGRAM_BOT_TOKEN"
	c.SkipPermissions = true
	return c
}

func configDir() (string, error) {
	if dir := os.Getenv("CODEGATE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".codegate"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		cfg := DefaultConfig()
		if saveErr := cfg.Save(); saveErr != nil {
			return nil, fmt.Errorf("save default config: %w", saveErr)
		}
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat config: %w", err)
	}

	return LoadFrom(path)
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}
