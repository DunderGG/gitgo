package main

import (
	"embed"

	"gitgo/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// icon is the application icon. Windows and macOS builds get their icon from
// build/windows/icon.ico and the generated .icns; Linux and the macOS About
// panel need it at runtime.
//
//go:embed build/appicon.png
var icon []byte

func main() {
	app := app.New()

	err := wails.Run(&options.App{
		Title:  "GitGo",
		Width:  1200,
		Height: 800,
		// Below this size the header (path + branch selector) and the commit
		// list columns no longer fit.
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Matches the frontend's bg-gray-900 so there is no flash while loading.
		BackgroundColour: &options.RGBA{R: 17, G: 24, B: 39, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
		// The UI is dark-only, so use a dark title bar on every platform.
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
		Mac: &mac.Options{
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   "GitGo",
				Message: "Edit the message, date, and author of commits you have not pushed yet.\n\nCopyright © 2026 dunder.gg",
				Icon:    icon,
			},
		},
		Linux: &linux.Options{
			Icon:        icon,
			ProgramName: "gitgo",
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
