package channel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type GroupConfig struct {
	RequireMention bool     `json:"requireMention"`
	AllowFrom      []string `json:"allowFrom"`
}

type AccessConfig struct {
	DMPolicy  string                 `json:"dmPolicy"`
	AllowFrom []string               `json:"allowFrom"`
	Groups    map[string]GroupConfig  `json:"groups,omitempty"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "channels", "telegram"), nil
}

func AccessPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "access.json"), nil
}

func LoadAccess() (*AccessConfig, error) {
	path, err := AccessPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ac AccessConfig
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, err
	}
	return &ac, nil
}

func SaveAccess(ac *AccessConfig) error {
	path, err := AccessPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(ac)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func SetupAccess(allowedUsers []int64) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, "access.json")

	existing := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}

	userStrings := make([]string, len(allowedUsers))
	for i, u := range allowedUsers {
		userStrings[i] = strconv.FormatInt(u, 10)
	}
	existing["dmPolicy"] = "allowlist"
	existing["allowFrom"] = userStrings

	content, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0600)
}

func SetupEnv(token string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	content := "TELEGRAM_BOT_TOKEN=" + token
	return os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0600)
}
