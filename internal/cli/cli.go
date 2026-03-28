package cli

import (
	"log"
	"os"
	"path/filepath"
)

var (
	Version     string
	CodegateDir string
	PidFile     string
	LogFile     string
)

func init() {
	if dir := os.Getenv("CODEGATE_CONFIG_DIR"); dir != "" {
		CodegateDir = dir
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("cannot determine home directory: %v", err)
		}
		CodegateDir = filepath.Join(home, ".codegate")
	}
	PidFile = filepath.Join(CodegateDir, "codegate.pid")
	LogFile = filepath.Join(CodegateDir, "codegate.log")
}
