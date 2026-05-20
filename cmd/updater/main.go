package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/tqrcisio/self-checkout-pos/internal/applier"
	"github.com/tqrcisio/self-checkout-pos/internal/config"
	"github.com/tqrcisio/self-checkout-pos/internal/updater"
)

var version = "dev"

func main() {
	serviceName := flag.String("service", updater.ServiceName, "Windows service name")
	exeDir := flag.String("exe-dir", "", "Installation directory")
	newExe := flag.String("new-exe", "", "Path to staged new main binary")
	newUpdater := flag.String("new-updater", "", "Path to staged new helper binary")
	fromVersion := flag.String("from-version", "", "Version being replaced")
	toVersion := flag.String("to-version", "", "Version being installed")
	healthURL := flag.String("health-url", fmt.Sprintf("http://localhost:%d/health", config.DefaultPort), "Server health endpoint")
	logFile := flag.String("log", "", "Optional log file path (default: <exe-dir>/updater.log)")
	flag.Parse()

	if *serviceName == "" || *exeDir == "" || *newExe == "" || *newUpdater == "" || *fromVersion == "" || *toVersion == "" {
		log.Fatalf("missing required args; see -h")
	}

	logPath := *logFile
	if logPath == "" {
		logPath = filepath.Join(*exeDir, "updater.log")
	}
	if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Printf("%s %s starting: %s -> %s", applier.UpdaterName, version, *fromVersion, *toVersion)

	// Brief wait so the main binary has time to exit its HTTP handler before
	// we stop the service. Not strictly required (Stop is idempotent) but
	// cleaner.
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg := applier.Config{
		ServiceName: *serviceName,
		ExeDir:      *exeDir,
		NewExe:      *newExe,
		NewUpdater:  *newUpdater,
		FromVersion: *fromVersion,
		ToVersion:   *toVersion,
		HealthURL:   *healthURL,
	}

	st, err := applier.Apply(ctx, cfg, applier.NewSCM())
	if err != nil {
		log.Printf("apply: %v", err)
		os.Exit(2)
	}
	log.Printf("apply finished: status=%s", st.Status)
}
