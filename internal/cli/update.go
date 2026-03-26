package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Update(version string) {
	const repo = "ljdongz/codegate"

	fmt.Println("Checking for updates...")

	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check for updates: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse release info: %v\n", err)
		os.Exit(1)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(version, "v")
	if current == latest {
		fmt.Printf("Already up to date (%s).\n", version)
		return
	}
	fmt.Printf("Current: %s → Latest: %s\n", version, release.TagName)

	filename := fmt.Sprintf("codegate_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, release.TagName, filename)

	dlResp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download: %v\n", err)
		os.Exit(1)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Download failed: HTTP %d\n", dlResp.StatusCode)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "codegate-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, filename)
	f, err := os.Create(tarPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "Failed to download binary: %v\n", err)
		os.Exit(1)
	}
	f.Close()

	if err := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract: %v\n", err)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find current binary: %v\n", err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	newBin := filepath.Join(tmpDir, "codegate")
	if err := os.Rename(newBin, exe); err != nil {
		src, err := os.Open(newBin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open new binary: %v\n", err)
			os.Exit(1)
		}
		defer src.Close()
		dst, err := os.OpenFile(exe, os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write binary (try with sudo): %v\n", err)
			os.Exit(1)
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			fmt.Fprintf(os.Stderr, "Failed to copy binary: %v\n", err)
			os.Exit(1)
		}
		dst.Close()
	}

	fmt.Printf("Updated to %s.\n", release.TagName)
}
