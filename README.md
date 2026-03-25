# codegate

텔레그램에서 Claude Code Channel 세션을 원격으로 관리하는 봇.

`/cg new myapp` 한 줄로 Claude Code 풀 세션(Skills, OMC, MCP 지원)을 텔레그램에서 시작할 수 있습니다.

## Architecture

```
텔레그램 그룹
    ├─ @codegate_bot (관리) ── /cg new, /cg stop, /cg list, /cg switch
    └─ @claude_bot (작업) ─── Claude Code 풀 세션

맥미니 (codegate 상주)
    ├─ codegate 프로세스 → 텔레그램 폴링 → /cg 명령 → tmux 세션 관리
    └─ tmux sessions
        ├─ cg-myapp   → claude --channels plugin:telegram@...
        └─ cg-backend → claude --channels plugin:telegram@...
```

## Installation

### From source

```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make build
make install
```

### From release

[Releases](https://github.com/ljdongz/codegate/releases)에서 바이너리를 다운로드하세요.

## Setup

### Option A: Claude Code로 셋업 (추천)

Claude Code에서 다음을 입력하세요:

```
@SETUP.md 를 따라 codegate를 세팅해줘
```

의존성 설치, 플러그인 설치, 설정 파일 생성을 Claude Code가 안내합니다.

### Option B: CLI로 직접 셋업

```bash
codegate setup
```

사전 준비:
- `claude` CLI 설치 및 로그인 (`claude /login`)
- `tmux` 설치
- `bun` 설치 (Claude Code 텔레그램 플러그인 런타임)
- 텔레그램 플러그인 설치 (`claude /plugin install telegram@claude-plugins-official`)
- BotFather에서 봇 2개 생성 (관리용, Claude용)
- 텔레그램 user ID (@userinfobot에서 확인)

## Usage

### CLI

| 명령어 | 설명 |
|--------|------|
| `codegate start` | 백그라운드 실행 |
| `codegate stop` | 중단 |
| `codegate restart` | 재시작 |
| `codegate status` | 실행 상태 확인 |
| `codegate logs` | 로그 보기 |
| `codegate run` | 포그라운드 실행 |

### Telegram

| 명령어 | 설명 |
|--------|------|
| `/cg new <name> [path]` | 새 세션 시작 |
| `/cg stop <name>` | 세션 종료 |
| `/cg list` | 활성 세션 목록 |
| `/cg status` | 상세 상태 |
| `/cg switch <name> [path]` | 세션 전환 |

## Config

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

## License

MIT
