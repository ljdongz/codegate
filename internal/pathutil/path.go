package pathutil

import (
	"fmt"
	"os"
	"strings"
)

func Expand(p string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	if p == "~" || strings.HasPrefix(p, "~/") {
		return home + p[1:], nil
	}

	if strings.HasPrefix(p, "/") {
		return p, nil
	}

	p = strings.TrimPrefix(p, "./")
	return home + "/" + p, nil
}
