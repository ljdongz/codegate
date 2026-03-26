# codegate

텔레그램에서 Claude Code Channel 세션을 원격으로 관리하는 봇.

Claude Code의 [Channel](https://docs.anthropic.com/en/docs/claude-code/channels) 기능을 사용하면 텔레그램으로 Claude Code 세션과 대화할 수 있습니다. 하지만 세션마다 로컬에서 직접 시작하고 관리해야 하며, 여러 프로젝트를 텔레그램에 연동하려면 프로젝트별로 봇을 따로 만들어야 합니다.

codegate는 하나의 관리 봇으로 Claude Code 세션의 전체 라이프사이클(시작, 종료, 전환, 재개)을 텔레그램에서 제어할 수 있게 해줍니다. 관리 봇 하나, Claude 봇 하나로 무제한 프로젝트를 관리할 수 있습니다.

## Architecture

```
텔레그램 (DM 또는 그룹)
    ├─ 관리 봇   ── /new, /stop, /switch, /clear, /logs ...
    └─ Claude 봇 ── Claude Code 풀 세션

서버 (codegate 데몬)
    ├─ codegate 프로세스 → 텔레그램 폴링 → 명령 수신 → tmux 세션 관리
    └─ tmux sessions
        ├─ cg-myapp   → claude --channels plugin:telegram@...
        └─ cg-backend → claude --channels plugin:telegram@... --continue
```

### 동작 방식

1. codegate는 백그라운드 데몬으로 실행되며, 관리 봇을 통해 텔레그램 명령을 폴링합니다.
2. `/new` 시 **tmux 세션**을 생성하고 그 안에서 `claude --channels plugin:telegram`을 실행합니다 — Claude 봇이 텔레그램에 연결됩니다.
3. `/switch` 시 현재 tmux 세션을 종료하고 `--continue` 플래그로 새 세션을 시작합니다 — 해당 프로젝트 디렉토리의 이전 대화를 이어갑니다.
4. `/logs` 시 tmux pane 출력을 캡처(`tmux capture-pane`)하여 Claude Code가 응답하지 않을 때 현재 상태를 확인할 수 있습니다.
5. `/clear` 시 `--continue` 없이 세션을 재시작하여 새 대화를 시작합니다.

모든 세션 격리는 tmux로 처리됩니다 — 각 프로젝트는 자체 `cg-<name>` 세션에서 실행되며, 관리 봇이 라이프사이클을 관리합니다.

## 설치

### Homebrew (macOS)

```bash
brew install ljdongz/tap/codegate
```

### Shell script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/ljdongz/codegate/main/install.sh | sh
```

### 소스에서 빌드

```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make install
```

### 제거

```bash
# Homebrew
brew uninstall codegate

# 수동 (curl 설치)
sudo rm /usr/local/bin/codegate

# 설정 파일 정리
rm -rf ~/.codegate
```

## 설정

### 사전 준비

- `claude` CLI 설치 및 로그인 (`claude /login`)
- `tmux` 설치
- `bun` 설치 (Claude Code 텔레그램 플러그인 런타임)
- 텔레그램 플러그인 설치 (`claude plugin install telegram@claude-plugins-official`)
- BotFather에서 봇 2개 생성 (관리용, Claude용)
- 텔레그램 user ID 확인 (@userinfobot에서 확인)

### 방법 A: Claude Code로 셋업 (추천)

Claude Code에서 다음을 입력하세요:

```
@SETUP.md 를 따라 codegate를 세팅해줘
```

### 방법 B: CLI로 직접 셋업

```bash
codegate setup
```

## 사용법

### 사용 방식

codegate는 두 가지 방식으로 사용할 수 있습니다:

#### 1. 개인 DM 방식

각 봇에게 개별 DM으로 메시지를 보내는 방식입니다.

- **관리 봇 DM**: `/new`, `/switch`, `/stop` 등 세션 관리 명령
- **Claude 봇 DM**: Claude Code와 직접 대화

별도 설정 없이 바로 사용 가능합니다.

#### 2. 그룹 채널 방식

하나의 그룹에 두 봇을 모두 초대하여 사용하는 방식입니다.

**그룹 설정 방법:**

1. **그룹 생성** 후 두 봇(관리 봇, Claude 봇)을 초대합니다.

2. **Bot Privacy Mode 비활성화** (BotFather에서 설정)
   - `/mybots` → 봇 선택 → Bot Settings → Group Privacy → **Disable**
   - 두 봇 모두 설정해야 합니다.
   - Privacy Mode가 활성화되어 있으면 봇이 그룹 메시지를 읽지 못합니다.

3. **Chat History 설정** (그룹 설정에서)
   - 그룹 설정 → Chat History for New Members → **Visible**
   - 이 설정이 없으면 봇이 초대 이전 메시지를 볼 수 없습니다.

4. **그룹 등록** (그룹 채팅에서 관리 봇에게)
   ```
   /groupadd
   ```
   그룹이 Claude 봇의 허용 목록에 추가됩니다.

5. **멘션 규칙**
   - 관리 봇: 슬래시 커맨드에 봇 이름을 붙여서 사용 (예: `/new@내봇이름 ~/myapp`)
   - Claude 봇: 그룹에서는 반드시 멘션해야 응답합니다.

**그룹 해제:**
```
/groupremove
```

### CLI 명령어

| 명령어 | 설명 |
|--------|------|
| `codegate setup` | 초기 설정 |
| `codegate start` | 백그라운드 실행 |
| `codegate stop` | 중단 |
| `codegate restart` | 재시작 |
| `codegate status` | 실행 상태 확인 |
| `codegate logs` | 로그 보기 |
| `codegate run` | 포그라운드 실행 |

### 관리 봇 Telegram 명령어

| 명령어 | 설명 |
|--------|------|
| `/new <path>` | 새 Claude 세션 시작 (경로가 존재해야 함) |
| `/stop` | 활성 세션 종료 |
| `/list` | 활성 세션 목록 |
| `/status` | 상태 및 기본 프로젝트 |
| `/switch <path> [new]` | 세션 전환 (기본: 이전 대화 이어감, `new` 옵션: 새 대화) |
| `/clear` | 현재 세션 재시작 (새 대화) |
| `/mkdir <path>` | 디렉토리 생성 |
| `/ls [flags] [path]` | 디렉토리 목록 (기본: ~) |
| `/logs [lines]` | Claude 세션 로그 확인 (기본: 50줄, 최대: 200줄) |
| `/groupadd` | 현재 그룹을 Claude 봇 허용 목록에 추가 |
| `/groupremove` | 현재 그룹을 허용 목록에서 제거 |
| `/help` | 도움말 |

### 세션 관리 예시

```
# Claude Code 세션 시작 (경로가 존재해야 함. 없으면 /mkdir 활용)
/new ~/myapp

# Claude 봇에게 작업 요청 (DM 또는 그룹에서 멘션)
@claudebot API 엔드포인트 만들어줘

# 다른 프로젝트로 전환 (이전 대화 유지)
/switch ~/backend

# 돌아올 때 이전 대화 이어감
/switch ~/myapp

# 새 대화로 전환
/switch ~/myapp new

# Claude가 응답하지 않을 때 로그 확인
/logs

# 세션 재시작
/clear

# 세션 종료
/stop
```

## 설정 파일

`~/.codegate/config.yaml`:

```yaml
telegram:
  token: "관리봇토큰"
  allowed_users:
    - 123456789
claude_bot_token: "클로드봇토큰"
max_sessions: 5
skip_permissions: true
```

| 항목 | 설명 |
|------|------|
| `telegram.token` | 관리 봇 토큰 (BotFather) |
| `telegram.allowed_users` | 허용된 텔레그램 user ID 목록 |
| `claude_bot_token` | Claude 봇 토큰 (BotFather) |
| `max_sessions` | 최대 동시 세션 수 |
| `skip_permissions` | `--dangerously-skip-permissions` 활성화 |

## 라이선스

MIT
