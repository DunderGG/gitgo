package app

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	testDefaultSize = WindowSize{Width: 1300, Height: 880}
	testMinimumSize = WindowSize{Width: 900, Height: 600}
)

func TestWindowSize_SaveAndLoad(test *testing.T) {
	path := filepath.Join(test.TempDir(), "gitgo", "window.json")
	saved := WindowSize{Width: 1500, Height: 950, Maximised: true}

	saveWindowSize(path, saved)

	if got := loadWindowSize(path, testDefaultSize, testMinimumSize); got != saved {
		test.Fatalf("loadWindowSize = %+v, want %+v", got, saved)
	}
}

func TestLoadWindowSize_DefaultWithoutUsableFile(test *testing.T) {
	dir := test.TempDir()
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o644); err != nil {
		test.Fatal(err)
	}
	zero := filepath.Join(dir, "zero.json")
	if err := os.WriteFile(zero, []byte(`{"width":0,"height":0}`), 0o644); err != nil {
		test.Fatal(err)
	}

	for name, path := range map[string]string{
		"no config dir": "",
		"missing":       filepath.Join(dir, "missing.json"),
		"corrupt":       corrupt,
		"zero size":     zero,
	} {
		if got := loadWindowSize(path, testDefaultSize, testMinimumSize); got != testDefaultSize {
			test.Errorf("%s: loadWindowSize = %+v, want the default %+v", name, got, testDefaultSize)
		}
	}
}

func TestLoadWindowSize_NotBelowMinimum(test *testing.T) {
	path := filepath.Join(test.TempDir(), "window.json")
	saveWindowSize(path, WindowSize{Width: 400, Height: 300})

	if got := loadWindowSize(path, testDefaultSize, testMinimumSize); got != testMinimumSize {
		test.Fatalf("loadWindowSize = %+v, want the minimum %+v", got, testMinimumSize)
	}
}

func TestFitWindowSize(test *testing.T) {
	cases := []struct {
		name                string
		size                WindowSize
		maxWidth, maxHeight int
		want                WindowSize
	}{
		{"fits", WindowSize{Width: 1300, Height: 880}, 2560, 1392, WindowSize{Width: 1300, Height: 880}},
		// 1920x1080 at 125% scaling.
		{"small screen", WindowSize{Width: 1300, Height: 880}, 1536, 816, WindowSize{Width: 1300, Height: 816}},
		{"never below minimum", WindowSize{Width: 1300, Height: 880}, 800, 500, testMinimumSize},
	}
	for _, testCase := range cases {
		got := fitWindowSize(testCase.size, testCase.maxWidth, testCase.maxHeight, testMinimumSize)
		if got != testCase.want {
			test.Errorf("%s: fitWindowSize = %+v, want %+v", testCase.name, got, testCase.want)
		}
	}
}
