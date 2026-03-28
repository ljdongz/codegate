# codegate

<p align="center">
  🇺🇸 English | <a href="docs/README.ko.md">🇰🇷 한국어</a>
</p>

<p align="center">
  <a href="https://github.com/ljdongz/codegate/releases"><img src="https://img.shields.io/github/v/release/ljdongz/codegate" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-blue" alt="Platform">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go">
</p>

Control Claude Code from your phone via Telegram — no SSH, no terminal needed.

Claude Code's [Channel](https://docs.anthropic.com/en/docs/claude-code/channels) feature connects Claude Code to external messaging platforms like Telegram, so you can chat with Claude remotely. However, without codegate, you need terminal access to start, stop, or switch sessions — and there's no built-in way to manage multiple projects from a single bot.

codegate runs as a lightweight daemon on your machine and gives you full session control from Telegram: start new sessions, switch between projects (with conversation history preserved), check logs, and more. One management bot, one Claude bot, unlimited projects.

<video src="docs/demo_video.mp4" controls width="100%"></video>

## Table of Contents

- [Architecture](#architecture)
- [Installation](#installation)
- [Setup](#setup)
- [Usage](#usage)
- [Config](#config)
- [Limitations](#limitations)
- [License](#license)

## Architecture

codegate uses two Telegram bots that work together:

- **Management bot** — controlled by codegate. Accepts commands like `/new`, `/stop`, `/switch` to manage session lifecycle.
- **Claude bot** — controlled by Claude Code's Channel plugin. Relays messages between you and a running Claude Code session.

Two bots are needed because the Claude bot is fully occupied by the Claude Code process (it can only relay messages to/from Claude). Session lifecycle commands must be handled separately by codegate through the management bot.

```
Telegram (DM or Group)
    ├─ Management bot ── /new, /stop, /switch, /clear, /logs ...
    └─ Claude bot     ── Full Claude Code session

Server (codegate daemon)
    ├─ codegate process → Telegram polling → command dispatch → tmux session management
    └─ tmux session (one active at a time)
        └─ cg-myapp → claude --channels plugin:telegram@claude-plugins-official
```

### How it works

1. codegate runs as a background daemon on your machine, polling Telegram for commands via the management bot.
2. On `/new`, it creates a [tmux](https://github.com/tmux/tmux) session (a persistent background terminal) and runs `claude --channels plugin:telegram@claude-plugins-official` inside it — this connects the Claude bot to Telegram.
3. On `/switch`, it stops the current session and starts a new one with `--continue`, which resumes the previous conversation in that project directory. No context is lost — conversation history is stored locally by Claude Code.
4. On `/logs`, it captures the tmux output so you can see what Claude Code is doing when it stops responding.
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

### From source (development)

```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make setup   # Initial config (tokens, etc.)
make dev     # Run in foreground
```

### Uninstall

```bash
# Homebrew
brew uninstall codegate

# Manual (curl install)
sudo rm /usr/local/bin/codegate

# Clean up config
rm -rf ~/.codegate

# Clean up dev environment (if used)
cd codegate && make uninstall
```

## Setup

### Prerequisites

- A [Telegram](https://telegram.org/) account
- `claude` CLI installed and authenticated (run `claude` in your terminal, then type `/login` inside the session)
- `tmux` installed (`brew install tmux` on macOS)
- `bun` installed — required as the JavaScript runtime for Claude Code's Telegram plugin (`curl -fsSL https://bun.sh/install | bash`)
- Telegram plugin installed: inside a Claude Code session, run `/plugin install telegram@claude-plugins-official`
- 2 Telegram bots created via [@BotFather](https://t.me/BotFather) (Telegram's built-in bot creation tool — send it `/newbot` to get started). You need one bot for management and one for Claude. Save both tokens (a string like `123456:ABC-DEF1234ghIkl`).
- Your Telegram numeric user ID: open Telegram, search for [@userinfobot](https://t.me/userinfobot), send it any message, and it will reply with your ID (a number like `123456789`). This is NOT your @username.

### Option A: Setup with Claude Code (recommended)

Type the following in a Claude Code session:

```
@https://github.com/ljdongz/codegate/blob/main/docs/SETUP.md Follow this guide to set up codegate
```

Claude Code will walk you through: installing dependencies, creating Telegram bots, collecting tokens and your user ID, and writing all config files.

### Option B: CLI setup

```bash
codegate setup
```

This collects your management bot token and user ID interactively. After starting codegate, you also need to register your Claude bot by sending a DM to the management bot:

```
/bot_add <your-claude-bot-token>
```

### Verify setup

After starting codegate (`codegate start`), send `/status` to your management bot on Telegram. If you get a response, everything is working.

## Usage

### Usage modes

codegate supports two usage modes. **Start with Direct DM** — it requires no additional setup beyond the initial configuration. Group mode is more convenient for ongoing use (everything in one chat) but requires extra Telegram configuration.

#### 1. Direct DM

Send messages to each bot via individual direct messages.

- **Management bot DM**: `/new`, `/switch`, `/stop` and other session commands
- **Claude bot DM**: Chat directly with Claude Code

No additional setup required.

> **Note:** When you first DM the Claude bot, Telegram automatically sends a `/start` command — this is default Telegram behavior and cannot be disabled. The resulting message can be safely ignored.

#### 2. Group chat

Invite both bots into a single Telegram group chat so all interactions happen in one place.

**Group setup:**

1. **Create a group** and invite both bots (management bot and Claude bot).

2. **Disable Bot Privacy Mode** (via [@BotFather](https://t.me/BotFather))
   - Send `/mybots` to BotFather → select bot → Bot Settings → Group Privacy → **Disable**
   - Must be configured for **both** bots.
   - By default, Telegram bots in groups can only see slash commands directed at them. Disabling Privacy Mode lets the bots see all messages, which is required for codegate to function.

3. **Chat History setting**
   - In the Telegram app: tap the group name → Edit (pencil icon) → Chat History for New Members → **Visible**
   - Without this, bots cannot see messages sent before they joined.

4. **Register the group** (send to management bot in the group)
   ```
   /group_add
   ```
   Adds the group to the Claude bot's allow list so it can respond there.

5. **Mention rules**
   - In groups with multiple bots, Telegram requires you to specify which bot receives a command by appending `@botname`.
   - Management bot: `/new@your_mgmt_bot ~/myapp`
   - Claude bot: Must be mentioned to respond (e.g., `@your_claude_bot fix the login bug`).

**Remove group:**
```
/group_remove
```

### CLI commands

| Command | Description |
|---------|-------------|
| `codegate setup` | Initial setup (tokens, user ID) |
| `codegate start` | Run as background daemon |
| `codegate stop` | Stop daemon |
| `codegate restart` | Restart daemon |
| `codegate status` | Check running status |
| `codegate logs` | View daemon logs |
| `codegate run` | Run in foreground (useful for debugging) |
| `codegate update` | Update to latest version |
| `codegate version` | Show current version |

### Management bot Telegram commands

**Session commands:**

| Command | Description |
|---------|-------------|
| `/new <path>` | Start a new Claude session. The path must be a directory where Claude Code has been run at least once (to complete the workspace trust prompt). |
| `/stop` | Stop the active session |
| `/status` | Show status and active project |
| `/switch <path>` | Switch to a project (resumes previous conversation via `--continue`) |
| `/clear` | Restart current session with a fresh conversation |
| `/logs [lines]` | Show Claude session output (default: 50, max: 200) |

**Bot & group management:**

| Command | Description |
|---------|-------------|
| `/bot_add <token>` | Register a Claude bot (DM only) |
| `/bot_remove <ID>` | Remove a registered bot (DM only) |
| `/group_add` | Add current group to Claude bot allow list |
| `/group_remove` | Remove current group from allow list |
| `/group_id` | Show current group's chat ID |
| `/update` | Update codegate to latest version |

**Other:**

| Command | Description |
|---------|-------------|
| `/ls [flags] [path]` | List directory contents (default: ~) |
| `/help` | Show available commands |

> **Note:** Paths without `/` or `~` prefix are resolved relative to your home directory. For example, `/new myapp` is equivalent to `/new ~/myapp`.

### Example workflow

```
# Send to management bot — start a Claude session
/new ~/myapp

# Send to Claude bot — ask Claude to work
@my_claude_bot Create API endpoints for user authentication

# Send to management bot — switch to another project (previous conversation preserved)
/switch ~/backend

# Send to management bot — switch back (resumes where you left off)
/switch ~/myapp

# Send to management bot — check what Claude is doing
/logs

# Send to management bot — restart with a fresh conversation
/clear

# Send to management bot — stop the session
/stop
```

## Config

`~/.codegate/config.yaml`:

```yaml
telegram:
  token: "management-bot-token"
  allowed_users:
    - 123456789
claude_bots:
  - token: "claude-bot-token"
    id: 12345
    username: "my_claude_bot"
skip_permissions: true
```

| Field | Description |
|-------|-------------|
| `telegram.token` | Management bot token (from [@BotFather](https://t.me/BotFather)) |
| `telegram.allowed_users` | Allowed Telegram user IDs (numeric) |
| `claude_bots` | Registered Claude bots (managed via `/bot_add` command). The `id` is automatically extracted from the token. |
| `skip_permissions` | Enable `--dangerously-skip-permissions` for Claude Code |

> **Warning:** When `skip_permissions` is `true`, Claude Code executes file writes and shell commands **without confirmation**. Since codegate enables remote access via Telegram, this means commands run on your machine from your phone with no interactive approval. Keep `allowed_users` restricted to your own user ID. Set to `false` if you want Claude to require permission for each action (you will need to approve via `/logs` and direct tmux access).

> **Warning:** If `allowed_users` is empty or omitted, the bot accepts commands from **all** Telegram users. Always set at least one user ID.

> **Note:** Set `CODEGATE_CONFIG_DIR` to use a custom config directory (default: `~/.codegate`). Claude Code's Telegram plugin also uses its own config at `~/.claude/channels/telegram/` (created automatically during setup).

## Limitations

- **One active session at a time.** `/new` and `/switch` automatically stop the current session before starting a new one. You can manage unlimited projects, but only one runs concurrently.
- **Session state is in-memory.** If the codegate daemon restarts or crashes, it does not reconnect to previously running tmux sessions. Use `/new` to start a fresh session after restart.
- **No auto-start on boot.** codegate does not install a system service. For persistent operation, configure a launchd plist (macOS) or systemd unit (Linux).
- **`/switch` relies on Claude Code's `--continue` flag**, which resumes the most recent conversation stored in Claude Code's local history for that project directory. If no previous conversation exists, it starts a new one.
- **`--dangerously-skip-permissions` is effectively required.** Claude Code normally prompts for interactive confirmation on file writes and shell commands. Since codegate operates remotely via Telegram, there is no way to approve these prompts in real time. The `skip_permissions` config option (see [Config](#config)) enables this flag so Claude can work autonomously.

## License

MIT
