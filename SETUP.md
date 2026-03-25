# codegate Setup Guide (for Claude Code)

Claude Code에서 이 파일을 참고해서 codegate 초기 설정을 진행합니다.

## 1. 의존성 설치

### tmux
- `which tmux`로 설치 여부 확인
- 없으면: `brew install tmux`

### bun
- `which bun`으로 설치 여부 확인
- 없으면: `curl -fsSL https://bun.sh/install | bash`
- 설치 후 `source ~/.zshrc` (또는 `source ~/.bashrc`)

## 2. 텔레그램 플러그인 설치

- Claude Code에서 `/plugin install telegram@claude-plugins-official` 실행

## 3. 텔레그램 봇 생성

사용자에게 다음을 안내하고 토큰을 입력받습니다:

1. 텔레그램에서 @BotFather에게 `/newbot` 전송
2. **관리 봇** 생성 (예: codegate_bot) → 토큰 받기
3. **Claude 봇** 생성 (예: my_claude_bot) → 토큰 받기
4. 텔레그램에서 @userinfobot에게 메시지 전송 → user ID 확인

## 4. 설정 파일 생성

입력받은 정보로 다음 파일들을 생성합니다:

### ~/.codegate/config.yaml (퍼미션 0600)
```yaml
telegram:
  token: "<관리봇 토큰>"
  allowed_users:
    - <user ID>
claude_bot_token: "<Claude봇 토큰>"
max_sessions: 5
skip_permissions: true
```

### ~/.claude/channels/telegram/.env (퍼미션 0600)
```
TELEGRAM_BOT_TOKEN=<Claude봇 토큰>
```

### ~/.claude/channels/telegram/access.json (퍼미션 0600)
```json
{"dmPolicy":"allowlist","allowFrom":["<user ID>"]}
```

## 5. codegate 설치

```bash
go install github.com/ljdongz/codegate/cmd/codegate@latest
```

또는 소스에서:
```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make install
```

## 6. 완료

사용자에게 안내:
- `codegate start`로 봇을 시작하세요
- 텔레그램에서 `/cg new <프로젝트명>`으로 Claude 세션을 시작할 수 있습니다
