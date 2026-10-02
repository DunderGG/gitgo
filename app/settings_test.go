package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func appWithSettingsFile(test *testing.T) (*App, string) {
	path := filepath.Join(test.TempDir(), "gitgo", "settings.json")
	return &App{settingsPath: path}, path
}

func TestGetSettings_DefaultsWithoutFile(test *testing.T) {
	app, _ := appWithSettingsFile(test)

	settings := app.GetSettings()

	if settings.RecentRepos == nil || len(settings.RecentRepos) != 0 {
		test.Fatalf("RecentRepos = %#v, want an empty, non-nil list", settings.RecentRepos)
	}
}

func TestSetRecentRepos_SavedForNextRun(test *testing.T) {
	app, path := appWithSettingsFile(test)
	want := []string{"/repos/newest", "/repos/older"}

	if err := app.SetRecentRepos(want); err != nil {
		test.Fatalf("SetRecentRepos: %v", err)
	}

	// A new App stands in for the next run.
	nextRun := &App{settingsPath: path}
	if got := nextRun.GetSettings().RecentRepos; !reflect.DeepEqual(got, want) {
		test.Fatalf("RecentRepos = %v, want %v", got, want)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
	if len(leftovers) != 0 {
		test.Fatalf("temporary files left behind: %v", leftovers)
	}
}

func TestGetSettings_CorruptFileGivesDefaults(test *testing.T) {
	app, path := appWithSettingsFile(test)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		test.Fatal(err)
	}

	if got := app.GetSettings().RecentRepos; len(got) != 0 {
		test.Fatalf("RecentRepos = %v, want empty", got)
	}
	// Saving replaces the corrupt file.
	if err := app.SetRecentRepos([]string{"/repos/a"}); err != nil {
		test.Fatalf("SetRecentRepos: %v", err)
	}
	if got := app.GetSettings().RecentRepos; !reflect.DeepEqual(got, []string{"/repos/a"}) {
		test.Fatalf("RecentRepos after save = %v", got)
	}
}

func TestSettings_WithoutConfigDir(test *testing.T) {
	app := &App{}

	if err := app.SetRecentRepos([]string{"/repos/a"}); err != nil {
		test.Fatalf("SetRecentRepos: %v", err)
	}
	if got := app.GetSettings().RecentRepos; len(got) != 0 {
		test.Fatalf("RecentRepos = %v, want empty", got)
	}
}

func TestGetSettings_ThemeDefaultsToSystem(test *testing.T) {
	app, path := appWithSettingsFile(test)

	if got := app.GetSettings().Theme; got != ThemeSystem {
		test.Fatalf("Theme without a file = %q, want %q", got, ThemeSystem)
	}

	// A file from an earlier version, or edited by hand, may hold anything.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme": "purple"}`), 0o644); err != nil {
		test.Fatal(err)
	}
	if got := app.GetSettings().Theme; got != ThemeSystem {
		test.Fatalf("Theme with an unknown value = %q, want %q", got, ThemeSystem)
	}
}

func TestSetTheme_SavedAlongsideRecentRepos(test *testing.T) {
	app, path := appWithSettingsFile(test)
	repos := []string{"/repos/a"}
	if err := app.SetRecentRepos(repos); err != nil {
		test.Fatalf("SetRecentRepos: %v", err)
	}

	if err := app.SetTheme(ThemeLight); err != nil {
		test.Fatalf("SetTheme: %v", err)
	}

	nextRun := &App{settingsPath: path}
	settings := nextRun.GetSettings()
	if settings.Theme != ThemeLight {
		test.Fatalf("Theme = %q, want %q", settings.Theme, ThemeLight)
	}
	if !reflect.DeepEqual(settings.RecentRepos, repos) {
		test.Fatalf("RecentRepos = %v, want %v", settings.RecentRepos, repos)
	}
}

func TestSetTheme_RejectsUnknownTheme(test *testing.T) {
	app, _ := appWithSettingsFile(test)

	if err := app.SetTheme("purple"); err == nil {
		test.Fatal("SetTheme accepted an unknown theme")
	}
	if got := app.GetSettings().Theme; got != ThemeSystem {
		test.Fatalf("Theme = %q, want %q", got, ThemeSystem)
	}
}

