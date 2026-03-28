package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Uninstall() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("This will remove all codegate data:")
	fmt.Println("  - Stop running codegate and all tmux sessions")
	fmt.Printf("  - %s/ (config, logs, PID file)\n", CodegateDir)
	fmt.Println("  - ~/.claude/channels/telegram/.env")
	fmt.Println("  - ~/.claude/channels/telegram/access.json")
	exe, _ := os.Executable()
	fmt.Printf("  - %s (binary)\n", exe)
	fmt.Println()
	fmt.Print("Continue? (y/N): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Cancelled.")
		return
	}

	if _, running := ReadPid(); running {
		fmt.Println("Stopping codegate...")
		Stop()
	}

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
		CodegateDir,
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

	if exe, err := os.Executable(); err == nil {
		if err := os.Remove(exe); err == nil {
			fmt.Printf("  Removed: %s\n", exe)
		}
	}

	fmt.Println()
	fmt.Println("codegate uninstalled.")
	fmt.Println("Telegram plugin can be removed manually: claude /plugin uninstall telegram@claude-plugins-official")
}
