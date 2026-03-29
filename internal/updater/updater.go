package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const repo = "ljdongz/codegate"

// CheckLatestVersion returns the latest release tag (e.g. "v0.1.0") or an error.
func CheckLatestVersion() (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return "", fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %w", err)
	}
	return release.TagName, nil
}

// DoUpdate downloads and replaces the binary with the given release tag.
func DoUpdate(tagName string) error {
	filename := fmt.Sprintf("codegate_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tagName, filename)

	dlResp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", dlResp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "codegate-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, filename)
	f, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		return fmt.Errorf("failed to download binary: %w", err)
	}
	f.Close()

	if err := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir).Run(); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find current binary: %w", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	newBin := filepath.Join(tmpDir, "codegate")
	if err := os.Rename(newBin, exe); err != nil {
		// In-place overwrite breaks macOS code signing (taskgated kills the binary).
		// Remove the old file first, then create a new one to preserve a valid signature.
		if err2 := os.Remove(exe); err2 != nil {
			return fmt.Errorf("failed to remove old binary (try with sudo): %w", err2)
		}
		src, err2 := os.Open(newBin)
		if err2 != nil {
			return fmt.Errorf("failed to open new binary: %w", err2)
		}
		defer src.Close()
		dst, err2 := os.OpenFile(exe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
		if err2 != nil {
			return fmt.Errorf("failed to create binary (try with sudo): %w", err2)
		}
		if _, err2 := io.Copy(dst, src); err2 != nil {
			dst.Close()
			return fmt.Errorf("failed to copy binary: %w", err2)
		}
		dst.Close()
	}

	return nil
}
