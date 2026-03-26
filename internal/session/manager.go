package session

import (
	"fmt"
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

func (m *Manager) startSession(name, projectPath string, resume bool) error {
	sessionName := sessionPrefix + name

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

	claudeCmd := "cd '" + shellEscape(projectPath) + "' && claude --channels plugin:telegram@claude-plugins-official"
	if resume {
		claudeCmd += " --continue"
	}
	if m.skipPermissions {
		claudeCmd += " --dangerously-skip-permissions"
	}

	if err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, claudeCmd).Run(); err != nil {
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

func countActiveSessions() (int, error) {
	sessions, err := listSessions()
	if err != nil {
		return 0, err
	}
	return len(sessions), nil
}
