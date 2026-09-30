package app

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// WindowSize is the main window's outer size in logical pixels, the unit
// Wails uses for window sizes at any display scaling.
type WindowSize struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

// taskbarAllowance is kept free below the window when fitting it to the
// screen, since Wails reports the whole screen rather than its work area.
const taskbarAllowance = 48

// Window remembers the main window's size between runs. It is not bound to
// the frontend; main passes its methods to Wails as lifecycle hooks.
type Window struct {
	// path is the settings file, or empty when there is no config directory.
	path    string
	minimum WindowSize
	size    WindowSize
}

// LoadWindow reads the size saved by the previous run, falling back to
// defaultSize when there is none, and never returns less than minimum.
func LoadWindow(defaultSize, minimum WindowSize) *Window {
	path := configFile("window.json")
	return &Window{path: path, minimum: minimum, size: loadWindowSize(path, defaultSize, minimum)}
}

// Size is the size to open the window at.
func (window *Window) Size() WindowSize {
	return window.size
}

// FitToScreen shrinks the window when it is larger than the screen it opened
// on, e.g. the default size on a small laptop or a size saved on a larger
// monitor. Call it once the window exists (OnDomReady).
func (window *Window) FitToScreen(ctx context.Context) {
	if window.size.Maximised {
		return
	}
	screens, err := runtime.ScreenGetAll(ctx)
	if err != nil {
		return
	}
	for _, screen := range screens {
		if !screen.IsCurrent {
			continue
		}
		fitted := fitWindowSize(window.size, screen.Size.Width, screen.Size.Height-taskbarAllowance, window.minimum)
		if fitted != window.size {
			runtime.WindowSetSize(ctx, fitted.Width, fitted.Height)
			runtime.WindowCenter(ctx)
		}
		return
	}
}

// Save stores the window's current size for the next run. It has the
// signature of OnBeforeClose and never prevents closing.
func (window *Window) Save(ctx context.Context) bool {
	switch {
	case runtime.WindowIsMinimised(ctx):
		// A minimised window has no useful size; keep the last one.
	case runtime.WindowIsMaximised(ctx):
		// Keep the normal size so un-maximising next time restores it.
		window.size.Maximised = true
	default:
		width, height := runtime.WindowGetSize(ctx)
		window.size = WindowSize{Width: width, Height: height}
	}
	saveWindowSize(window.path, window.size)
	return false
}

// loadWindowSize reads the size saved at path. A missing or unreadable file
// gives defaultSize.
func loadWindowSize(path string, defaultSize, minimum WindowSize) WindowSize {
	size := defaultSize
	if path != "" {
		var saved WindowSize
		if readJSONFile(path, &saved) == nil && saved.Width > 0 && saved.Height > 0 {
			size = saved
		}
	}
	size.Width = max(size.Width, minimum.Width)
	size.Height = max(size.Height, minimum.Height)
	return size
}

// saveWindowSize writes size to path. Failures are ignored: the next run
// then opens at the default size.
func saveWindowSize(path string, size WindowSize) {
	if path == "" {
		return
	}
	_ = writeJSONFile(path, size)
}

// fitWindowSize shrinks size to at most maxWidth by maxHeight, but not below
// minimum.
func fitWindowSize(size WindowSize, maxWidth, maxHeight int, minimum WindowSize) WindowSize {
	size.Width = max(min(size.Width, maxWidth), minimum.Width)
	size.Height = max(min(size.Height, maxHeight), minimum.Height)
	return size
}
