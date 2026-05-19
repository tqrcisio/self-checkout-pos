//go:build windows

package service

import (
	"fmt"
	"os/exec"
)

// configureServiceRecovery sets Windows Service Recovery Actions so the
// service restarts up to 3 times with a 10s delay between attempts.
//
// Equivalent CLI: sc.exe failure <name> reset= 60 actions= restart/10000/restart/10000/restart/10000
func configureServiceRecovery(name string) error {
	cmd := exec.Command("sc.exe", "failure", name,
		"reset=", "60",
		"actions=", "restart/10000/restart/10000/restart/10000",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc.exe failure: %w: %s", err, string(out))
	}
	return nil
}
