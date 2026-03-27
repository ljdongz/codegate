package bot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ljdongz/codegate/internal/channel"
	"github.com/ljdongz/codegate/internal/config"
	"github.com/ljdongz/codegate/internal/updater"
	"github.com/ljdongz/codegate/internal/pathutil"
	"github.com/ljdongz/codegate/internal/session"
)

type SessionManager interface {
	Start(name, projectPath string) error
	Stop(name string) error
	List() ([]session.SessionInfo, error)
	Switch(name, projectPath string, resume bool) error
	StopAll() error
	Clear(name string) error
	Logs(name string, lines int) (string, error)
	SetClaudeBotToken(token string)
}

var projectNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const maxMessageLen = 4096

type Bot struct {
	api            *tgbotapi.BotAPI
	sm             SessionManager
	allowedUsers   []int64
	version        string
	mu             sync.RWMutex
	defaultProject map[int64]string
	stopCh         chan struct{}
}

func New(token string, sm SessionManager, allowedUsers []int64, version string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating bot API: %w", err)
	}
	return &Bot{
		api:            api,
		sm:             sm,
		allowedUsers:   allowedUsers,
		version:        version,
		defaultProject: make(map[int64]string),
		stopCh:         make(chan struct{}),
	}, nil
}

func (b *Bot) registerCommands() {
	cmds := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "new", Description: "Start a new Claude session — /new <path>"},
		tgbotapi.BotCommand{Command: "stop", Description: "Stop active sessions"},
		tgbotapi.BotCommand{Command: "status", Description: "Show status"},
		tgbotapi.BotCommand{Command: "switch", Description: "Switch session (resume) — /switch <path>"},
		tgbotapi.BotCommand{Command: "switch_new", Description: "Switch session (fresh) — /switch_new <path>"},
		tgbotapi.BotCommand{Command: "ls", Description: "List directory — /ls [flags] [path]"},
		tgbotapi.BotCommand{Command: "bot_add", Description: "Register Claude bot — /bot_add <token>"},
		tgbotapi.BotCommand{Command: "bot_remove", Description: "Remove Claude bot — /bot_remove <ID>"},
		tgbotapi.BotCommand{Command: "group_add", Description: "Allow group — /group_add [ID]"},
		tgbotapi.BotCommand{Command: "group_remove", Description: "Remove group — /group_remove [ID]"},
		tgbotapi.BotCommand{Command: "group_id", Description: "Show this group's chat ID"},
		tgbotapi.BotCommand{Command: "mkdir", Description: "Create directory — /mkdir <path>"},
		tgbotapi.BotCommand{Command: "clear", Description: "Restart current session"},
		tgbotapi.BotCommand{Command: "logs", Description: "Show Claude session logs — /logs [lines]"},
		tgbotapi.BotCommand{Command: "update", Description: "Update codegate to latest version"},
		tgbotapi.BotCommand{Command: "help", Description: "Show help"},
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
			if cmd, args, ok := b.parseCommand(update.Message); ok {
				b.dispatchCommand(update.Message, cmd, args)
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

// parseCommand extracts the command and arguments from a message,
// handling both "/command @bot args" and "@bot /command args" orderings.
func (b *Bot) parseCommand(msg *tgbotapi.Message) (string, []string, bool) {
	if msg.Entities == nil {
		return "", nil, false
	}

	botName := b.api.Self.UserName
	isPrivate := msg.Chat.Type == "private"
	hasBotMention := isPrivate

	var cmdEntity *tgbotapi.MessageEntity
	for i := range msg.Entities {
		e := &msg.Entities[i]
		switch e.Type {
		case "bot_command":
			cmdEntity = e
			// /help@codegatebot format
			cmdText := msg.Text[e.Offset : e.Offset+e.Length]
			if strings.Contains(cmdText, "@"+botName) {
				hasBotMention = true
			}
		case "mention":
			mention := msg.Text[e.Offset : e.Offset+e.Length]
			if mention == "@"+botName {
				hasBotMention = true
			}
		}
	}

	if cmdEntity == nil || !hasBotMention {
		return "", nil, false
	}

	// Extract command name (skip '/', strip @botname suffix)
	cmdText := msg.Text[cmdEntity.Offset : cmdEntity.Offset+cmdEntity.Length]
	cmd := cmdText[1:]
	if at := strings.Index(cmd, "@"); at != -1 {
		cmd = cmd[:at]
	}

	// Arguments: text after the command entity
	rawArgs := strings.TrimSpace(msg.Text[cmdEntity.Offset+cmdEntity.Length:])
	args := b.cleanArgs(rawArgs)

	return cmd, args, true
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

func (b *Bot) dispatchCommand(msg *tgbotapi.Message, cmd string, args []string) {
	switch cmd {
	case "new":
		b.handleNew(msg.Chat.ID, msg.From.ID, args)
	case "stop":
		b.handleStop(msg.Chat.ID, msg.From.ID)
	case "status":
		b.handleStatus(msg, msg.From.ID)
	case "switch":
		b.handleSwitch(msg.Chat.ID, msg.From.ID, args, true)
	case "switch_new":
		b.handleSwitch(msg.Chat.ID, msg.From.ID, args, false)
	case "ls":
		b.handleLs(msg.Chat.ID, args)
	case "bot_add":
		b.handleBotAdd(msg, args)
	case "bot_remove":
		b.handleBotRemove(msg, args)
	case "group_add":
		b.handleGroupAdd(msg, args)
	case "group_remove":
		b.handleGroupRemove(msg, args)
	case "group_id":
		b.handleGroupID(msg)
	case "mkdir":
		b.handleMkdir(msg.Chat.ID, args)
	case "clear":
		b.handleClear(msg.Chat.ID, msg.From.ID)
	case "logs":
		b.handleLogs(msg.Chat.ID, msg.From.ID, args)
	case "update":
		b.handleUpdate(msg.Chat.ID)
	case "help":
		b.reply(msg.Chat.ID, helpText())
	}
}

func (b *Bot) handleNew(chatID int64, userID int64, args []string) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /new <path>")
		return
	}

	path, err := pathutil.Expand(args[0])
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Invalid path: %v", err))
		return
	}

	name := filepath.Base(path)
	if !projectNameRe.MatchString(name) {
		b.reply(chatID, fmt.Sprintf("Invalid project name %q (from path). Use only letters, numbers, hyphens, or underscores.", name))
		return
	}

	if err := b.sm.Start(name, path); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to start session: %v", err))
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

