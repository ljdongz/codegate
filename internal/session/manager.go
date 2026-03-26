package session

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu              sync.RWMutex
	claudeBotToken  string
	allowedUsers    []int64
	maxSessions     int
	skipPermissions bool
}

func NewManager(claudeBotToken string, allowedUsers []int64, maxSessions int, skipPermissions bool) *Manager {
	return &Manager{
		claudeBotToken:  claudeBotToken,
		allowedUsers:    allowedUsers,
		maxSessions:     maxSessions,
		skipPermissions: skipPermissions,
	}
}

type SessionInfo struct {
	Name      string
	CreatedAt time.Time
}

func (m *Manager) Start(name, projectPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startSession(name, projectPath)
}

func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return stopSession(name)
}

func (m *Manager) List() ([]SessionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return listSessions()
}

func (m *Manager) Switch(name, projectPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.stopAllSessions(); err != nil {
		return err
	}
	return m.startSession(name, projectPath)
}

func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopAllSessions()
}

func (m *Manager) startSession(name, projectPath string) error {
	sessionName := "cg-" + name

	if tmuxSessionExists(sessionName) {
		return fmt.Errorf("session %q already exists", name)
	}

	count, err := countActiveSessions()
	if err != nil {
		return fmt.Errorf("counting active sessions: %w", err)
	}
	if count >= m.maxSessions {
		return fmt.Errorf("max sessions (%d) reached", m.maxSessions)
	}

	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Errorf("creating project path: %w", err)
	}

	if err := setupAccessJSON(m.allowedUsers); err != nil {
		return fmt.Errorf("setting up access.json: %w", err)
	}

	if err := setupTelegramEnv(m.claudeBotToken); err != nil {
		return fmt.Errorf("setting up telegram .env: %w", err)
	}

	claudeCmd := "cd '" + shellEscape(projectPath) + "' && claude --channels plugin:telegram@claude-plugins-official"
	if m.skipPermissions {
		claudeCmd += " --dangerously-skip-permissions"
	}

	shellCmd := "zsh -l -c " + "'" + shellEscape(claudeCmd) + "'"
	if err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, shellCmd).Run(); err != nil {
		return fmt.Errorf("starting tmux session: %w", err)
	}
	return nil
}

func (m *Manager) stopAllSessions() error {
	sessions, err := listSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if err := stopSession(s.Name); err != nil {
			return err
		}
	}
	return nil
}

func stopSession(name string) error {
	if err := exec.Command("tmux", "kill-session", "-t", "cg-"+name).Run(); err != nil {
		return fmt.Errorf("killing session %q: %w", name, err)
	}
	return nil
}

func listSessions() ([]SessionInfo, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}|#{session_created}").Output()
	if err != nil {
		// tmux returns non-zero when there are no sessions
		return nil, nil
	}

	var sessions []SessionInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		sessionName := parts[0]
		if !strings.HasPrefix(sessionName, "cg-") {
			continue
		}
		createdAt, err := parseTimestamp(parts[1])
		if err != nil {
			continue
		}
		sessions = append(sessions, SessionInfo{
			Name:      strings.TrimPrefix(sessionName, "cg-"),
			CreatedAt: createdAt,
		})
	}
	return sessions, nil
}

func setupAccessJSON(allowedUsers []int64) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".claude", "channels", "telegram")
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

func setupTelegramEnv(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".claude", "channels", "telegram")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	content := "TELEGRAM_BOT_TOKEN=" + token
	return os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0600)
}

func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

func parseTimestamp(s string) (time.Time, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(n, 0), nil
}

func tmuxSessionExists(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

func countActiveSessions() (int, error) {
	sessions, err := listSessions()
	if err != nil {
		return 0, err
	}
	return len(sessions), nil
}
