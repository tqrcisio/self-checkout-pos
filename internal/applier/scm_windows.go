//go:build windows

package applier

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SCM struct{}

func NewSCM() SCM { return SCM{} }

// Stop sends Stop to the service and waits until SCM reports STOPPED.
// Times out after ~30s.
func (SCM) Stop(name string) error {
	if _, err := exec.Command("sc.exe", "stop", name).CombinedOutput(); err != nil {
		if !isAlreadyStopped(err) {
			return fmt.Errorf("sc.exe stop %s: %w", name, err)
		}
	}
	return waitState(name, "STOPPED", 30*time.Second)
}

// Start sends Start to the service.
func (SCM) Start(name string) error {
	out, err := exec.Command("sc.exe", "start", name).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "1056") {
			// 1056 = An instance of the service is already running.
			return nil
		}
		return fmt.Errorf("sc.exe start %s: %w: %s", name, err, string(out))
	}
	return nil
}

func isAlreadyStopped(err error) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		// 1062 = The service has not been started.
		return strings.Contains(string(ee.Stderr), "1062")
	}
	return false
}

func waitState(name, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("sc.exe", "query", name).CombinedOutput()
		if err == nil && strings.Contains(string(out), "STATE              : 1  "+target) {
			return nil
		}
		if err == nil && strings.Contains(string(out), target) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service %s to reach state %s", name, target)
}
