package bot

import (
	"fmt"
	"os"
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

func (b *Bot) Start() error {
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
			if strings.HasPrefix(update.Message.Text, "/cg") {
				b.handleCG(update.Message)
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

func (b *Bot) handleCG(msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, "/cg")
	args := strings.Fields(text)

	if len(args) == 0 || args[0] == "help" {
		b.reply(msg.Chat.ID, helpText())
		return
	}

	switch args[0] {
	case "new":
		b.handleNew(msg.Chat.ID, msg.From.ID, args[1:])
	case "stop":
		b.handleStop(msg.Chat.ID, msg.From.ID, args[1:])
	case "list":
		b.handleList(msg.Chat.ID)
	case "status":
		b.handleStatus(msg.Chat.ID, msg.From.ID)
	case "switch":
		b.handleSwitch(msg.Chat.ID, msg.From.ID, args[1:])
	default:
		b.reply(msg.Chat.ID, fmt.Sprintf("Unknown command %q. Use /cg help.", args[0]))
	}
}

func (b *Bot) handleNew(chatID int64, userID int64, args []string) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /cg new <name> [path]")
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

func (b *Bot) handleStop(chatID int64, userID int64, args []string) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /cg stop <name>")
		return
	}
	name := args[0]

	if err := b.sm.Stop(name); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to stop session %q: %v", name, err))
		return
	}

	b.mu.Lock()
	if b.defaultProject[userID] == name {
		delete(b.defaultProject, userID)
	}
	b.mu.Unlock()

	b.reply(chatID, fmt.Sprintf("Session %q stopped.", name))
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
		b.reply(chatID, "Usage: /cg switch <name> [path]")
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

// resolveNamePath validates the project name in args[0] and resolves the path
// from args[1] (if provided) or defaults to ~/Dev/<name>.
// Returns (name, path, true) on success, or sends an error reply and returns ("", "", false).
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

// splitMessage splits text into parts no longer than maxLen, preferring newline boundaries.
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
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return home + p[1:], nil
	}
	return p, nil
}

func helpText() string {
	return `codegate bot commands:
  /cg new <name> [path]    Start a new Claude session
  /cg stop <name>          Stop a session
  /cg list                 List active sessions
  /cg status               Show status and default project
  /cg switch <name> [path] Switch to a different session
  /cg help                 Show this help`
}