func (b *Bot) handleStatus(msg *tgbotapi.Message, userID int64) {
	chatID := msg.Chat.ID

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
		home, _ := os.UserHomeDir()
		sb.WriteString(fmt.Sprintf("\nDefault project: %s", home))
	}

	// Registered bots
	cfg, err := config.Load()
	if err == nil && len(cfg.ClaudeBots) > 0 {
		sb.WriteString("\n\nClaude bots:\n")
		for _, cb := range cfg.ClaudeBots {
			sb.WriteString(fmt.Sprintf("  • @%s (ID: %d)\n", cb.UserName, cb.ID))
		}
	} else {
		sb.WriteString("\n\nNo Claude bots registered.")
	}

	// Allowed groups
	access, accessErr := channel.LoadAccess()
	if accessErr == nil && len(access.Groups) > 0 {
		sb.WriteString("\nAllowed groups:\n")
		currentChatID := strconv.FormatInt(chatID, 10)
		for groupID := range access.Groups {
			gid, parseErr := strconv.ParseInt(groupID, 10, 64)
			title := groupID
			if parseErr == nil {
				chatCfg := tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: gid}}
				if chat, chatErr := b.api.GetChat(chatCfg); chatErr == nil && chat.Title != "" {
					title = fmt.Sprintf("%s (ID: %s)", chat.Title, groupID)
				}
			}
			marker := ""
			if groupID == currentChatID {
				marker = " [current]"
			}
			sb.WriteString(fmt.Sprintf("  • %s%s\n", title, marker))
		}
	}

	versionLine := b.version
	if tagName, err := updater.CheckLatestVersion(); err == nil {
		latest := strings.TrimPrefix(tagName, "v")
		current := strings.TrimPrefix(b.version, "v")
		if current == latest {
			versionLine += " (up to date)"
		} else {
			versionLine += fmt.Sprintf(" (%s available)", tagName)
		}
	}
	sb.WriteString(fmt.Sprintf("\nVersion: %s", versionLine))

	b.reply(chatID, sb.String())
}

