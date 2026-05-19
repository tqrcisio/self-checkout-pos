package main

import (
	"flag"
	"log"

	"github.com/tqrcisio/golang-boilerplate/internal/service"
	"github.com/tqrcisio/golang-boilerplate/internal/updater"
)

var version = "dev"

func main() {
	updater.SetVersion(version)

	action := flag.String("action", "", "Service action: install, uninstall, start, stop, run")
	flag.Parse()

	if err := service.Run(*action); err != nil {
		log.Fatal(err)
	}
}
