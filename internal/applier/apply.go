package applier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	SentinelTTL        = 10 * time.Minute
	healthPollWindow   = 60 * time.Second
	healthPollInterval = 2 * time.Second
)

type Config struct {
	ServiceName string
	ExeDir      string
	NewExe      string
	NewUpdater  string
	FromVersion string
	ToVersion   string
	HealthURL   string
}

type Restarter interface {
	Stop(name string) error
	Start(name string) error
}

// Apply performs the full update cycle: stop service, swap binaries, start
// service, poll /health for the expected version, then either finalize or
// rollback.
func Apply(ctx context.Context, cfg Config, r Restarter) (UpdateStatus, error) {
	log.Printf("applier: %s -> %s, stopping service %s", cfg.FromVersion, cfg.ToVersion, cfg.ServiceName)
	if err := r.Stop(cfg.ServiceName); err != nil {
		return failed(cfg, fmt.Errorf("stop service: %w", err)), err
	}

	if err := swap(cfg); err != nil {
		_ = r.Start(cfg.ServiceName)
		return failed(cfg, fmt.Errorf("swap binaries: %w", err)), err
	}

	if err := WriteSentinel(cfg.ExeDir, Sentinel{
		FromVersion: cfg.FromVersion,
		ToVersion:   cfg.ToVersion,
		StartedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(SentinelTTL),
	}); err != nil {
		log.Printf("applier: write sentinel: %v (continuing)", err)
	}

	log.Printf("applier: starting service")
	if err := r.Start(cfg.ServiceName); err != nil {
		log.Printf("applier: start failed, rolling back: %v", err)
		return rollback(ctx, cfg, r, fmt.Errorf("start service: %w", err)), nil
	}

	if err := pollHealth(ctx, cfg.HealthURL, cfg.ToVersion); err != nil {
		log.Printf("applier: health poll failed, rolling back: %v", err)
		return rollback(ctx, cfg, r, err), nil
	}

	finalize(cfg)
	st := UpdateStatus{
		FromVersion: cfg.FromVersion,
		ToVersion:   cfg.ToVersion,
		AppliedAt:   time.Now().UTC(),
		Status:      "ok",
	}
	_ = WriteStatus(cfg.ExeDir, st)
	log.Printf("applier: finalized %s -> %s", cfg.FromVersion, cfg.ToVersion)
	return st, nil
}

func swap(cfg Config) error {
	exePath := filepath.Join(cfg.ExeDir, "server.exe")
	exeOld := filepath.Join(cfg.ExeDir, "server.exe.old")
	updPath := filepath.Join(cfg.ExeDir, "updater.exe")
	updOld := filepath.Join(cfg.ExeDir, "updater.exe.old")

	for _, p := range []string{exeOld, updOld} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale %s: %w", p, err)
		}
	}

	if err := os.Rename(exePath, exeOld); err != nil {
		return fmt.Errorf("rename server.exe -> .old: %w", err)
	}
	if err := copyFile(cfg.NewExe, exePath); err != nil {
		_ = os.Rename(exeOld, exePath)
		return fmt.Errorf("copy new server.exe: %w", err)
	}

	if _, err := os.Stat(updPath); err == nil {
		if err := os.Rename(updPath, updOld); err != nil {
			_ = os.Remove(exePath)
			_ = os.Rename(exeOld, exePath)
			return fmt.Errorf("rename updater.exe -> .old: %w", err)
		}
	}
	if err := copyFile(cfg.NewUpdater, updPath); err != nil {
		_ = os.Remove(updPath)
		_ = os.Rename(updOld, updPath)
		_ = os.Remove(exePath)
		_ = os.Rename(exeOld, exePath)
		return fmt.Errorf("copy new updater.exe: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

type healthBody struct {
	Version string `json:"version"`
}

func pollHealth(ctx context.Context, url, expectedVersion string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(healthPollWindow)
	var last error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err != nil {
			last = err
			time.Sleep(healthPollInterval)
			continue
		}
		var body healthBody
		dec := json.NewDecoder(resp.Body)
		decErr := dec.Decode(&body)
		resp.Body.Close()
		if decErr == nil && body.Version == expectedVersion {
			return nil
		}
		last = fmt.Errorf("got version=%q want=%q (decode err: %v)", body.Version, expectedVersion, decErr)
		time.Sleep(healthPollInterval)
	}
	if last == nil {
		last = fmt.Errorf("health did not report version %s within %s", expectedVersion, healthPollWindow)
	}
	return fmt.Errorf("health poll: %w", last)
}

func finalize(cfg Config) {
	for _, name := range []string{"server.exe.old", "updater.exe.old"} {
		p := filepath.Join(cfg.ExeDir, name)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("applier: remove %s: %v", name, err)
		}
	}
	stage := filepath.Join(cfg.ExeDir, ".update", cfg.ToVersion)
	if err := os.RemoveAll(stage); err != nil {
		log.Printf("applier: remove %s: %v", stage, err)
	}
	_ = RemoveSentinel(cfg.ExeDir)
}

func rollback(ctx context.Context, cfg Config, r Restarter, cause error) UpdateStatus {
	_ = r.Stop(cfg.ServiceName)

	exePath := filepath.Join(cfg.ExeDir, "server.exe")
	exeOld := filepath.Join(cfg.ExeDir, "server.exe.old")
	updPath := filepath.Join(cfg.ExeDir, "updater.exe")
	updOld := filepath.Join(cfg.ExeDir, "updater.exe.old")
	failedExe := filepath.Join(cfg.ExeDir, "server.exe.failed-"+cfg.ToVersion)
	failedUpd := filepath.Join(cfg.ExeDir, "updater.exe.failed-"+cfg.ToVersion)

	if err := os.Rename(exePath, failedExe); err != nil {
		log.Printf("applier: rename failed server.exe: %v", err)
	}
	if err := os.Rename(exeOld, exePath); err != nil {
		log.Printf("applier: restore server.exe.old: %v (manual recovery required)", err)
	}
	if _, err := os.Stat(updOld); err == nil {
		_ = os.Rename(updPath, failedUpd)
		_ = os.Rename(updOld, updPath)
	}

	if err := r.Start(cfg.ServiceName); err != nil {
		log.Printf("applier: start after rollback: %v", err)
	}

	st := UpdateStatus{
		FromVersion: cfg.FromVersion,
		ToVersion:   cfg.ToVersion,
		AttemptedAt: time.Now().UTC(),
		Status:      "rolled_back",
		Error:       cause.Error(),
	}
	_ = WriteStatus(cfg.ExeDir, st)
	_ = RemoveSentinel(cfg.ExeDir)
	_ = ctx
	return st
}

func failed(cfg Config, cause error) UpdateStatus {
	st := UpdateStatus{
		FromVersion: cfg.FromVersion,
		ToVersion:   cfg.ToVersion,
		AttemptedAt: time.Now().UTC(),
		Status:      "failed",
		Error:       cause.Error(),
	}
	_ = WriteStatus(cfg.ExeDir, st)
	return st
}
