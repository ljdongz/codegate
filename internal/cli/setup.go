package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ljdongz/codegate/internal/channel"
	"github.com/ljdongz/codegate/internal/config"
)

func Setup() {
	fmt.Println("=== codegate setup ===")
	fmt.Println()

	fmt.Println("Checking dependencies...")
	hasBrew := false
	if _, err := exec.LookPath("brew"); err == nil {
		hasBrew = true
	}

	type dep struct {
		name    string
		brewPkg string
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

	if err := channel.SetupAccess([]int64{userID}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write access.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✓ Setup complete!")
	fmt.Println("  1. Run 'codegate start' to start the daemon.")
	fmt.Println("  2. Send /bot_add <claude-bot-token> to the management bot to register your Claude bot.")
}
