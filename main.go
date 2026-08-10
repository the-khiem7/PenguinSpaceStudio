package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if handled, err := runElevatedHelper(os.Args[1:]); handled {
		if err != nil {
			log.Print(err)
		}
		return
	}

	service, err := NewAppService()
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()

	app := application.New(application.Options{
		Name:        "PenguinSpace",
		Description: "Windows-first developer storage manager",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "PenguinSpace",
		Width:            1440,
		Height:           900,
		BackgroundColour: application.NewRGB(18, 24, 36),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
