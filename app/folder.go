package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// fileManagerCommand builds the command that shows dir in the system file
// manager for goos: Explorer on Windows, Finder on macOS and the default file
// manager (through xdg-open) elsewhere.
func fileManagerCommand(goos string, dir string) *exec.Cmd {
	switch goos {
	case "windows":
		// Explorer does not understand forward slashes.
		return exec.Command("explorer.exe", filepath.FromSlash(dir))
	case "darwin":
		return exec.Command("open", dir)
	default:
		return exec.Command("xdg-open", dir)
	}
}

// OpenFolder shows the root of the open repository in Explorer, Finder or the
// Linux file manager. The window is not tied to GitGo.
// OpenRepository must be called before this method.
func (app *App) OpenFolder() error {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return fmt.Errorf("no repository is open; call OpenRepository first")
	}

	cmd := fileManagerCommand(runtime.GOOS, state.Path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open the repository folder: %w", err)
	}
	// Reap the process when it exits. Its exit code is ignored: explorer.exe
	// exits with 1 even when it opened the folder.
	go func() { _ = cmd.Wait() }()
	return nil
}
