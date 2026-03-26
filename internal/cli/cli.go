package cli

import (
	"log"
	"os"
	"path/filepath"
)

var (
	CodegateDir string
	PidFile     string
	LogFile     string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}
	CodegateDir = filepath.Join(home, ".codegate")
	PidFile = filepath.Join(CodegateDir, "codegate.pid")
	LogFile = filepath.Join(CodegateDir, "codegate.log")
}
