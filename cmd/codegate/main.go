package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ljdongz/codegate/internal/bot"
	"github.com/ljdongz/codegate/internal/config"
	"github.com/ljdongz/codegate/internal/session"
)

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
	installCmd := exec.Command("claude", "-p", "--output-format", "text", "/plugin install telegram@claude-plugins-official")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: plugin install failed: %v\n", err)
		fmt.Println("You may need to install it manually: claude /plugin install telegram@claude-plugins-official")
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