func TestGetSettings_MessageGuideDefaults(test *testing.T) {
	app, path := appWithSettingsFile(test)

	settings := app.GetSettings()
	if settings.SubjectGuide != 50 || settings.BodyGuide != 72 {
		test.Fatalf("guides without a file = %d/%d, want 50/72", settings.SubjectGuide, settings.BodyGuide)
	}

	// A file from before the guides existed has no guide fields.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme": "dark"}`), 0o644); err != nil {
		test.Fatal(err)
	}
	settings = app.GetSettings()
	if settings.SubjectGuide != 50 || settings.BodyGuide != 72 {
		test.Fatalf("guides from an older file = %d/%d, want 50/72", settings.SubjectGuide, settings.BodyGuide)
	}

	// Out-of-range values, edited by hand, fall back to the defaults.
	if err := os.WriteFile(path, []byte(`{"subjectGuide": -1, "bodyGuide": 5000}`), 0o644); err != nil {
		test.Fatal(err)
	}
	settings = app.GetSettings()
	if settings.SubjectGuide != 50 || settings.BodyGuide != 72 {
		test.Fatalf("out-of-range guides = %d/%d, want 50/72", settings.SubjectGuide, settings.BodyGuide)
	}
}

func TestSetMessageGuides_SavedForNextRun(test *testing.T) {
	app, path := appWithSettingsFile(test)

	// 0 turns a guide off, and must survive a reload rather than become a default.
	if err := app.SetMessageGuides(0, 100); err != nil {
		test.Fatalf("SetMessageGuides: %v", err)
	}

	nextRun := &App{settingsPath: path}
	settings := nextRun.GetSettings()
	if settings.SubjectGuide != 0 || settings.BodyGuide != 100 {
		test.Fatalf("guides = %d/%d, want 0/100", settings.SubjectGuide, settings.BodyGuide)
	}
}

func TestSetMessageGuides_RejectsOutOfRange(test *testing.T) {
	app, _ := appWithSettingsFile(test)

	for _, columns := range [][2]int{{-1, 72}, {50, 201}} {
		if err := app.SetMessageGuides(columns[0], columns[1]); err == nil {
			test.Fatalf("SetMessageGuides(%d, %d) accepted out-of-range columns", columns[0], columns[1])
		}
	}
	settings := app.GetSettings()
	if settings.SubjectGuide != 50 || settings.BodyGuide != 72 {
		test.Fatalf("guides = %d/%d, want the defaults", settings.SubjectGuide, settings.BodyGuide)
	}
}

func TestSetTerminalCommand_SavedForNextRun(test *testing.T) {
	app, path := appWithSettingsFile(test)

	if got := app.GetSettings().TerminalCommand; got != "" {
		test.Fatalf("TerminalCommand without a file = %q, want empty (automatic)", got)
	}
	if err := app.SetTerminalCommand("  wt.exe -d {dir} pwsh  "); err != nil {
		test.Fatalf("SetTerminalCommand: %v", err)
	}

	nextRun := &App{settingsPath: path}
	if got := nextRun.GetSettings().TerminalCommand; got != "wt.exe -d {dir} pwsh" {
		test.Fatalf("TerminalCommand = %q, want the trimmed command", got)
	}

	// An empty command goes back to the automatic choice.
	if err := nextRun.SetTerminalCommand(""); err != nil {
		test.Fatalf("SetTerminalCommand(\"\"): %v", err)
	}
	if got := nextRun.GetSettings().TerminalCommand; got != "" {
		test.Fatalf("TerminalCommand = %q, want empty", got)
	}
}

func TestSetTerminalCommand_RejectsUnclosedQuote(test *testing.T) {
	app, _ := appWithSettingsFile(test)

	if err := app.SetTerminalCommand(`"C:\Program Files\PowerShell\7\pwsh.exe`); err == nil {
		test.Fatal("SetTerminalCommand accepted an unclosed quote")
	}
	if got := app.GetSettings().TerminalCommand; got != "" {
		test.Fatalf("TerminalCommand = %q, want it unchanged", got)
	}
}

func TestGetSettings_OfficeHoursDefaults(test *testing.T) {
	app, path := appWithSettingsFile(test)
	want := OfficeHours{Start: "09:00", End: "17:00", Days: []int{1, 2, 3, 4, 5}}

	if got := app.GetSettings().OfficeHours; !reflect.DeepEqual(got, want) {
		test.Fatalf("office hours without a file = %+v, want %+v", got, want)
	}

	// Invalid hours, edited by hand, fall back to the defaults.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"officeHours": {"start": "18:00", "end": "08:00", "days": [1]}}`), 0o644); err != nil {
		test.Fatal(err)
	}
	if got := app.GetSettings().OfficeHours; !reflect.DeepEqual(got, want) {
		test.Fatalf("invalid office hours = %+v, want the defaults %+v", got, want)
	}
}

