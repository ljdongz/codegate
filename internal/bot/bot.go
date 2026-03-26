package bot

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ljdongz/codegate/internal/session"
)

type SessionManager interface {
	Start(name, projectPath string) error
	Stop(name string) error
	List() ([]session.SessionInfo, error)
	Switch(name, projectPath string) error
	StopAll() error
}

var projectNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const maxMessageLen = 4096

type Bot struct {
	api            *tgbotapi.BotAPI
	sm             SessionManager
	allowedUsers   []int64
	mu             sync.RWMutex
	defaultProject map[int64]string
	stopCh         chan struct{}
}

func New(token string, sm SessionManager, allowedUsers []int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating bot API: %w", err)
	}
	return &Bot{
		api:            api,
		sm:             sm,
		allowedUsers:   allowedUsers,
		defaultProject: make(map[int64]string),
		stopCh:         make(chan struct{}),
	}, nil
}

func (b *Bot) registerCommands() {
	cmds := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "new", Description: "새 Claude 세션 시작 — /new <name> [path]"},
		tgbotapi.BotCommand{Command: "stop", Description: "활성 세션 종료"},
		tgbotapi.BotCommand{Command: "list", Description: "활성 세션 목록"},
		tgbotapi.BotCommand{Command: "status", Description: "상태 및 기본 프로젝트"},
		tgbotapi.BotCommand{Command: "switch", Description: "세션 전환 — /switch <name> [path]"},
		tgbotapi.BotCommand{Command: "ls", Description: "디렉토리 목록 — /ls [path]"},
		tgbotapi.BotCommand{Command: "help", Description: "도움말"},
	)
	b.api.Request(cmds) //nolint:errcheck
}

func (b *Bot) Start() error {
	b.registerCommands()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			if update.Message.From == nil {
				continue
			}
			if !b.isAllowed(update.Message.From.ID) {
				continue
			}
			if update.Message.IsCommand() {
				b.handleCommand(update.Message)
			}
		case <-b.stopCh:
			return nil
		}
	}
}

func (b *Bot) Stop() {
	close(b.stopCh)
	b.api.StopReceivingUpdates()
}

func (b *Bot) isAllowed(userID int64) bool {
	if len(b.allowedUsers) == 0 {
		return true
	}
	for _, id := range b.allowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	args := strings.Fields(msg.CommandArguments())

	switch msg.Command() {
	case "new":
		b.handleNew(msg.Chat.ID, msg.From.ID, args)
	case "stop":
		b.handleStop(msg.Chat.ID, msg.From.ID)
	case "list":
		b.handleList(msg.Chat.ID)
	case "status":
		b.handleStatus(msg.Chat.ID, msg.From.ID)
	case "switch":
		b.handleSwitch(msg.Chat.ID, msg.From.ID, args)
	case "ls":
		b.handleLs(msg.Chat.ID, args)
	case "help":
		b.reply(msg.Chat.ID, helpText())
	}
}

func (b *Bot) handleNew(chatID int64, userID int64, args []string) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /new <name> [path]")
		return
	}
	name, path, ok := b.resolveNamePath(chatID, args)
	if !ok {
		return
	}

	if err := b.sm.Start(name, path); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to start session %q: %v", name, err))
		return
	}

	b.mu.Lock()
	b.defaultProject[userID] = name
	b.mu.Unlock()

	b.reply(chatID, fmt.Sprintf("Session %q started at %s.", name, path))
}

func (b *Bot) handleStop(chatID int64, userID int64) {
	sessions, err := b.sm.List()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}
	if len(sessions) == 0 {
		b.reply(chatID, "No active sessions.")
		return
	}

	if err := b.sm.StopAll(); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to stop sessions: %v", err))
		return
	}

	b.mu.Lock()
	delete(b.defaultProject, userID)
	b.mu.Unlock()

	if len(sessions) == 1 {
		b.reply(chatID, fmt.Sprintf("Session %q stopped.", sessions[0].Name))
	} else {
		b.reply(chatID, fmt.Sprintf("%d sessions stopped.", len(sessions)))
	}
}

