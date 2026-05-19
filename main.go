package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "dev"

func main() {
	app := NewApp(version)

	err := wails.Run(&options.App{
		Title:  "Self-Checkout POS",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.OnStartup,
		OnShutdown:       app.OnShutdown,
		OnDomReady:       app.OnDomReady,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}
