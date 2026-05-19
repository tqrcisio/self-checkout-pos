package service

import (
	"fmt"
	"log"

	"github.com/kardianos/service"

	"github.com/tqrcisio/self-checkout-pos/internal/config"
	"github.com/tqrcisio/self-checkout-pos/internal/server"
	"github.com/tqrcisio/self-checkout-pos/internal/updater"
)

const (
	serviceDisplayName = "Self-Checkout POS"
	serviceDescription = "HTTP service with out-of-process auto-updater."
)

type program struct {
	srv     *server.Server
	updater *updater.Updater
	svc     service.Service
}

func (p *program) Start(s service.Service) error {
	log.Printf("service: starting")

	exeDir, err := config.ExecutableDir()
	if err != nil {
		log.Printf("ExecutableDir: %v (auto-update disabled this boot)", err)
	} else {
		updater.PruneAppliedStageDir(exeDir)
		if err := updater.EnsureUpdaterBinary(exeDir); err != nil {
			log.Printf("warn: bootstrap updater.exe: %v (auto-update disabled until fixed)", err)
		}
	}

	go func() {
		if err := p.srv.Start(); err != nil {
			log.Printf("server start error: %v", err)
		}
	}()

	if exeDir != "" {
		p.updater = updater.Start(exeDir, p.srv.Config)
		p.srv.SetUpdater(p.updater)
	}

	return nil
}

func (p *program) Stop(s service.Service) error {
	log.Printf("service: stopping")
	if p.updater != nil {
		p.updater.Stop()
	}
	p.srv.Stop()
	return nil
}

func Run(action string) error {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config load error: %v, using defaults", err)
	}

	svcConfig := &service.Config{
		Name:        updater.ServiceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
	}

	prg := &program{srv: server.New(cfg)}

	svc, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("service.New: %w", err)
	}
	prg.svc = svc

	switch action {
	case "install":
		if err := svc.Install(); err != nil {
			return err
		}
		if err := configureServiceRecovery(updater.ServiceName); err != nil {
			log.Printf("warn: configure service recovery: %v (run sc.exe failure manually)", err)
		}
		return nil
	case "uninstall":
		return svc.Uninstall()
	case "start":
		return svc.Start()
	case "stop":
		return svc.Stop()
	case "run", "":
		return svc.Run()
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}
