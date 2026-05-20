package updater

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tqrcisio/self-checkout-pos/internal/applier"
)

// EnsureUpdaterBinary makes sure the helper binary is present next to the
// main binary. If missing (e.g. an install upgraded from a version that did
// not ship the helper), it is downloaded from the matching release using
// the running version's tag. Failure is non-fatal; auto-update is skipped
// until the file is present.
func EnsureUpdaterBinary(exeDir string) error {
	target := filepath.Join(exeDir, applier.UpdaterName)
	if _, err := os.Stat(target); err == nil {
		return nil
	}

	v := Version()
	if v == "dev" {
		return nil
	}

	log.Printf("updater: bootstrap - %s missing, downloading for %s", applier.UpdaterName, v)

	url := releaseAssetURL(v, applier.UpdaterName)
	sumURL := releaseAssetURL(v, applier.UpdaterName+".sha256")
	tmp := filepath.Join(exeDir, ".update", "bootstrap")
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	defer os.RemoveAll(tmp)

	exe := filepath.Join(tmp, applier.UpdaterName)
	sum := filepath.Join(tmp, applier.UpdaterName+".sha256")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 60 * time.Second}

	if err := fetchAndVerify(ctx, client, url, sumURL, exe, sum); err != nil {
		return fmt.Errorf("fetch %s: %w", applier.UpdaterName, err)
	}

	if err := os.Rename(exe, target); err != nil {
		return fmt.Errorf("install %s: %w", applier.UpdaterName, err)
	}
	log.Printf("updater: bootstrap - %s installed (%s)", applier.UpdaterName, target)
	return nil
}
