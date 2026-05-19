package main

import (
	"flag"
	"log"

	"github.com/tqrcisio/self-checkout-pos/internal/service"
	"github.com/tqrcisio/self-checkout-pos/internal/updater"
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
