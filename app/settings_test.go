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
