package updater

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/tqrcisio/golang-boilerplate/internal/applier"
	"github.com/tqrcisio/golang-boilerplate/internal/config"
)

const (
	pollInterval = 1 * time.Hour
	windowStart  = 2
	windowEnd    = 5
	httpTimeout  = 30 * time.Second
)

// Service identifiers shared with the updater binary.
const (
	ServiceName = "golang-boilerplate"
)

type Updater struct {
	cfgGetter  func() config.Config
	exeDir     string
	httpClient *http.Client
	mu         sync.Mutex
	stop       chan struct{}
	stopped    chan struct{}
	clock      func() time.Time
	startedAt  time.Time

	pollMu        sync.Mutex
	lastPollAt    time.Time
	lastPollError string
}

type Result struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Status      string `json:"status"` // "applied" | "noop" | "pending"
}

type Status struct {
	Version           string                `json:"version"`
	AutoUpdateEnabled bool                  `json:"auto_update_enabled"`
	StartedAt         *time.Time            `json:"started_at,omitempty"`
	Uptime            string                `json:"uptime,omitempty"`
	LastPollAt        *time.Time            `json:"last_poll_at,omitempty"`
	LastPollError     string                `json:"last_poll_error,omitempty"`
	LastUpdate        *applier.UpdateStatus `json:"last_update,omitempty"`
}

func Start(exeDir string, cfgGetter func() config.Config) *Updater {
	u := &Updater{
		cfgGetter:  cfgGetter,
		exeDir:     exeDir,
		httpClient: &http.Client{Timeout: httpTimeout},
		stop:       make(chan struct{}),
		stopped:    make(chan struct{}),
		clock:      func() time.Time { return time.Now() },
		startedAt:  time.Now().UTC(),
	}
	go u.loop()
	return u
}

func (u *Updater) Stop() {
	close(u.stop)
	<-u.stopped
}

func (u *Updater) Status() Status {
	st := Status{
		Version:           Version(),
		AutoUpdateEnabled: u.cfgGetter().AutoUpdateOn(),
	}
	if !u.startedAt.IsZero() {
		t := u.startedAt
		st.StartedAt = &t
		st.Uptime = time.Since(u.startedAt).Round(time.Second).String()
	}
	u.pollMu.Lock()
	if !u.lastPollAt.IsZero() {
		t := u.lastPollAt
		st.LastPollAt = &t
		st.LastPollError = u.lastPollError
	}
	u.pollMu.Unlock()
	if last, ok, _ := applier.ReadStatus(u.exeDir); ok {
		st.LastUpdate = &last
	}
	return st
}

func (u *Updater) recordPoll(err error) {
	u.pollMu.Lock()
	defer u.pollMu.Unlock()
	u.lastPollAt = time.Now().UTC()
	if err != nil {
		u.lastPollError = err.Error()
	} else {
		u.lastPollError = ""
	}
}

func (u *Updater) loop() {
	defer close(u.stopped)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	u.tick()
	for {
		select {
		case <-u.stop:
			return
		case <-ticker.C:
			u.tick()
		}
	}
}

func (u *Updater) healthURL() string {
	cfg := u.cfgGetter()
	port := cfg.Port
	if port == 0 {
		port = config.DefaultPort
	}
	return fmt.Sprintf("http://localhost:%d/health", port)
}

func (u *Updater) tick() {
	if !u.cfgGetter().AutoUpdateOn() {
		return
	}
	if Version() == "dev" {
		log.Printf("updater: skipping poll (running dev build)")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	m, err := FetchLatest(ctx, u.httpClient)
	u.recordPoll(err)
	if err != nil {
		log.Printf("updater: poll failed: %v", err)
		return
	}
	cur := Version()
	cmp := CompareVersions(cur, m.Version)
	if cmp == 0 {
		return
	}
	if cmp > 0 {
		log.Printf("updater: refusing downgrade %s -> %s", cur, m.Version)
		return
	}

	if !IsAlreadyStaged(u.exeDir, m) {
		if _, err := u.ensureDownloaded(ctx, m); err != nil {
			log.Printf("updater: download failed: %v", err)
			return
		}
	}

	if !IsWithinWindow(u.clock(), windowStart, windowEnd) {
		log.Printf("updater: %s downloaded, waiting for window %02d:00-%02d:00", m.Version, windowStart, windowEnd)
		return
	}
	if err := u.handOffToUpdater(m); err != nil {
		log.Printf("updater: handoff failed: %v", err)
	}
}

func (u *Updater) ensureDownloaded(ctx context.Context, m LatestManifest) (stagedDownload, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if IsAlreadyStaged(u.exeDir, m) {
		return stagedDownload{
			ExePath:        filepath.Join(stageDir(u.exeDir, m.Version), "server.exe"),
			UpdaterExePath: filepath.Join(stageDir(u.exeDir, m.Version), "updater.exe"),
			Version:        m.Version,
		}, nil
	}
	return Download(ctx, u.httpClient, u.exeDir, m)
}

// handOffToUpdater spawns the staged updater.exe detached. updater.exe is
// responsible for stopping the service, swapping files, restarting, and
// finalizing (or rolling back). server.exe just exits when SCM tells it to.
func (u *Updater) handOffToUpdater(m LatestManifest) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !IsAlreadyStaged(u.exeDir, m) {
		return errors.New("no staged download for " + m.Version)
	}
	stagedUpdater := filepath.Join(stageDir(u.exeDir, m.Version), "updater.exe")
	stagedExe := filepath.Join(stageDir(u.exeDir, m.Version), "server.exe")

	cmd := detachedCmd(stagedUpdater,
		"--service", ServiceName,
		"--exe-dir", u.exeDir,
		"--new-exe", stagedExe,
		"--new-updater", stagedUpdater,
		"--from-version", Version(),
		"--to-version", m.Version,
		"--health-url", u.healthURL(),
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn updater.exe: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		log.Printf("updater: release process handle: %v", err)
	}
	log.Printf("updater: handed off to %s (pid=%d), will be stopped by it shortly", stagedUpdater, cmd.Process.Pid)
	return nil
}

// CheckAndApplyNow is the on-demand path (POST /update). Runs the full chain
// immediately, ignoring the nightly window.
func (u *Updater) CheckAndApplyNow(ctx context.Context) (Result, error) {
	if Version() == "dev" {
		return Result{}, errors.New("dev builds do not auto-update")
	}
	m, err := FetchLatest(ctx, u.httpClient)
	u.recordPoll(err)
	if err != nil {
		return Result{}, fmt.Errorf("fetch latest: %w", err)
	}
	cur := Version()
	cmp := CompareVersions(cur, m.Version)
	if cmp == 0 {
		return Result{FromVersion: cur, ToVersion: m.Version, Status: "noop"}, nil
	}
	if cmp > 0 {
		return Result{}, fmt.Errorf("refusing downgrade %s -> %s", cur, m.Version)
	}
	if !IsAlreadyStaged(u.exeDir, m) {
		if _, err := u.ensureDownloaded(ctx, m); err != nil {
			return Result{}, fmt.Errorf("download: %w", err)
		}
	}
	if err := u.handOffToUpdater(m); err != nil {
		return Result{}, fmt.Errorf("handoff: %w", err)
	}
	return Result{FromVersion: cur, ToVersion: m.Version, Status: "applied"}, nil
}
