# codegate

<p align="center">
  <a href="../README.md">🇺🇸 English</a> | 🇰🇷 한국어
</p>

<p align="center">
  <a href="https://github.com/ljdongz/codegate/releases"><img src="https://img.shields.io/github/v/release/ljdongz/codegate" alt="Release"></a>
  <a href="../LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-blue" alt="Platform">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go">
</p>

휴대폰에서 텔레그램으로 Claude Code를 제어하세요 — SSH도, 터미널도 필요 없습니다.

Claude Code의 [Channel](https://docs.anthropic.com/en/docs/claude-code/channels) 기능은 텔레그램 같은 외부 메신저를 통해 Claude Code와 원격으로 대화할 수 있게 해줍니다. 하지만 codegate 없이는 세션을 시작하거나 종료하려면 직접 터미널에 접근해야 하고, 하나의 봇으로 여러 프로젝트를 관리하는 기능도 기본 제공되지 않습니다.

codegate는 서버에서 경량 데몬으로 실행되며, 텔레그램에서 세션의 전체 라이프사이클을 제어할 수 있게 해줍니다: 새 세션 시작, 프로젝트 전환 (대화 기록 유지), 로그 확인 등. 관리 봇 하나, Claude 봇 하나로 무제한 프로젝트를 관리할 수 있습니다.

<video src="demo_video.mp4" controls width="100%"></video>

## 목차

- [아키텍처](#아키텍처)
- [설치](#설치)
- [설정](#설정)
- [사용법](#사용법)
- [설정 파일](#설정-파일)
- [제한 사항](#제한-사항)
- [라이선스](#라이선스)

## 아키텍처

codegate는 두 개의 텔레그램 봇을 함께 사용합니다:

- **관리 봇** — codegate가 제어합니다. `/new`, `/stop`, `/switch` 같은 세션 관리 명령을 처리합니다.
- **Claude 봇** — Claude Code의 Channel 플러그인이 제어합니다. 사용자와 Claude Code 세션 간 메시지를 중계합니다.

봇이 두 개 필요한 이유: Claude 봇은 Claude Code 프로세스가 완전히 점유하고 있어 메시지 중계만 가능합니다. 세션 제어 명령은 codegate가 별도의 관리 봇을 통해 처리해야 합니다.

```
텔레그램 (DM 또는 그룹)
    ├─ 관리 봇   ── /new, /stop, /switch, /clear, /logs ...
    └─ Claude 봇 ── Claude Code 풀 세션

서버 (codegate 데몬)
    ├─ codegate 프로세스 → 텔레그램 폴링 → 명령 수신 → tmux 세션 관리
    └─ tmux 세션 (한 번에 하나만 활성)
        └─ cg-myapp → claude --channels plugin:telegram@claude-plugins-official
```

### 동작 방식

1. codegate는 백그라운드 데몬으로 실행되며, 관리 봇을 통해 텔레그램 명령을 폴링합니다.
2. `/new` 시 [tmux](https://github.com/tmux/tmux) 세션(백그라운드에서 지속되는 터미널)을 생성하고 그 안에서 `claude --channels plugin:telegram@claude-plugins-official`을 실행합니다 — Claude 봇이 텔레그램에 연결됩니다.
3. `/switch` 시 현재 세션을 종료하고 `--continue` 플래그로 새 세션을 시작합니다 — 해당 프로젝트 디렉토리의 이전 대화를 이어갑니다. 대화 기록은 Claude Code가 로컬에 저장하므로 유실되지 않습니다.
4. `/logs` 시 tmux 출력을 캡처하여 Claude Code가 응답하지 않을 때 현재 상태를 확인할 수 있습니다.
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

### 소스에서 빌드 (개발용)

```bash
git clone https://github.com/ljdongz/codegate.git
cd codegate
make setup   # 초기 설정 (토큰 등)
make dev     # 포그라운드 실행
```

### 제거

```bash
# Homebrew
brew uninstall codegate

# 수동 (curl 설치)
sudo rm /usr/local/bin/codegate

# 설정 파일 정리
rm -rf ~/.codegate

# 개발 환경 정리 (사용한 경우)
cd codegate && make uninstall
```

## 설정

### 사전 준비

- [텔레그램](https://telegram.org/) 계정
- `claude` CLI 설치 및 인증 (터미널에서 `claude` 실행 후 세션 내에서 `/login` 입력)
- `tmux` 설치 (macOS: `brew install tmux`)
- `bun` 설치 — Claude Code 텔레그램 플러그인의 JavaScript 런타임으로 필요 (`curl -fsSL https://bun.sh/install | bash`)
- 텔레그램 플러그인 설치: Claude Code 세션 내에서 `/plugin install telegram@claude-plugins-official` 실행
- [@BotFather](https://t.me/BotFather)(텔레그램의 봇 생성 도구)에서 봇 2개 생성 — BotFather에게 `/newbot`을 보내면 시작할 수 있습니다. 관리용 1개, Claude용 1개가 필요합니다. 두 봇의 토큰(`123456:ABC-DEF1234ghIkl` 형태의 문자열)을 저장하세요.
- 텔레그램 숫자 user ID: 텔레그램에서 [@userinfobot](https://t.me/userinfobot)을 검색하고 아무 메시지를 보내면 숫자 ID(`123456789` 같은 형태)를 알려줍니다. @username과 다릅니다.

### 방법 A: Claude Code로 셋업 (추천)

Claude Code 세션에서 다음을 입력하세요:

```
@https://github.com/ljdongz/codegate/blob/main/docs/SETUP.md 를 따라 codegate를 세팅해줘
```

Claude Code가 의존성 설치, 텔레그램 봇 생성, 토큰 및 user ID 수집, 설정 파일 작성까지 안내합니다.

### 방법 B: CLI로 직접 셋업

```bash
codegate setup
```

관리 봇 토큰과 user ID를 대화형으로 수집합니다. codegate를 시작한 후, 관리 봇에게 DM으로 Claude 봇도 등록해야 합니다:

```
/bot_add <클로드봇토큰>
```

### 셋업 확인

codegate를 시작한 후 (`codegate start`), 텔레그램에서 관리 봇에게 `/status`를 보내세요. 응답이 오면 정상 작동 중입니다.

## 사용법

### 사용 방식

codegate는 두 가지 방식으로 사용할 수 있습니다. **개인 DM 방식으로 시작하세요** — 초기 설정 외에 추가 설정이 필요 없습니다. 그룹 방식은 모든 대화를 한 곳에서 할 수 있어 편리하지만 추가 텔레그램 설정이 필요합니다.

#### 1. 개인 DM 방식

각 봇에게 개별 DM(다이렉트 메시지)으로 메시지를 보내는 방식입니다.

- **관리 봇 DM**: `/new`, `/switch`, `/stop` 등 세션 관리 명령
- **Claude 봇 DM**: Claude Code와 직접 대화

별도 설정 없이 바로 사용 가능합니다.

> **참고:** Claude 봇에게 처음 DM을 보내면 텔레그램이 자동으로 `/start` 명령을 전송합니다 — 이는 텔레그램의 기본 동작이며 비활성화할 수 없습니다. 이때 표시되는 메시지는 무시해도 됩니다.

#### 2. 그룹 채팅 방식

하나의 텔레그램 그룹에 두 봇을 모두 초대하여 모든 대화를 한 곳에서 하는 방식입니다.

**그룹 설정 방법:**

1. **그룹 생성** 후 두 봇(관리 봇, Claude 봇)을 초대합니다.

2. **Bot Privacy Mode 비활성화** ([@BotFather](https://t.me/BotFather)에서 설정)
   - BotFather에게 `/mybots` 전송 → 봇 선택 → Bot Settings → Group Privacy → **Disable**
   - 두 봇 **모두** 설정해야 합니다.
   - 기본적으로 텔레그램 봇은 그룹에서 자신에게 직접 보낸 슬래시 명령만 볼 수 있습니다. Privacy Mode를 비활성화해야 봇이 모든 메시지를 읽을 수 있으며, codegate가 정상 작동합니다.

3. **Chat History 설정**
   - 텔레그램 앱에서: 그룹 이름 탭 → 편집 (연필 아이콘) → Chat History for New Members → **Visible**
   - 이 설정이 없으면 봇이 초대 이전 메시지를 볼 수 없습니다.

4. **그룹 등록** (그룹 채팅에서 관리 봇에게)
   ```
   /group_add
   ```
   그룹이 Claude 봇의 허용 목록에 추가되어 그룹에서 응답할 수 있게 됩니다.

5. **멘션 규칙**
   - 여러 봇이 있는 그룹에서는 텔레그램이 어떤 봇에게 보내는 명령인지 `@봇이름`을 붙여서 구분해야 합니다.
   - 관리 봇: `/new@내관리봇 ~/myapp`
   - Claude 봇: 반드시 멘션해야 응답합니다 (예: `@내클로드봇 로그인 버그 수정해줘`).

**그룹 해제:**
```
/group_remove
```

### CLI 명령어

| 명령어 | 설명 |
|--------|------|
| `codegate setup` | 초기 설정 (토큰, user ID) |
| `codegate start` | 백그라운드 데몬으로 실행 |
| `codegate stop` | 데몬 중단 |
| `codegate restart` | 데몬 재시작 |
| `codegate status` | 실행 상태 확인 |
| `codegate logs` | 데몬 로그 보기 |
| `codegate run` | 포그라운드 실행 (디버깅에 유용) |
| `codegate update` | 최신 버전으로 업데이트 |
| `codegate version` | 현재 버전 확인 |

### 관리 봇 텔레그램 명령어

**세션 관리:**

| 명령어 | 설명 |
|--------|------|
| `/new <path>` | 새 Claude 세션 시작. 해당 경로는 Claude Code를 한 번 이상 실행하여 workspace trust를 완료한 디렉토리여야 합니다. |
| `/stop` | 활성 세션 종료 |
| `/status` | 상태 및 활성 프로젝트 확인 |
| `/switch <path>` | 세션 전환 (`--continue`로 이전 대화 이어감) |
| `/clear` | 현재 세션을 새 대화로 재시작 |
| `/logs [lines]` | Claude 세션 출력 확인 (기본: 50줄, 최대: 200줄) |

**봇 & 그룹 관리:**

| 명령어 | 설명 |
|--------|------|
| `/bot_add <token>` | Claude 봇 등록 (DM 전용) |
| `/bot_remove <ID>` | 등록된 봇 제거 (DM 전용) |
| `/group_add` | 현재 그룹을 Claude 봇 허용 목록에 추가 |
| `/group_remove` | 현재 그룹을 허용 목록에서 제거 |
| `/group_id` | 현재 그룹의 chat ID 표시 |
| `/update` | codegate 최신 버전으로 업데이트 |

**기타:**

| 명령어 | 설명 |
|--------|------|
| `/ls [flags] [path]` | 디렉토리 목록 (기본: ~) |
| `/help` | 사용 가능한 명령어 표시 |

> **참고:** `/` 또는 `~`로 시작하지 않는 경로는 홈 디렉토리 기준으로 해석됩니다. 예를 들어 `/new myapp`은 `/new ~/myapp`과 동일합니다.

### 세션 관리 예시

```
# 관리 봇에게 전송 — Claude 세션 시작
/new ~/myapp

# Claude 봇에게 전송 — 작업 요청
@my_claude_bot 사용자 인증 API 엔드포인트 만들어줘

# 관리 봇에게 전송 — 다른 프로젝트로 전환 (이전 대화 유지)
/switch ~/backend

# 관리 봇에게 전송 — 돌아올 때 이전 대화 이어감
/switch ~/myapp

# 관리 봇에게 전송 — Claude가 뭘 하고 있는지 확인
/logs

# 관리 봇에게 전송 — 새 대화로 재시작
/clear

# 관리 봇에게 전송 — 세션 종료
/stop
```

## 설정 파일

`~/.codegate/config.yaml`:

```yaml
telegram:
  token: "관리봇토큰"
  allowed_users:
    - 123456789
claude_bots:
  - token: "클로드봇토큰"
    id: 12345
    username: "my_claude_bot"
skip_permissions: true
```

| 항목 | 설명 |
|------|------|
| `telegram.token` | 관리 봇 토큰 ([@BotFather](https://t.me/BotFather)에서 발급) |
| `telegram.allowed_users` | 허용된 텔레그램 user ID (숫자) |
| `claude_bots` | 등록된 Claude 봇 목록 (`/bot_add` 명령으로 관리). `id`는 토큰에서 자동 추출됩니다. |
| `skip_permissions` | Claude Code의 `--dangerously-skip-permissions` 활성화 |

> **경고:** `skip_permissions`가 `true`이면 Claude Code가 파일 수정, 쉘 명령 등을 **확인 없이 실행**합니다. codegate는 텔레그램을 통한 원격 접근이므로, 휴대폰에서 보낸 명령이 서버에서 무확인으로 실행됩니다. `allowed_users`를 반드시 본인 user ID로 제한하세요. `false`로 설정하면 Claude가 매 작업마다 권한을 요청합니다 (`/logs` 및 직접 tmux 접근으로 승인 필요).

> **경고:** `allowed_users`가 비어있거나 생략되면 **모든 텔레그램 사용자**의 명령을 수락합니다. 반드시 하나 이상의 user ID를 설정하세요.

> **참고:** `CODEGATE_CONFIG_DIR` 환경변수로 설정 디렉토리를 변경할 수 있습니다 (기본값: `~/.codegate`). Claude Code의 텔레그램 플러그인도 자체 설정 파일을 `~/.claude/channels/telegram/`에 저장합니다 (셋업 시 자동 생성).

## 제한 사항

- **한 번에 하나의 세션만 활성화됩니다.** `/new`과 `/switch`는 현재 세션을 자동 종료한 후 새 세션을 시작합니다. 프로젝트는 무제한으로 관리할 수 있지만, 동시에 하나만 실행됩니다.
- **세션 상태는 메모리에만 저장됩니다.** codegate 데몬이 재시작되거나 크래시되면 기존 tmux 세션에 재연결하지 않습니다. 재시작 후 `/new`로 새 세션을 시작하세요.
- **부팅 시 자동 시작되지 않습니다.** codegate는 시스템 서비스를 설치하지 않습니다. 상시 운영이 필요하면 launchd plist (macOS) 또는 systemd unit (Linux)을 설정하세요.
- **`/switch`는 Claude Code의 `--continue` 플래그에 의존합니다.** 해당 프로젝트 디렉토리의 Claude Code 로컬 기록에서 가장 최근 대화를 이어갑니다. 이전 대화가 없으면 새 대화를 시작합니다.
- **`--dangerously-skip-permissions` 플래그가 사실상 필수입니다.** Claude Code는 파일 수정, 쉘 명령 실행 시 대화형 확인을 요구합니다. codegate는 텔레그램을 통해 원격으로 동작하므로 이러한 프롬프트를 실시간으로 승인할 수 없습니다. `skip_permissions` 설정 옵션([설정 파일](#설정-파일) 참고)으로 이 플래그를 활성화하면 Claude가 자율적으로 작업할 수 있습니다.

## 라이선스

MIT
