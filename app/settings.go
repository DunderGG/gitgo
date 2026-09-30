package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// SetTheme saves the colour theme: ThemeSystem, ThemeLight or ThemeDark.
func (app *App) SetTheme(theme string) error {
	if !validTheme(theme) {
		return fmt.Errorf("unknown theme %q", theme)
	}
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.Theme = theme
	return app.saveSettings(settings)
}

// SetMessageGuides saves the commit message guide columns: the subject ruler
// and longest subject, and the longest body line. 0 turns a guide off.
func (app *App) SetMessageGuides(subject, body int) error {
	if !validGuide(subject) || !validGuide(body) {
		return fmt.Errorf("message guide columns must be between 0 and %d", maxGuideColumn)
	}
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.SubjectGuide = subject
	settings.BodyGuide = body
	return app.saveSettings(settings)
}

// SetTerminalCommand saves the command the header's terminal button runs, with
// {dir} standing for the repository folder. An empty command goes back to
// picking a terminal automatically.
func (app *App) SetTerminalCommand(command string) error {
	command = strings.TrimSpace(command)
	if command != "" {
		if _, err := customTerminalLaunch(command); err != nil {
			return err
		}
	}
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.TerminalCommand = command
	return app.saveSettings(settings)
}

// The usual limits for commit messages: a 50-character subject and a body
// wrapped at 72, as git's own documentation and most GUIs suggest.
const (
	defaultSubjectGuide = 50
	defaultBodyGuide    = 72
	maxGuideColumn      = 200
)

func validGuide(column int) bool {
	return column >= 0 && column <= maxGuideColumn
}

func defaultSettings() Settings {
	return Settings{
		Theme:        ThemeSystem,
		SubjectGuide: defaultSubjectGuide,
		BodyGuide:    defaultBodyGuide,
	}
}

// loadSettings reads the settings file. The caller holds settingsMutex.
// Fields missing from the file, for example in a file from an earlier
// version, keep their defaults.
func (app *App) loadSettings() Settings {
	settings := defaultSettings()
	if app.settingsPath != "" && readJSONFile(app.settingsPath, &settings) != nil {
		settings = defaultSettings()
	}
	if settings.RecentRepos == nil {
		// An empty list rather than null for the frontend.
		settings.RecentRepos = []string{}
	}
	if !validTheme(settings.Theme) {
		settings.Theme = ThemeSystem
	}
	if !validGuide(settings.SubjectGuide) {
		settings.SubjectGuide = defaultSubjectGuide
	}
	if !validGuide(settings.BodyGuide) {
		settings.BodyGuide = defaultBodyGuide
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
