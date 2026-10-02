package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitpkg "gitgo/git"
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

// SetOfficeHours saves the user's working hours, used by "Only office hours"
// when spreading commits.
func (app *App) SetOfficeHours(hours OfficeHours) error {
	if _, err := parseOfficeHours(hours); err != nil {
		return err
	}
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.OfficeHours = hours
	return app.saveSettings(settings)
}

// SetStaleFetchDays saves how many days after the last fetch the app warns
// that the pushed / unpushed split may be out of date. 0 turns it off.
func (app *App) SetStaleFetchDays(days int) error {
	if !validStaleFetchDays(days) {
		return fmt.Errorf("stale fetch days must be between 0 and %d", maxStaleFetchDays)
	}
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.StaleFetchDays = days
	return app.saveSettings(settings)
}

// SetBackupSettings saves whether the confirm dialog's "Back up first"
// checkbox starts ticked, and how many automatic backups to keep per branch.
func (app *App) SetBackupSettings(backupBeforeApply bool, autoBackupsKept int) error {
	if !validAutoBackupsKept(autoBackupsKept) {
		return fmt.Errorf("automatic backups kept must be between 1 and %d", maxAutoBackupsKept)
	}
	app.settingsMutex.Lock()
	defer app.settingsMutex.Unlock()

	settings := app.loadSettings()
	settings.BackupBeforeApply = backupBeforeApply
	settings.AutoBackupsKept = autoBackupsKept
	return app.saveSettings(settings)
}

// The last 20 automatic backups of a branch are kept by default. At least one
// is always kept, or the backup made before an edit would be pruned at once.
const (
	defaultAutoBackupsKept = 20
	maxAutoBackupsKept     = 1000
)

func validAutoBackupsKept(count int) bool {
	return count >= 1 && count <= maxAutoBackupsKept
}

// A week without a fetch counts as stale by default.
const (
	defaultStaleFetchDays = 7
	maxStaleFetchDays     = 365
)

func validStaleFetchDays(days int) bool {
	return days >= 0 && days <= maxStaleFetchDays
}

// defaultOfficeHours are Monday to Friday, 09:00 to 17:00.
func defaultOfficeHours() OfficeHours {
	return OfficeHours{Start: "09:00", End: "17:00", Days: []int{1, 2, 3, 4, 5}}
}

// parseOfficeHours converts hours to git.OfficeHours, checking that the
// times are "HH:MM", the days are 0 (Sunday) to 6, and the result is valid.
func parseOfficeHours(hours OfficeHours) (gitpkg.OfficeHours, error) {
	var parsed gitpkg.OfficeHours
	for _, field := range []struct {
		text   string
		target *time.Duration
	}{{hours.Start, &parsed.Start}, {hours.End, &parsed.End}} {
		clock, err := time.Parse("15:04", field.text)
		if err != nil {
			return gitpkg.OfficeHours{}, fmt.Errorf("invalid office hours time %q, want HH:MM", field.text)
		}
		*field.target = time.Duration(clock.Hour())*time.Hour + time.Duration(clock.Minute())*time.Minute
	}
	for _, day := range hours.Days {
		if day < 0 || day > 6 {
			return gitpkg.OfficeHours{}, fmt.Errorf("invalid office hours day %d, want 0 (Sunday) to 6", day)
		}
		parsed.Days[day] = true
	}
	return parsed, parsed.Validate()
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
		OfficeHours:  defaultOfficeHours(),

		StaleFetchDays: defaultStaleFetchDays,

		BackupBeforeApply: true,
		AutoBackupsKept:   defaultAutoBackupsKept,
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
	if _, err := parseOfficeHours(settings.OfficeHours); err != nil {
		settings.OfficeHours = defaultOfficeHours()
	}
	if !validGuide(settings.BodyGuide) {
		settings.BodyGuide = defaultBodyGuide
	}
	if !validStaleFetchDays(settings.StaleFetchDays) {
		settings.StaleFetchDays = defaultStaleFetchDays
	}
	if !validAutoBackupsKept(settings.AutoBackupsKept) {
		settings.AutoBackupsKept = defaultAutoBackupsKept
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