func (b *Bot) handleSwitch(chatID int64, userID int64, args []string, resume bool) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /switch <path>")
		return
	}

	path, err := pathutil.Expand(args[0])
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Invalid path: %v", err))
		return
	}

	name := filepath.Base(path)
	if !projectNameRe.MatchString(name) {
		b.reply(chatID, fmt.Sprintf("Invalid project name %q (from path). Use only letters, numbers, hyphens, or underscores.", name))
		return
	}

	if err := b.sm.Switch(name, path, resume); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to switch to session %q: %v", name, err))
		return
	}

	b.mu.Lock()
	b.defaultProject[userID] = name
	b.mu.Unlock()

	mode := "resumed"
	if !resume {
		mode = "new"
	}
	b.reply(chatID, fmt.Sprintf("Switched to session %q at %s (%s).", name, path, mode))
}

func (b *Bot) handleLs(chatID int64, args []string) {
	var flags []string
	var dir string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else if dir == "" {
			dir = a
		}
	}
	if dir == "" {
		dir = "~"
	}

	expanded, err := pathutil.Expand(dir)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Invalid path: %v", err))
		return
	}

	lsArgs := append(flags, expanded)
	out, err := exec.Command("ls", lsArgs...).CombinedOutput()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("ls %s failed: %s", strings.Join(args, " "), string(out)))
		return
	}

	b.reply(chatID, fmt.Sprintf("$ ls %s\n```\n%s```", strings.Join(append(flags, dir), " "), string(out)))
}

func (b *Bot) handleMkdir(chatID int64, args []string) {
	if len(args) < 1 {
		b.reply(chatID, "Usage: /mkdir <path>")
		return
	}

	path, err := pathutil.Expand(args[0])
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Invalid path: %v", err))
		return
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to create directory: %v", err))
		return
	}

	b.reply(chatID, fmt.Sprintf("Directory created: %s", path))
}

func (b *Bot) handleClear(chatID int64, userID int64) {
	b.mu.RLock()
	def := b.defaultProject[userID]
	b.mu.RUnlock()

	if def == "" {
		b.reply(chatID, "No active session. Start one with /new first.")
		return
	}

	if err := b.sm.Clear(def); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to restart session: %v", err))
		return
	}

	b.reply(chatID, fmt.Sprintf("Session %q restarted.", def))
}

func (b *Bot) handleLogs(chatID int64, userID int64, args []string) {
	lines := 50
	if len(args) >= 1 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			if n > 200 {
				n = 200
			}
			lines = n
		}
	}

	b.mu.RLock()
	def := b.defaultProject[userID]
	b.mu.RUnlock()

	if def == "" {
		b.reply(chatID, "No active session. Start one with /new first.")
		return
	}

	output, err := b.sm.Logs(def, lines)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to get logs: %v", err))
		return
	}

	if output == "" {
		b.reply(chatID, "No log output available.")
		return
	}

	b.reply(chatID, fmt.Sprintf("Logs for session %q (last %d lines):\n```\n%s\n```", def, lines, output))
}

