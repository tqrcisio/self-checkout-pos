package applier

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func sentinelPath(exeDir string) string {
	return filepath.Join(exeDir, ".update", "sentinel.json")
}

func statusPath(exeDir string) string {
	return filepath.Join(exeDir, ".update", "status.json")
}

func WriteSentinel(exeDir string, s Sentinel) error {
	if err := os.MkdirAll(filepath.Join(exeDir, ".update"), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sentinelPath(exeDir), data, 0644)
}

func ReadSentinel(exeDir string) (Sentinel, bool, error) {
	data, err := os.ReadFile(sentinelPath(exeDir))
	if errors.Is(err, os.ErrNotExist) {
		return Sentinel{}, false, nil
	}
	if err != nil {
		return Sentinel{}, false, err
	}
	var s Sentinel
	if err := json.Unmarshal(data, &s); err != nil {
		return Sentinel{}, false, err
	}
	return s, true, nil
}

func RemoveSentinel(exeDir string) error {
	err := os.Remove(sentinelPath(exeDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func WriteStatus(exeDir string, st UpdateStatus) error {
	if err := os.MkdirAll(filepath.Join(exeDir, ".update"), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statusPath(exeDir), data, 0644)
}

func ReadStatus(exeDir string) (UpdateStatus, bool, error) {
	data, err := os.ReadFile(statusPath(exeDir))
	if errors.Is(err, os.ErrNotExist) {
		return UpdateStatus{}, false, nil
	}
	if err != nil {
		return UpdateStatus{}, false, err
	}
	var st UpdateStatus
	if err := json.Unmarshal(data, &st); err != nil {
		return UpdateStatus{}, false, err
	}
	return st, true, nil
}
