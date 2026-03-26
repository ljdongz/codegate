package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ljdongz/codegate/internal/bot"
	"github.com/ljdongz/codegate/internal/config"
	"github.com/ljdongz/codegate/internal/session"
)

var version = "dev"

var (
	codegateDir string
	pidFile     string
	logFile     string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}
	codegateDir = filepath.Join(home, ".codegate")
	pidFile = filepath.Join(codegateDir, "codegate.pid")
	logFile = filepath.Join(codegateDir, "codegate.log")
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "setup":
		cmdSetup()
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "restart":
		cmdStop()
		cmdStart()
	case "status":
		cmdStatus()
	case "logs":
		cmdLogs()
	case "run":
		cmdRun()
	case "update":
		cmdUpdate()
	case "version", "-v", "--version":
		fmt.Printf("codegate %s\n", version)
	case "uninstall":
		cmdUninstall()
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("codegate - Telegram Claude Code Session Manager")
	fmt.Println()
	fmt.Println("Usage: codegate <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup    Interactive setup (tokens, dependencies, plugin install)")
	fmt.Println("  start    Start codegate in background")
	fmt.Println("  stop     Stop codegate")
	fmt.Println("  restart  Restart codegate")
	fmt.Println("  status   Show running status")
	fmt.Println("  logs     Tail log file")
	fmt.Println("  run      Run in foreground (for debugging)")
	fmt.Println("  update   Update to the latest version")
	fmt.Println("  version  Show current version")
	fmt.Println("  uninstall Remove all codegate data and settings")
	fmt.Println("  help     Show this help")
}

func cmdSetup() {
	fmt.Println("=== codegate setup ===")
	fmt.Println()

	fmt.Println("Checking dependencies...")
	hasBrew := false
	if _, err := exec.LookPath("brew"); err == nil {
		hasBrew = true
	}

	type dep struct {
		name    string
		brewPkg string // empty means not auto-installable
	}
	deps := []dep{
		{"claude", ""},
		{"tmux", "tmux"},
		{"bun", "oven-sh/bun/bun"},
	}
	for _, d := range deps {
		if _, err := exec.LookPath(d.name); err == nil {
			fmt.Printf("  ✓ %s\n", d.name)
			continue
		}
		if !hasBrew || d.brewPkg == "" {
			fmt.Fprintf(os.Stderr, "  ✗ %s not found. Please install it first.\n", d.name)
			os.Exit(1)
		}
		fmt.Printf("  ✗ %s not found. Installing via Homebrew...\n", d.name)
		installCmd := exec.Command("brew", "install", d.brewPkg)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to install %s: %v\n", d.name, err)
			os.Exit(1)
		}
		fmt.Printf("  ✓ %s installed\n", d.name)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter management bot token (from BotFather): ")
	mgmtToken, _ := reader.ReadString('\n')
	mgmtToken = strings.TrimSpace(mgmtToken)
	if mgmtToken == "" {
		fmt.Fprintln(os.Stderr, "Token cannot be empty.")
		os.Exit(1)
	}

	fmt.Print("Enter Claude bot token (from BotFather): ")
	claudeToken, _ := reader.ReadString('\n')
	claudeToken = strings.TrimSpace(claudeToken)
	if claudeToken == "" {
		fmt.Fprintln(os.Stderr, "Token cannot be empty.")
		os.Exit(1)
	}

	fmt.Print("Enter your Telegram user ID (from @userinfobot): ")
	userIDStr, _ := reader.ReadString('\n')
	userIDStr = strings.TrimSpace(userIDStr)
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid user ID: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig()
	cfg.Telegram.Token = mgmtToken
	cfg.Telegram.AllowedUsers = []int64{userID}
	cfg.ClaudeBotToken = claudeToken
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Config saved to ~/.codegate/config.yaml")

	fmt.Println("Installing Claude Code telegram plugin...")
	installCmd := exec.Command("claude", "plugin", "install", "telegram@claude-plugins-official")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: auto-install failed: %v\n", err)
		fmt.Println("Install manually: claude /plugin install telegram@claude-plugins-official")
	}

	home, _ := os.UserHomeDir()
	channelDir := filepath.Join(home, ".claude", "channels", "telegram")
	if err := os.MkdirAll(channelDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create channel dir: %v\n", err)
		os.Exit(1)
	}

	envContent := "TELEGRAM_BOT_TOKEN=" + claudeToken
	if err := os.WriteFile(filepath.Join(channelDir, ".env"), []byte(envContent), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write .env: %v\n", err)
		os.Exit(1)
	}

	accessContent := fmt.Sprintf(`{"dmPolicy":"allowlist","allowFrom":["%d"]}`, userID)
	if err := os.WriteFile(filepath.Join(channelDir, "access.json"), []byte(accessContent), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write access.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✓ Setup complete!")
	fmt.Println("  Run 'codegate start' to begin.")
}

func cmdStart() {
	if pid, running := readPid(); running {
		fmt.Printf("codegate is already running (PID %d).\n", pid)
		return
	}

	if err := os.MkdirAll(codegateDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dir: %v\n", err)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find executable: %v\n", err)
		os.Exit(1)
	}

	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer lf.Close()

	cmd := exec.Command(exe, "run")
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}

	pidStr := strconv.Itoa(cmd.Process.Pid)
	if err := os.WriteFile(pidFile, []byte(pidStr), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write PID file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("codegate started (PID %s).\n", pidStr)
	fmt.Printf("Logs: %s\n", logFile)
}

