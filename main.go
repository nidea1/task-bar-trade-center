package main

import (
	"embed"
	"fmt"

	core "github.com/nidea1/task-bar-trade-center/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsOptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if core.RunRestartAfterUpdateHelper() || core.RunRestartAfterElevationHelper() {
		return
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:             core.AppName,
		Width:             1180,
		Height:            820,
		MinWidth:          960,
		MinHeight:         680,
		StartHidden:       true,
		HideWindowOnClose: true,
		Frameless:         true,
		Windows: &windowsOptions.Options{
			WindowClassName: core.DashboardWindowClassName,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "task-bar-trade-center",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.showDashboard()
			},
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Println("Error:", err.Error())
	}
}
