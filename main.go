package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/Kingsley-ChenChen/MockCharles2/internal/core"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	base, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dir := os.Getenv("MOCKCHARLES_DATA_DIR")
	if dir == "" {
		dir = filepath.Join(base, "MockCharles")
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		log.Fatal(err)
	}
	service, err := core.Open(filepath.Join(dir, "config.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()
	app := &App{service: service}
	if err = wails.Run(&options.App{
		Title: "MockCharles", Width: 1380, Height: 900, MinWidth: 1080, MinHeight: 700,
		AssetServer:        &assetserver.Options{Assets: assets},
		Bind:               []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "mockcharles-desktop-6f5127b2"},
	}); err != nil {
		log.Fatal(err)
	}
}
