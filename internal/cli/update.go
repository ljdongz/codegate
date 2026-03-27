package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ljdongz/codegate/internal/updater"
)

// Update is the CLI entrypoint for `codegate update`.
func Update(version string) {
	fmt.Println("Checking for updates...")

	tagName, err := updater.CheckLatestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	latest := strings.TrimPrefix(tagName, "v")
	current := strings.TrimPrefix(version, "v")
	if current == latest {
		fmt.Printf("Already up to date (%s).\n", version)
		return
	}
	fmt.Printf("Current: %s → Latest: %s\n", version, tagName)

	if err := updater.DoUpdate(tagName); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to %s.\n", tagName)
}
