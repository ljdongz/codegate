package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ljdongz/codegate/internal/channel"
)

const sessionPrefix = "cg-"

type Manager struct {
	mu              sync.RWMutex
	claudeBotToken  string
	allowedUsers    []int64
	skipPermissions bool
}

func NewManager(claudeBotToken string, allowedUsers []int64, skipPermissions bool) *Manager {
	return &Manager{
		claudeBotToken:  claudeBotToken,
		allowedUsers:    allowedUsers,
		skipPermissions: skipPermissions,
	}
}

type SessionInfo struct {
	Name      string
	CreatedAt time.Time
}

func (m *Manager) SetClaudeBotToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claudeBotToken = token
}

func (m *Manager) Start(name, projectPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.stopAllSessions()
	m.flushBotUpdates()
	time.Sleep(1 * time.Second)
	return m.startSession(name, projectPath, false)
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

func (m *Manager) Switch(name, projectPath string, resume bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.stopAllSessions(); err != nil {
		return err
	}
	m.flushBotUpdates()
	time.Sleep(1 * time.Second)
	return m.startSession(name, projectPath, resume)
}

func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopAllSessions()
}

func (m *Manager) Clear(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionName := sessionPrefix + name
	if !tmuxSessionExists(sessionName) {
		return fmt.Errorf("session %q does not exist", name)
	}

	out, err := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{pane_current_path}").Output()
	if err != nil {
		return fmt.Errorf("getting session path: %w", err)
	}
	projectPath := strings.TrimSpace(string(out))

	if err := stopSession(name); err != nil {
		return err
	}
	m.flushBotUpdates()
	return m.startSession(name, projectPath, false)
}

func (m *Manager) Logs(name string, lines int) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionName := sessionPrefix + name
	if !tmuxSessionExists(sessionName) {
		return "", fmt.Errorf("session %q does not exist", name)
	}

	out, err := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-S", fmt.Sprintf("-%d", lines)).Output()
	if err != nil {
		return "", fmt.Errorf("capturing pane output: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// CheckAuth captures recent tmux output and checks for auth-failure indicators.
// Returns true if authentication appears to be needed.
func (m *Manager) CheckAuth(name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionName := sessionPrefix + name
	if !tmuxSessionExists(sessionName) {
		return false, fmt.Errorf("session %q does not exist", name)
	}

	out, err := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-S", "-30").Output()
	if err != nil {
		return false, fmt.Errorf("capturing pane output: %w", err)
	}

	lower := strings.ToLower(string(out))
	authIndicators := []string{
		"not logged in",
		"run /login",
		"please run /login",
	}
	for _, indicator := range authIndicators {
		if strings.Contains(lower, indicator) {
			return true, nil
		}
	}
	return false, nil
}

// flushBotUpdates acknowledges all pending Telegram updates for the Claude bot
// so that a newly started session does not receive stale messages from a previous session.
func (m *Manager) flushBotUpdates() {
	if m.claudeBotToken == "" {
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=-1&timeout=0", m.claudeBotToken)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int64 `json:"update_id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !result.OK || len(result.Result) == 0 {
		return
	}

	// Acknowledge all updates by requesting offset = last_update_id + 1
	ackURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=0", m.claudeBotToken, result.Result[0].UpdateID+1)
	ackResp, err := http.Get(ackURL)
	if err != nil {
		return
	}
	ackResp.Body.Close()
}

func (m *Manager) startSession(name, projectPath string, resume bool) error {
	if m.claudeBotToken == "" {
		return fmt.Errorf("Claude bot token not set. Send /bot <token> to the management bot first")
	}

	sessionName := sessionPrefix + name

	if tmuxSessionExists(sessionName) {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	}

	info, err := os.Stat(projectPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", projectPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", projectPath)
	}

	if err := channel.SetupAccess(m.allowedUsers); err != nil {
		return fmt.Errorf("setting up access.json: %w", err)
	}

	if err := channel.SetupEnv(m.claudeBotToken); err != nil {
		return fmt.Errorf("setting up telegram .env: %w", err)
	}

	cdCmd := "cd '" + shellEscape(projectPath) + "'"
	claudeArgs := "claude --channels plugin:telegram@claude-plugins-official"
	if m.skipPermissions {
		claudeArgs += " --dangerously-skip-permissions"
	}

	var claudeCmd string
	if resume {
		claudeCmd = cdCmd + " && (" + claudeArgs + " --continue || " + claudeArgs + ")"
	} else {
		claudeCmd = cdCmd + " && " + claudeArgs
	}

	if err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, claudeCmd).Run(); err != nil {
		return fmt.Errorf("starting tmux session: %w", err)
	}

	// Keep tmux session alive after command exits so /logs can capture error output
	_ = exec.Command("tmux", "set-option", "-t", sessionName, "remain-on-exit", "on").Run()
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
	if err := exec.Command("tmux", "kill-session", "-t", sessionPrefix+name).Run(); err != nil {
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
		if !strings.HasPrefix(sessionName, sessionPrefix) {
			continue
		}
		createdAt, err := parseTimestamp(parts[1])
		if err != nil {
			continue
		}
		sessions = append(sessions, SessionInfo{
			Name:      strings.TrimPrefix(sessionName, sessionPrefix),
			CreatedAt: createdAt,
		})
	}
	return sessions, nil
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

