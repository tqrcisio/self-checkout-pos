package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tqrcisio/self-checkout-pos/internal/applier"
)

type stagedDownload struct {
	ExePath        string
	UpdaterExePath string
	Version        string
}

type downloadManifest struct {
	Version      string    `json:"version"`
	DownloadedAt time.Time `json:"downloaded_at"`
	HasUpdater   bool      `json:"has_updater"`
}

func stageDir(exeDir, version string) string {
	return filepath.Join(exeDir, ".update", version)
}

// IsAlreadyStaged returns true if a valid download for `version` exists on
// disk: manifest present, main binary checksum matches, and (if the manifest
// expects the helper binary) its checksum also matches.
func IsAlreadyStaged(exeDir string, m LatestManifest) bool {
	dir := stageDir(exeDir, m.Version)
	manifestPath := filepath.Join(dir, "manifest.json")
	mfBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}
	var mf downloadManifest
	if json.Unmarshal(mfBytes, &mf) != nil {
		return false
	}
	if !checksumOK(filepath.Join(dir, applier.BinaryName), filepath.Join(dir, applier.BinaryName+".sha256")) {
		return false
	}
	if mf.HasUpdater && !checksumOK(filepath.Join(dir, applier.UpdaterName), filepath.Join(dir, applier.UpdaterName+".sha256")) {
		return false
	}
	return true
}

// Download fetches the main binary (and optional helper) plus their sha256
// files into the staging dir, verifying integrity. On failure the staging
// dir is removed.
func Download(ctx context.Context, client *http.Client, exeDir string, m LatestManifest) (stagedDownload, error) {
	dir := stageDir(exeDir, m.Version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return stagedDownload{}, fmt.Errorf("mkdir staging: %w", err)
	}

	exePath := filepath.Join(dir, applier.BinaryName)
	exeSum := filepath.Join(dir, applier.BinaryName+".sha256")
	if err := fetchAndVerify(ctx, client, m.DownloadURL, m.SHA256URL, exePath, exeSum); err != nil {
		os.RemoveAll(dir)
		return stagedDownload{}, fmt.Errorf("%s: %w", applier.BinaryName, err)
	}

	staged := stagedDownload{ExePath: exePath, Version: m.Version}
	hasUpdater := m.UpdaterDownloadURL != "" && m.UpdaterSHA256URL != ""
	if hasUpdater {
		updPath := filepath.Join(dir, applier.UpdaterName)
		updSum := filepath.Join(dir, applier.UpdaterName+".sha256")
		if err := fetchAndVerify(ctx, client, m.UpdaterDownloadURL, m.UpdaterSHA256URL, updPath, updSum); err != nil {
			os.RemoveAll(dir)
			return stagedDownload{}, fmt.Errorf("%s: %w", applier.UpdaterName, err)
		}
		staged.UpdaterExePath = updPath
	}

	mf := downloadManifest{Version: m.Version, DownloadedAt: time.Now().UTC(), HasUpdater: hasUpdater}
	data, _ := json.MarshalIndent(mf, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); err != nil {
		os.RemoveAll(dir)
		return stagedDownload{}, fmt.Errorf("write manifest: %w", err)
	}
	return staged, nil
}

func fetchAndVerify(ctx context.Context, client *http.Client, downloadURL, sumURL, dest, sumDest string) error {
	if err := fetchToFile(ctx, client, downloadURL, dest); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := fetchToFile(ctx, client, sumURL, sumDest); err != nil {
		return fmt.Errorf("download sha256: %w", err)
	}
	expected, err := readSHA256File(sumDest)
	if err != nil {
		return fmt.Errorf("read sha256: %w", err)
	}
	actual, err := computeSHA256(dest)
	if err != nil {
		return fmt.Errorf("compute sha256: %w", err)
	}
	if expected != actual {
		return fmt.Errorf("sha256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func checksumOK(filePath, sumPath string) bool {
	expected, err := readSHA256File(sumPath)
	if err != nil {
		return false
	}
	actual, err := computeSHA256(filePath)
	if err != nil {
		return false
	}
	return expected == actual
}

func fetchToFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return out.Sync()
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readSHA256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(data))
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		s = s[:i]
	}
	if len(s) != 64 {
		return "", fmt.Errorf("invalid sha256 length %d", len(s))
	}
	return strings.ToLower(s), nil
}
