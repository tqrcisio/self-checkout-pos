package updater

import (
	"log"
	"os"
	"path/filepath"
)

// PruneAppliedStageDir removes <exeDir>/.update/<currentVersion>/ on boot.
// updater.exe cannot delete its own running file from finalize, so a
// successfully-applied stage dir is left behind containing the (now-stopped)
// updater.exe binary. By the time server.exe boots, that updater.exe has
// exited and the directory is free to remove.
//
// Safe to call on every boot: no-op when the directory is absent.
func PruneAppliedStageDir(exeDir string) {
	v := Version()
	if v == "dev" {
		return
	}
	path := filepath.Join(exeDir, ".update", v)
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := os.RemoveAll(path); err != nil {
		log.Printf("cleanup: remove %s: %v", path, err)
	}
}