func cmdStop() {
	pid, running := readPid()
	if !running {
		fmt.Println("codegate is not running.")
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find process: %v\n", err)
		os.Exit(1)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send SIGTERM: %v\n", err)
	}

	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			os.Remove(pidFile)
			fmt.Println("codegate stopped.")
			return
		}
	}

	fmt.Println("Sending SIGKILL...")
	proc.Signal(syscall.SIGKILL)
	os.Remove(pidFile)
	fmt.Println("codegate killed.")
}

func cmdStatus() {
	pid, running := readPid()
	if running {
		fmt.Printf("codegate is running (PID %d).\n", pid)
	} else {
		fmt.Println("codegate is not running.")
	}
}

func cmdLogs() {
	cmd := exec.Command("tail", "-f", "-n", "50", logFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to tail logs: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cmd.Process.Kill()
	}()

	cmd.Wait()
}

func cmdRun() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Telegram.Token == "" || cfg.Telegram.Token == "YOUR_TELEGRAM_BOT_TOKEN" {
		log.Fatal("Telegram token not configured. Run 'codegate setup' first.")
	}
	if cfg.ClaudeBotToken == "" {
		log.Fatal("Claude bot token not configured. Run 'codegate setup' first.")
	}

	sm := session.NewManager(cfg.ClaudeBotToken, cfg.Telegram.AllowedUsers, cfg.MaxSessions, cfg.SkipPermissions)

	b, err := bot.New(cfg.Telegram.Token, sm, cfg.Telegram.AllowedUsers)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	log.Println("codegate started. Listening for Telegram updates...")

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Start()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received %v, shutting down...", sig)
	case err := <-errCh:
		if err != nil {
			log.Printf("Bot error: %v", err)
		}
	}

	b.Stop()
	if err := sm.StopAll(); err != nil {
		log.Printf("Error stopping sessions: %v", err)
	}
	log.Println("codegate stopped.")
}

func readPid() (int, bool) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return 0, false
	}

	return pid, true
}

func cmdUpdate() {
	const repo = "ljdongz/codegate"

	fmt.Println("Checking for updates...")

	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check for updates: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse release info: %v\n", err)
		os.Exit(1)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(version, "v")
	if current == latest {
		fmt.Printf("Already up to date (%s).\n", version)
		return
	}
	fmt.Printf("Current: %s → Latest: %s\n", version, release.TagName)

	filename := fmt.Sprintf("codegate_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, release.TagName, filename)

	dlResp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download: %v\n", err)
		os.Exit(1)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Download failed: HTTP %d\n", dlResp.StatusCode)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "codegate-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, filename)
	f, err := os.Create(tarPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "Failed to download binary: %v\n", err)
		os.Exit(1)
	}
	f.Close()

	if err := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract: %v\n", err)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find current binary: %v\n", err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	newBin := filepath.Join(tmpDir, "codegate")
	if err := os.Rename(newBin, exe); err != nil {
		// cross-device rename: copy instead
		src, err := os.Open(newBin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open new binary: %v\n", err)
			os.Exit(1)
		}
		defer src.Close()
		dst, err := os.OpenFile(exe, os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write binary (try with sudo): %v\n", err)
			os.Exit(1)
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			fmt.Fprintf(os.Stderr, "Failed to copy binary: %v\n", err)
			os.Exit(1)
		}
		dst.Close()
	}

	fmt.Printf("Updated to %s.\n", release.TagName)
}

func cmdUninstall() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("This will remove all codegate data:")
	fmt.Println("  - Stop running codegate and all tmux sessions")
	fmt.Println("  - ~/.codegate/ (config, logs, PID file)")
	fmt.Println("  - ~/.claude/channels/telegram/.env")
	fmt.Println("  - ~/.claude/channels/telegram/access.json")
	fmt.Println("  - ~/go/bin/codegate binary")
	fmt.Println()
	fmt.Print("Continue? (y/N): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Cancelled.")
		return
	}

	// Stop codegate if running
	if _, running := readPid(); running {
		fmt.Println("Stopping codegate...")
		cmdStop()
	}

	// Kill all cg- tmux sessions
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err == nil {
		for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.HasPrefix(name, "cg-") {
				exec.Command("tmux", "kill-session", "-t", name).Run()
				fmt.Printf("  Killed tmux session: %s\n", name)
			}
		}
	}

	home, _ := os.UserHomeDir()
	removals := []string{
		filepath.Join(home, ".codegate"),
		filepath.Join(home, ".claude", "channels", "telegram", ".env"),
		filepath.Join(home, ".claude", "channels", "telegram", "access.json"),
	}

	for _, path := range removals {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			os.RemoveAll(path)
		} else {
			os.Remove(path)
		}
		fmt.Printf("  Removed: %s\n", path)
	}

	// Remove binary
	binPath := filepath.Join(home, "go", "bin", "codegate")
	if err := os.Remove(binPath); err == nil {
		fmt.Printf("  Removed: %s\n", binPath)
	}

	fmt.Println()
	fmt.Println("codegate uninstalled.")
	fmt.Println("Telegram plugin can be removed manually: claude /plugin uninstall telegram@claude-plugins-official")
}
