# codegate

<p align="center">
  🇺🇸 English | <a href="docs/README.ko.md">🇰🇷 한국어</a>
</p>

<p align="center">
  <a href="https://github.com/ljdongz/codegate/releases"><img src="https://img.shields.io/github/v/release/ljdongz/codegate" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-blue" alt="Platform">
</p>

A Telegram bot for remotely managing Claude Code Channel sessions.

Claude Code's [Channel](https://docs.anthropic.com/en/docs/claude-code/channels) feature allows you to interact with Claude Code sessions via Telegram. However, each session must be started and managed manually on the local machine, and connecting multiple projects requires creating separate bots for each one.

codegate solves this by acting as a single management bot that handles the full lifecycle of Claude Code sessions — start, stop, switch, and resume — all from Telegram. One management bot, one Claude bot, unlimited projects.

## Architecture

```
Telegram (DM or Group)
    ├─ Management bot ── /new, /stop, /switch, /clear, /logs ...
    └─ Claude bot     ── Full Claude Code session

Server (codegate daemon)
    ├─ codegate process → Telegram polling → command dispatch → tmux session management
    └─ tmux sessions
        ├─ cg-myapp   → claude --channels plugin:telegram@...
        └─ cg-backend → claude --channels plugin:telegram@... --continue
```

### How it works

1. codegate runs as a background daemon, polling Telegram for commands via the management bot.
2. On `/new`, it spawns a **tmux session** and starts `claude --channels plugin:telegram` inside it — this connects the Claude bot to Telegram.
3. On `/switch`, it kills the current tmux session and starts a new one with `--continue`, which resumes the previous conversation in that project directory.
4. On `/logs`, it captures the tmux pane output (`tmux capture-pane`) so you can see what Claude Code is doing when it stops responding.
5. On `/clear`, it restarts the session without `--continue`, starting a fresh conversation.

All session isolation is handled by tmux — each project runs in its own `cg-<name>` session, and the management bot orchestrates their lifecycle.

## Installation

### Homebrew (macOS)

```bash
brew install ljdongz/tap/codegate
```

### Shell script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/ljdongz/codegate/main/install.sh | sh
```

### From source

```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make install
```

### Uninstall

```bash
# Homebrew
brew uninstall codegate

# Manual (curl install)
sudo rm /usr/local/bin/codegate

# Clean up config
rm -rf ~/.codegate
```

## Setup

### Prerequisites

- `claude` CLI installed and logged in (`claude /login`)
- `tmux` installed
- `bun` installed (runtime for Claude Code Telegram plugin)
- Telegram plugin installed (`claude plugin install telegram@claude-plugins-official`)
- 2 Telegram bots created via BotFather (one for management, one for Claude)
- Your Telegram user ID (check via @userinfobot)

### Option A: Setup with Claude Code (recommended)

Type the following in Claude Code:

```
@SETUP.md Follow this guide to set up codegate
```

### Option B: CLI setup

```bash
codegate setup
```

## Usage

### Usage modes

codegate supports two usage modes:

#### 1. Direct DM

Send messages to each bot via individual DMs.

- **Management bot DM**: `/new`, `/switch`, `/stop` and other session commands
- **Claude bot DM**: Chat directly with Claude Code

No additional setup required.

#### 2. Group channel

Invite both bots into a single group chat.

**Group setup:**

1. **Create a group** and invite both bots (management bot and Claude bot).

2. **Disable Bot Privacy Mode** (via BotFather)
   - `/mybots` → select bot → Bot Settings → Group Privacy → **Disable**
   - Must be configured for both bots.
   - Bots cannot read group messages when Privacy Mode is enabled.

3. **Chat History setting** (in group settings)
   - Group settings → Chat History for New Members → **Visible**
   - Without this, bots cannot see messages sent before they joined.

4. **Register the group** (send to management bot in the group)
   ```
   /group_add
   ```
   Adds the group to the Claude bot's allow list.

5. **Mention rules**
   - Management bot: Append the bot name to slash commands (e.g., `/new@yourbotname ~/myapp`)
   - Claude bot: Must be mentioned to respond in groups.

**Remove group:**
```
/group_remove
```

### CLI commands

| Command | Description |
|---------|-------------|
| `codegate setup` | Initial setup |
| `codegate start` | Run in background |
| `codegate stop` | Stop daemon |
| `codegate restart` | Restart daemon |
| `codegate status` | Check running status |
| `codegate logs` | View logs |
| `codegate run` | Run in foreground |
| `codegate update` | Update to latest version |
| `codegate version` | Show current version |

### Management bot Telegram commands

| Command | Description |
|---------|-------------|
| `/new <path>` | Start a new Claude session (path must exist) |
| `/stop` | Stop active sessions |
| `/list` | List active sessions |
| `/status` | Show status and default project |
| `/switch <path>` | Switch session (resumes previous conversation) |
| `/switch_new <path>` | Switch session (fresh conversation) |
| `/clear` | Restart current session (fresh conversation) |
| `/mkdir <path>` | Create a directory |
| `/ls [flags] [path]` | List directory contents (default: ~) |
| `/logs [lines]` | Show Claude session logs (default: 50, max: 200) |
| `/group_add` | Add current group to Claude bot allow list |
| `/group_remove` | Remove current group from allow list |
| `/help` | Show help |

### Example workflow

```
# Start a Claude Code session (path must exist; use /mkdir if needed)
/new ~/myapp

# Ask Claude bot to work (via DM or mention in group)
@claudebot Create API endpoints

# Switch to another project (previous conversation preserved)
/switch ~/backend

# Switch back (resumes previous conversation)
/switch ~/myapp

# Switch with a fresh conversation
/switch_new ~/myapp

# Check logs when Claude is not responding
/logs

# Restart session
/clear

# Stop session
/stop
```

## Config

`~/.codegate/config.yaml`:

```yaml
telegram:
  token: "management-bot-token"
  allowed_users:
    - 123456789
claude_bot_token: "claude-bot-token"
max_sessions: 5
skip_permissions: true
```

| Field | Description |
|-------|-------------|
| `telegram.token` | Management bot token (from BotFather) |
| `telegram.allowed_users` | Allowed Telegram user IDs |
| `claude_bot_token` | Claude bot token (from BotFather) |
| `max_sessions` | Maximum concurrent sessions |
| `skip_permissions` | Enable `--dangerously-skip-permissions` |

## License

MIT
