package app

import (
	"errors"
	"reflect"
	"testing"
)

// fakeLookPath finds only the named programs.
func fakeLookPath(available ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, candidate := range available {
			if candidate == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func noEnv(string) string { return "" }

func launchNames(launches []terminalLaunch) []string {
	names := make([]string, len(launches))
	for i, launch := range launches {
		names[i] = launch.name
	}
	return names
}

func TestTerminalLaunches_WindowsPrefersWindowsTerminal(test *testing.T) {
	got := launchNames(terminalLaunches("windows", fakeLookPath("cmd.exe", "wt.exe"), noEnv))
	want := []string{"wt.exe", "cmd.exe"}
	if !reflect.DeepEqual(got, want) {
		test.Fatalf("launches = %v, want %v", got, want)
	}
}

func TestTerminalLaunches_WindowsFallsBackToCmd(test *testing.T) {
	launches := terminalLaunches("windows", fakeLookPath("cmd.exe"), noEnv)
	if len(launches) != 1 || launches[0].name != "cmd.exe" || !launches[0].newConsole {
		test.Fatalf("launches = %+v, want only cmd.exe in a new console", launches)
	}
}

func TestTerminalLaunches_LinuxTriesTerminalEnvFirst(test *testing.T) {
	getenv := func(key string) string {
		if key == "TERMINAL" {
			return "foot"
		}
		return ""
	}
	got := launchNames(terminalLaunches("linux", fakeLookPath("xterm", "foot", "konsole"), getenv))
	want := []string{"foot", "konsole", "xterm"}
	if !reflect.DeepEqual(got, want) {
		test.Fatalf("launches = %v, want %v", got, want)
	}
}

func TestTerminalLaunches_NoneFound(test *testing.T) {
	if launches := terminalLaunches("linux", fakeLookPath(), noEnv); len(launches) != 0 {
		test.Fatalf("launches = %v, want none", launchNames(launches))
	}
}

func TestTerminalLaunch_CommandFillsInDirectory(test *testing.T) {
	dir := "/home/user/my repo"
	cmd := terminalLaunch{name: "gnome-terminal", args: []string{"--working-directory={dir}"}}.command(dir)
	if want := []string{"gnome-terminal", "--working-directory=" + dir}; !reflect.DeepEqual(cmd.Args, want) {
		test.Fatalf("args = %q, want %q", cmd.Args, want)
	}
	if cmd.Dir != dir {
		test.Fatalf("dir = %q, want %q", cmd.Dir, dir)
	}
}

func TestOpenTerminal_RequiresOpenRepository(test *testing.T) {
	if err := New().OpenTerminal(); err == nil {
		test.Fatal("OpenTerminal without a repository: want an error")
	}
}
