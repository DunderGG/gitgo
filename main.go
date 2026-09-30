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

var (
	// defaultWindowSize fits the edit panel without scrolling: 72 characters
	// of the message field next to the commit list, and the panel's usual
	// fields (839px of page) under the title bar. FitToScreen shrinks it on
	// smaller screens.
	defaultWindowSize = app.WindowSize{Width: 1300, Height: 880}
	// Below this size the header (path + branch selector) and the commit
	// list columns no longer fit.
	minimumWindowSize = app.WindowSize{Width: 900, Height: 600}
)

func main() {
	application := app.New()
	window := app.LoadWindow(defaultWindowSize, minimumWindowSize)
	size := window.Size()
	startState := options.Normal
	if size.Maximised {
		startState = options.Maximised
	}

	err := wails.Run(&options.App{
		Title:            "GitGo",
		Width:            size.Width,
		Height:           size.Height,
		WindowStartState: startState,
		MinWidth:         minimumWindowSize.Width,
		MinHeight:        minimumWindowSize.Height,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Matches the frontend's bg-gray-900 so there is no flash while loading.
		BackgroundColour: &options.RGBA{R: 17, G: 24, B: 39, A: 1},
		OnStartup:        application.Startup,
		OnDomReady:       window.FitToScreen,
		OnBeforeClose:    window.Save,
		Bind: []interface{}{
			application,
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
