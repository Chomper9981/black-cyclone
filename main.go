package main

import (
	"embed"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"redcyclone-vpn/internal/settings"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// --autostart is passed by the logon task (see internal/autostart).
	auto := slices.Contains(os.Args[1:], "--autostart")
	hidden := false
	if s, err := settings.Load(); err == nil && auto && s.StartMinimized {
		hidden = true
	}

	app := NewApp(auto)

	err := wails.Run(&options.App{
		Title:       "Black Cyclone",
		Width:       440,
		Height:      720,
		MinWidth:    380,
		MinHeight:   560,
		StartHidden: hidden,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 20, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		// A second launch just brings the running window forward; two
		// instances would fight over the TUN interface.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "redcyclone-vpn-9f3b2c",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.ShowWindow() },
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
