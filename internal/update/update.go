package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "cam"
	repoName  = "cine-cli"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
	Body string `json:"body"`
}

type Checker struct {
	client    *http.Client
	currentVersion string
	repoOwner string
	repoName  string
}

func New(currentVersion string) *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		currentVersion: currentVersion,
		repoOwner:      repoOwner,
		repoName:       repoName,
	}
}

func (c *Checker) Check(ctx context.Context) (*Release, bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", c.repoOwner, c.repoName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, false, nil
	}
	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, err
	}

	if release.TagName == "" {
		return nil, false, nil
	}

	hasUpdate := isNewerVersion(release.TagName, c.currentVersion)
	return &release, hasUpdate, nil
}

// isNewerVersion does a basic semver comparison (major.minor.patch).
// Accepts optional leading "v". Returns true if remote is strictly greater than current.
func isNewerVersion(remote, current string) bool {
	r := parseVersion(remote)
	c := parseVersion(current)
	for i := 0; i < 3; i++ {
		if r[i] > c[i] {
			return true
		}
		if r[i] < c[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var parts [3]int
	segs := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(segs); i++ {
		n, _ := strconv.Atoi(segs[i])
		parts[i] = n
	}
	return parts
}

func (c *Checker) Download(ctx context.Context, release *Release, destPath string) error {
	if release == nil {
		return fmt.Errorf("no release specified")
	}

	assetName := fmt.Sprintf("cine-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		for _, asset := range release.Assets {
			if asset.Name == "checksums.txt" {
				continue
			}
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no suitable asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Chmod(destPath, 0755)
}

func (c *Checker) VerifyChecksum(binaryPath, expectedSHA256 string) (bool, error) {
	f, err := os.Open(binaryPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}

	got := hex.EncodeToString(h.Sum(nil))
	return got == expectedSHA256, nil
}

func (c *Checker) ReplaceBinary(newPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	backupPath := execPath + ".bak"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	if err := os.Rename(newPath, execPath); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("replace failed: %w", err)
	}

	os.Remove(backupPath)
	return nil
}

func (c *Checker) Restart() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restart: %w", err)
	}

	os.Exit(0)
	return nil
}

func (c *Checker) CurrentVersion() string {
	return c.currentVersion
}

func BinaryPath() string {
	path, err := os.Executable()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".local", "bin", "cine")
	}
	return path
}

func DefaultDownloadPath() string {
	tmpDir := filepath.Join(os.TempDir(), "cine-update")
	_ = os.MkdirAll(tmpDir, 0755)
	return filepath.Join(tmpDir, "cine")
}
