package updater

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// EnsureUpdaterBinary makes sure updater.exe is present next to server.exe.
// If missing (e.g. an install upgraded from a version that did not ship the
// helper), it is downloaded from the matching release using the running
// version's tag. Failure is non-fatal; auto-update will be skipped until the
// file is present.
func EnsureUpdaterBinary(exeDir string) error {
	target := filepath.Join(exeDir, "updater.exe")
	if _, err := os.Stat(target); err == nil {
		return nil
	}

	v := Version()
	if v == "dev" {
		return nil
	}

	log.Printf("updater: bootstrap - updater.exe missing, downloading for %s", v)

	url := releaseAssetURL(v, "updater.exe")
	sumURL := releaseAssetURL(v, "updater.exe.sha256")
	tmp := filepath.Join(exeDir, ".update", "bootstrap")
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	defer os.RemoveAll(tmp)

	exe := filepath.Join(tmp, "updater.exe")
	sum := filepath.Join(tmp, "updater.exe.sha256")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 60 * time.Second}

	if err := fetchAndVerify(ctx, client, url, sumURL, exe, sum); err != nil {
		return fmt.Errorf("fetch updater.exe: %w", err)
	}

	if err := os.Rename(exe, target); err != nil {
		return fmt.Errorf("install updater.exe: %w", err)
	}
	log.Printf("updater: bootstrap - updater.exe installed (%s)", target)
	return nil
}
