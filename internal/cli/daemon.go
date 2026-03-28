package cli

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ljdongz/codegate/internal/bot"
	"github.com/ljdongz/codegate/internal/config"
	"github.com/ljdongz/codegate/internal/session"
)

func Start() {
	if pid, running := ReadPid(); running {
		fmt.Printf("codegate is already running (PID %d).\n", pid)
		return
	}

	if err := os.MkdirAll(CodegateDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dir: %v\n", err)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find executable: %v\n", err)
		os.Exit(1)
	}

	lf, err := os.OpenFile(LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
	if err := os.WriteFile(PidFile, []byte(pidStr), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write PID file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("codegate started (PID %s).\n", pidStr)
	fmt.Printf("Logs: %s\n", LogFile)
}

func Stop() {
	pid, running := ReadPid()
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
			os.Remove(PidFile)
			fmt.Println("codegate stopped.")
			return
		}
	}

	fmt.Println("Sending SIGKILL...")
	proc.Signal(syscall.SIGKILL)
	os.Remove(PidFile)
	fmt.Println("codegate killed.")
}

func Status() {
	pid, running := ReadPid()
	if running {
		fmt.Printf("codegate is running (PID %d).\n", pid)
	} else {
		fmt.Println("codegate is not running.")
	}
}

func Logs() {
	cmd := exec.Command("tail", "-f", "-n", "50", LogFile)
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

func Run() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Telegram.Token == "" || cfg.Telegram.Token == "YOUR_TELEGRAM_BOT_TOKEN" {
		log.Fatal("Telegram token not configured. Run 'codegate setup' first.")
	}
	var claudeBotToken string
	if len(cfg.ClaudeBots) > 0 {
		claudeBotToken = cfg.ClaudeBots[0].Token
	}
	sm := session.NewManager(claudeBotToken, cfg.Telegram.AllowedUsers, cfg.SkipPermissions)

	b, err := bot.New(cfg.Telegram.Token, sm, cfg.Telegram.AllowedUsers, Version)
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

func ReadPid() (int, bool) {
	data, err := os.ReadFile(PidFile)
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