func (b *Bot) handleBotAdd(msg *tgbotapi.Message, args []string) {
	if msg.Chat.Type != "private" {
		b.reply(msg.Chat.ID, "This command is available in DM(private chat) only (token is sensitive).")
		return
	}

	if len(args) < 1 {
		b.reply(msg.Chat.ID, "Usage: /bot_add <claude-bot-token>")
		return
	}

	token := args[0]

	// Validate token by calling Telegram getMe
	claudeBot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Invalid bot token: %v", err))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Check for duplicate
	for _, cb := range cfg.ClaudeBots {
		if cb.ID == claudeBot.Self.ID {
			b.reply(msg.Chat.ID, fmt.Sprintf("Bot @%s (ID: %d) is already registered.", cb.UserName, cb.ID))
			return
		}
	}

	cfg.ClaudeBots = append(cfg.ClaudeBots, config.ClaudeBot{
		Token:    token,
		ID:       claudeBot.Self.ID,
		UserName: claudeBot.Self.UserName,
	})
	if err := cfg.Save(); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	// Write .env with the first bot's token for the channel plugin
	if err := channel.SetupEnv(cfg.ClaudeBots[0].Token); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to write channel .env: %v", err))
		return
	}

	b.sm.SetClaudeBotToken(cfg.ClaudeBots[0].Token)

	b.reply(msg.Chat.ID, fmt.Sprintf("Bot added: @%s (ID: %d). You can now start sessions with /new.", claudeBot.Self.UserName, claudeBot.Self.ID))
}

func (b *Bot) handleBotRemove(msg *tgbotapi.Message, args []string) {
	if msg.Chat.Type != "private" {
		b.reply(msg.Chat.ID, "This command is available in DM(private chat) only.")
		return
	}

	if len(args) < 1 {
		b.reply(msg.Chat.ID, "Usage: /bot_remove <bot-id>")
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Invalid bot ID: %v", err))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	found := false
	var removedName string
	filtered := cfg.ClaudeBots[:0]
	for _, cb := range cfg.ClaudeBots {
		if cb.ID == targetID {
			found = true
			removedName = cb.UserName
		} else {
			filtered = append(filtered, cb)
		}
	}

	if !found {
		b.reply(msg.Chat.ID, fmt.Sprintf("Bot with ID %d not found.", targetID))
		return
	}

	cfg.ClaudeBots = filtered
	if err := cfg.Save(); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	// Update .env and session manager with first remaining bot (or clear)
	if len(cfg.ClaudeBots) > 0 {
		channel.SetupEnv(cfg.ClaudeBots[0].Token) //nolint:errcheck
		b.sm.SetClaudeBotToken(cfg.ClaudeBots[0].Token)
	} else {
		channel.SetupEnv("") //nolint:errcheck
		b.sm.SetClaudeBotToken("")
	}

	b.reply(msg.Chat.ID, fmt.Sprintf("Bot removed: @%s (ID: %d).", removedName, targetID))
}

func (b *Bot) handleUpdate(chatID int64) {
	b.reply(chatID, "Checking for updates...")

	tagName, err := updater.CheckLatestVersion()
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to check updates: %v", err))
		return
	}

	latest := strings.TrimPrefix(tagName, "v")
	current := strings.TrimPrefix(b.version, "v")
	if current == latest {
		b.reply(chatID, fmt.Sprintf("Already up to date (%s).", b.version))
		return
	}

	b.reply(chatID, fmt.Sprintf("Updating: %s → %s ...", b.version, tagName))

	if err := updater.DoUpdate(tagName); err != nil {
		b.reply(chatID, fmt.Sprintf("Update failed: %v", err))
		return
	}

	b.reply(chatID, fmt.Sprintf("Updated to %s. Restarting...", tagName))

	// Spawn restart as a detached process
	exec.Command("codegate", "restart").Start() //nolint:errcheck
}

