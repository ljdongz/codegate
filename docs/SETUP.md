# codegate Setup Guide (for Claude Code)

Claude Code references this file to perform the initial codegate setup.

## 1. Install dependencies

### tmux
- Check with `which tmux`
- If not installed: `brew install tmux`

### bun
- Check with `which bun`
- If not installed: `curl -fsSL https://bun.sh/install | bash`
- After installation: `source ~/.zshrc` (or `source ~/.bashrc`)

## 2. Install Telegram plugin

- Run `/plugin install telegram@claude-plugins-official` in Claude Code

## 3. Create Telegram bots

Guide the user through the following and collect tokens:

1. Send `/newbot` to @BotFather on Telegram
2. Create a **management bot** (e.g., codegate_bot) → copy the token
3. Create a **Claude bot** (e.g., my_claude_bot) → copy the token
4. Send any message to @userinfobot on Telegram → note the user ID

## 4. Create config files

Generate the following files using the collected information:

### ~/.codegate/config.yaml (permissions 0600)
```yaml
telegram:
  token: "<management bot token>"
  allowed_users:
    - <user ID>
claude_bots:
  - token: "<Claude bot token>"
    id: <bot ID from validation>
    username: "<bot username>"
skip_permissions: true
```

### ~/.claude/channels/telegram/.env (permissions 0600)
```
TELEGRAM_BOT_TOKEN=<Claude bot token>
```

### ~/.claude/channels/telegram/access.json (permissions 0600)
```json
{"dmPolicy":"allowlist","allowFrom":["<user ID>"]}
```

## 5. Install codegate

From source (development):
```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make setup
```

## 6. Done

Inform the user:
- Run the bot with `make dev` (foreground) for development
- Start a Claude session on Telegram with `/new <path>`
