//go:build !windows

package service

// configureServiceRecovery is a no-op outside Windows. The repo cross-compiles
// for Windows but local builds on darwin/linux must still compile.
func configureServiceRecovery(_ string) error {
	return nil
}
