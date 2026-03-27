package main

import (
	"fmt"
	"os"

	"github.com/ljdongz/codegate/internal/cli"
)

var version = "dev"

func main() {
	cli.Version = version

	if len(os.Args) < 2 {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "setup":
		cli.Setup()
	case "start":
		cli.Start()
	case "stop":
		cli.Stop()
	case "restart":
		cli.Stop()
		cli.Start()
	case "status":
		cli.Status()
	case "logs":
		cli.Logs()
	case "run":
		cli.Run()
	case "update":
		cli.Update(version)
	case "version", "-v", "--version":
		fmt.Printf("codegate %s\n", version)
	case "uninstall":
		cli.Uninstall()
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
