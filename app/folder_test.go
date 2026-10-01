package app

import (
	"reflect"
	"runtime"
	"testing"
)

func TestFileManagerCommand(test *testing.T) {
	cases := []struct {
		goos string
		dir  string
		want []string
	}{
		{"windows", "C:/repos/my repo", []string{"explorer.exe", `C:\repos\my repo`}},
		{"darwin", "/Users/me/my repo", []string{"open", "/Users/me/my repo"}},
		{"linux", "/home/me/my repo", []string{"xdg-open", "/home/me/my repo"}},
	}
	for _, testCase := range cases {
		if testCase.goos == "windows" && runtime.GOOS != "windows" {
			// filepath.FromSlash only converts slashes on Windows.
			continue
		}
		got := fileManagerCommand(testCase.goos, testCase.dir).Args
		if !reflect.DeepEqual(got, testCase.want) {
			test.Fatalf("%s: args = %q, want %q", testCase.goos, got, testCase.want)
		}
	}
}

func TestOpenFolder_RequiresOpenRepository(test *testing.T) {
	if err := New().OpenFolder(); err == nil {
		test.Fatal("OpenFolder without a repository: want an error")
	}
}
