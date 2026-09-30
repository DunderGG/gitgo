package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// configFile is the path of the named file in GitGo's config directory
// (gitgo/ under os.UserConfigDir), or empty when the system has none.
func configFile(name string) string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "gitgo", name)
}

// readJSONFile decodes the JSON file at path into value. A missing file is
// not an error; value is then left unchanged.
func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

// writeJSONFile writes value to path as JSON, creating the directory. It
// writes a temporary file and renames it, so a crash never leaves a
// half-written file behind.
func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	_, writeErr := temp.Write(append(data, '\n'))
	closeErr := temp.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil {
		writeErr = os.Rename(temp.Name(), path)
	}
	if writeErr != nil {
		os.Remove(temp.Name())
	}
	return writeErr
}

// GetSettings returns the saved settings. A missing or unreadable settings
// file gives the defaults, so the app always starts.
func (app *App) GetSettings() Settings {
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()
	return app.loadSettings()
}

// SetRecentRepos saves the list of recently opened repositories, newest
// first.
func (app *App) SetRecentRepos(paths []string) error {
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.RecentRepos = paths
	return app.saveSettings(settings)
}

// loadSettings reads the settings file. The caller holds settingsMutex.
func (app *App) loadSettings() Settings {
	settings := Settings{}
	if app.settingsPath != "" && readJSONFile(app.settingsPath, &settings) != nil {
		settings = Settings{}
	}
	if settings.RecentRepos == nil {
		// An empty list rather than null for the frontend.
		settings.RecentRepos = []string{}
	}
	return settings
}

// saveSettings writes the settings file. The caller holds settingsMutex.
func (app *App) saveSettings(settings Settings) error {
	if app.settingsPath == "" {
		return nil
	}
	return writeJSONFile(app.settingsPath, settings)
}