func (b *Bot) handleList(chatID int64) {
	sessions, err := b.sm.List()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}
	if len(sessions) == 0 {
		b.reply(chatID, "No active sessions.")
		return
	}

	var sb strings.Builder
	sb.WriteString("Active sessions:\n")
	for _, s := range sessions {
		uptime := time.Since(s.CreatedAt).Round(time.Second)
		sb.WriteString(fmt.Sprintf("  • %s (up %s)\n", s.Name, uptime))
	}
	b.reply(chatID, sb.String())
}

func (b *Bot) handleStatus(chatID int64, userID int64) {
	sessions, err := b.sm.List()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to get status: %v", err))
		return
	}

	b.mu.RLock()
	def := b.defaultProject[userID]
	b.mu.RUnlock()

	var sb strings.Builder
	if len(sessions) == 0 {
		sb.WriteString("No active sessions.")
	} else {
		sb.WriteString("Active sessions:\n")
		for _, s := range sessions {
			uptime := time.Since(s.CreatedAt).Round(time.Second)
			marker := ""
			if s.Name == def {
				marker = " [default]"
			}
			sb.WriteString(fmt.Sprintf("  • %s (up %s)%s\n", s.Name, uptime, marker))
		}
	}

	if def != "" {
		sb.WriteString(fmt.Sprintf("\nDefault project: %s", def))
	} else {
		sb.WriteString("\nNo default project set.")
	}

	b.reply(chatID, sb.String())
}

func (b *Bot) handleSwitch(chatID int64, userID int64, args []string) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /switch <name> [path]")
		return
	}
	name, path, ok := b.resolveNamePath(chatID, args)
	if !ok {
		return
	}

	if err := b.sm.Switch(name, path); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to switch to session %q: %v", name, err))
		return
	}

	b.mu.Lock()
	b.defaultProject[userID] = name
	b.mu.Unlock()

	b.reply(chatID, fmt.Sprintf("Switched to session %q at %s.", name, path))
}

func (b *Bot) handleLs(chatID int64, args []string) {
	dir := "~"
	if len(args) >= 1 {
		dir = args[0]
	}

	expanded, err := expandPath(dir)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Invalid path: %v", err))
		return
	}

	out, err := exec.Command("ls", "-al", expanded).CombinedOutput()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("ls %s failed: %s", dir, string(out)))
		return
	}

	b.reply(chatID, fmt.Sprintf("$ ls -al %s\n```\n%s```", dir, string(out)))
}

func (b *Bot) resolveNamePath(chatID int64, args []string) (string, string, bool) {
	name := args[0]
	if !projectNameRe.MatchString(name) {
		b.reply(chatID, fmt.Sprintf("Invalid project name %q. Use only letters, numbers, hyphens, or underscores.", name))
		return "", "", false
	}

	if len(args) >= 2 {
		path, err := expandPath(args[1])
		if err != nil {
			b.reply(chatID, fmt.Sprintf("Invalid path: %v", err))
			return "", "", false
		}
		return name, path, true
	}

	home, err := os.UserHomeDir()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Could not determine home directory: %v", err))
		return "", "", false
	}
	return name, home + "/Dev/" + name, true
}

func (b *Bot) reply(chatID int64, text string) {
	parts := splitMessage(text, maxMessageLen)
	for _, part := range parts {
		msg := tgbotapi.NewMessage(chatID, part)
		b.api.Send(msg) //nolint:errcheck
	}
}

func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	splitAt := strings.LastIndex(text[:maxLen], "\n")
	if splitAt <= 0 {
		return append([]string{text[:maxLen]}, splitMessage(text[maxLen:], maxLen)...)
	}

	first := text[:splitAt]
	remaining := text[splitAt+1:]
	return append([]string{first}, splitMessage(remaining, maxLen)...)
}

func expandPath(p string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	if p == "~" || strings.HasPrefix(p, "~/") {
		return home + p[1:], nil
	}

	// Absolute paths stay as-is
	if strings.HasPrefix(p, "/") {
		return p, nil
	}

	// Relative paths (e.g. "Dev", "./Dev") resolve from home directory
	p = strings.TrimPrefix(p, "./")
	return home + "/" + p, nil
}

func helpText() string {
	return `codegate commands:
  /new <name> [path]    Start a new Claude session
  /stop                 Stop active session
  /list                 List active sessions
  /status               Show status and default project
  /switch <name> [path] Switch to a different session
  /ls [path]            List directory contents (default: ~)
  /help                 Show this help`
}
