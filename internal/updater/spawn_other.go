//go:build !windows

package updater

import "os/exec"

func detachedCmd(path string, args ...string) *exec.Cmd {
	return exec.Command(path, args...)
}