func TestSetOfficeHours_SavedForNextRun(test *testing.T) {
	app, path := appWithSettingsFile(test)
	want := OfficeHours{Start: "07:30", End: "22:00", Days: []int{0, 6}}

	if err := app.SetOfficeHours(want); err != nil {
		test.Fatalf("SetOfficeHours: %v", err)
	}

	nextRun := &App{settingsPath: path}
	if got := nextRun.GetSettings().OfficeHours; !reflect.DeepEqual(got, want) {
		test.Fatalf("office hours = %+v, want %+v", got, want)
	}
}

func TestSetOfficeHours_RejectsInvalidHours(test *testing.T) {
	app, _ := appWithSettingsFile(test)

	invalid := []OfficeHours{
		{Start: "17:00", End: "09:00", Days: []int{1}},
		{Start: "9", End: "17:00", Days: []int{1}},
		{Start: "09:00", End: "17:00", Days: []int{}},
		{Start: "09:00", End: "17:00", Days: []int{7}},
	}
	for _, hours := range invalid {
		if err := app.SetOfficeHours(hours); err == nil {
			test.Errorf("SetOfficeHours(%+v) accepted invalid hours", hours)
		}
	}
	if got := app.GetSettings().OfficeHours; got.Start != "09:00" || got.End != "17:00" {
		test.Fatalf("office hours = %+v, want the defaults", got)
	}
}

func TestSetStaleFetchDays_SavedForNextRun(test *testing.T) {
	app, path := appWithSettingsFile(test)

	if got := app.GetSettings().StaleFetchDays; got != 7 {
		test.Fatalf("StaleFetchDays without a file = %d, want 7", got)
	}
	// 0 turns the warning off, and must survive a reload rather than become the default.
	if err := app.SetStaleFetchDays(0); err != nil {
		test.Fatalf("SetStaleFetchDays: %v", err)
	}

	nextRun := &App{settingsPath: path}
	if got := nextRun.GetSettings().StaleFetchDays; got != 0 {
		test.Fatalf("StaleFetchDays = %d, want 0", got)
	}
}

func TestSetStaleFetchDays_RejectsOutOfRange(test *testing.T) {
	app, path := appWithSettingsFile(test)

	for _, days := range []int{-1, 366} {
		if err := app.SetStaleFetchDays(days); err == nil {
			test.Fatalf("SetStaleFetchDays(%d) accepted an out-of-range value", days)
		}
	}
	if got := app.GetSettings().StaleFetchDays; got != 7 {
		test.Fatalf("StaleFetchDays = %d, want the default", got)
	}

	// An out-of-range value, edited by hand, falls back to the default.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"staleFetchDays": 1000}`), 0o644); err != nil {
		test.Fatal(err)
	}
	if got := app.GetSettings().StaleFetchDays; got != 7 {
		test.Fatalf("out-of-range StaleFetchDays = %d, want 7", got)
	}
}

func TestSetBackupSettings_SavedForNextRun(test *testing.T) {
	app, path := appWithSettingsFile(test)

	settings := app.GetSettings()
	if !settings.BackupBeforeApply || settings.AutoBackupsKept != 20 {
		test.Fatalf("defaults = %v, %d, want true, 20", settings.BackupBeforeApply, settings.AutoBackupsKept)
	}
	if err := app.SetBackupSettings(false, 5); err != nil {
		test.Fatalf("SetBackupSettings: %v", err)
	}

	nextRun := &App{settingsPath: path}
	settings = nextRun.GetSettings()
	if settings.BackupBeforeApply || settings.AutoBackupsKept != 5 {
		test.Fatalf("saved = %v, %d, want false, 5", settings.BackupBeforeApply, settings.AutoBackupsKept)
	}
}

func TestSetBackupSettings_RejectsOutOfRange(test *testing.T) {
	app, path := appWithSettingsFile(test)

	for _, count := range []int{0, -1, 1001} {
		if err := app.SetBackupSettings(true, count); err == nil {
			test.Fatalf("SetBackupSettings(%d) accepted an out-of-range value", count)
		}
	}

	// A file from before backups existed gets the defaults, and an
	// out-of-range count, edited by hand, falls back to the default.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"autoBackupsKept": 0}`), 0o644); err != nil {
		test.Fatal(err)
	}
	settings := app.GetSettings()
	if !settings.BackupBeforeApply || settings.AutoBackupsKept != 20 {
		test.Fatalf("settings = %v, %d, want true, 20", settings.BackupBeforeApply, settings.AutoBackupsKept)
	}
}