func (b *Bot) resolveGroupID(msg *tgbotapi.Message, args []string) (string, bool) {
	if len(args) > 0 {
		// Validate the provided ID is a number
		if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
			b.reply(msg.Chat.ID, fmt.Sprintf("Invalid group ID: %v", err))
			return "", false
		}
		return args[0], true
	}
	if msg.Chat.Type == "private" {
		b.reply(msg.Chat.ID, "Usage: /group_add <ID> (in DM) or /group_add (in group chat)")
		return "", false
	}
	return strconv.FormatInt(msg.Chat.ID, 10), true
}

func (b *Bot) handleGroupAdd(msg *tgbotapi.Message, args []string) {
	groupID, ok := b.resolveGroupID(msg, args)
	if !ok {
		return
	}

	access, err := channel.LoadAccess()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to read access.json: %v", err))
		return
	}

	if access.Groups == nil {
		access.Groups = make(map[string]channel.GroupConfig)
	}

	if _, exists := access.Groups[groupID]; exists {
		b.reply(msg.Chat.ID, fmt.Sprintf("Group %s is already in the allow list.", groupID))
		return
	}

	access.Groups[groupID] = channel.GroupConfig{
		RequireMention: true,
		AllowFrom:      []string{},
	}
	if err := channel.SaveAccess(access); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to save access.json: %v", err))
		return
	}

	b.reply(msg.Chat.ID, fmt.Sprintf("Group added (ID: %s). Restart the session with /stop + /new.", groupID))
}

func (b *Bot) handleGroupRemove(msg *tgbotapi.Message, args []string) {
	groupID, ok := b.resolveGroupID(msg, args)
	if !ok {
		return
	}

	access, err := channel.LoadAccess()
	if err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to read access.json: %v", err))
		return
	}

	if _, exists := access.Groups[groupID]; !exists {
		b.reply(msg.Chat.ID, fmt.Sprintf("Group %s is not in the allow list.", groupID))
		return
	}

	delete(access.Groups, groupID)
	if err := channel.SaveAccess(access); err != nil {
		b.reply(msg.Chat.ID, fmt.Sprintf("Failed to save access.json: %v", err))
		return
	}

	b.reply(msg.Chat.ID, fmt.Sprintf("Group removed (ID: %s). Restart the session with /stop + /new.", groupID))
}

func (b *Bot) handleGroupID(msg *tgbotapi.Message) {
	if msg.Chat.Type == "private" {
		b.reply(msg.Chat.ID, "This command must be used in a group chat.")
		return
	}
	b.reply(msg.Chat.ID, fmt.Sprintf("This group's chat ID: %d", msg.Chat.ID))
}


func (b *Bot) cleanArgs(raw string) []string {
	fields := strings.Fields(raw)
	botMention := "@" + b.api.Self.UserName
	cleaned := fields[:0]
	for _, f := range fields {
		if !strings.EqualFold(f, botMention) {
			cleaned = append(cleaned, f)
		}
	}
	return cleaned
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


func helpText() string {
	return `codegate commands:

Session:
  • /new <path> — Start a new Claude session. Path must exist (use /mkdir to create).
  • /stop — Stop all active sessions.
  • /switch <path> — Switch to another project. Resumes previous conversation.
  • /switch_new <path> — Switch to another project. Starts a fresh conversation.
  • /clear — Restart current session with a fresh conversation.
  • /status — Show status (sessions, bots, groups, version).
  • /logs [lines] — Show Claude session terminal output (default: 50, max: 200). Useful when Claude bot is typing but not responding.

File system:
  • /mkdir <path> — Create a directory (supports nested paths).
  • /ls [flags] [path] — List directory contents (default: ~).

Bot:
  • /bot_add <token> — Register a Claude bot (DM only).
  • /bot_remove <ID> — Remove a registered bot (DM only).
  • /update — Update codegate to latest version.

Group:
  • /group_add [ID] — Add group to allow list. Omit ID in group chat to use current group.
  • /group_remove [ID] — Remove group from allow list. Omit ID in group chat to use current group.
  • /group_id — Show this group's chat ID (group chat only).

  • /help — Show this help.`
}
