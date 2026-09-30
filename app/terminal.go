package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// terminalLaunch is one way to open a terminal window in a directory.
type terminalLaunch struct {
	name string
	args []string
	// Start the program in a console window of its own (Windows console
	// programs such as cmd.exe; ignored on other platforms).
	newConsole bool
}

// linuxTerminals are tried in order after $TERMINAL. Terminals that do not
// take a directory flag start in the working directory they inherit.
var linuxTerminals = []terminalLaunch{
	{name: "x-terminal-emulator"},
	{name: "gnome-terminal", args: []string{"--working-directory={dir}"}},
	{name: "konsole", args: []string{"--workdir", "{dir}"}},
	{name: "xfce4-terminal", args: []string{"--working-directory={dir}"}},
	{name: "kitty"},
	{name: "alacritty"},
	{name: "xterm"},
}

// terminalLaunches returns the terminals to try for goos, best first, keeping
// only those lookPath finds. {dir} in args stands for the directory.
func terminalLaunches(goos string, lookPath func(string) (string, error), getenv func(string) string) []terminalLaunch {
	var candidates []terminalLaunch
	switch goos {
	case "windows":
		candidates = []terminalLaunch{
			// Windows Terminal opens its default profile (usually PowerShell).
			{name: "wt.exe", args: []string{"-d", "{dir}"}},
			{name: "cmd.exe", newConsole: true},
		}
	case "darwin":
		candidates = []terminalLaunch{{name: "open", args: []string{"-a", "Terminal", "{dir}"}}}
	default:
		if terminal := strings.TrimSpace(getenv("TERMINAL")); terminal != "" {
			candidates = append(candidates, terminalLaunch{name: terminal})
		}
		candidates = append(candidates, linuxTerminals...)
	}

	var found []terminalLaunch
	for _, candidate := range candidates {
		if _, err := lookPath(candidate.name); err == nil {
			found = append(found, candidate)
		}
	}
	return found
}

// splitCommandLine splits a terminal command from the settings into the
// program and its arguments. Arguments are separated by spaces; double or
// single quotes keep spaces inside one argument. Backslashes are ordinary
// characters, so Windows paths need no escaping.
func splitCommandLine(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	inArg := false
	var quote rune
	for _, char := range line {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
		case char == '"' || char == '\'':
			quote = char
			inArg = true
		case char == ' ' || char == '\t':
			if inArg {
				args = append(args, current.String())
				current.Reset()
				inArg = false
			}
		default:
			current.WriteRune(char)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("the terminal command has an unclosed %c quote", quote)
	}
	if inArg {
		args = append(args, current.String())
	}
	if len(args) > 0 && args[0] == "" {
		return nil, errors.New("the terminal command has no program name")
	}
	return args, nil
}

// customTerminalLaunch is the terminal command from the settings. On Windows
// it gets a console of its own, which console programs such as pwsh.exe need
// and programs with a window of their own ignore.
func customTerminalLaunch(line string) (terminalLaunch, error) {
	args, err := splitCommandLine(line)
	if err != nil {
		return terminalLaunch{}, err
	}
	if len(args) == 0 {
		return terminalLaunch{}, errors.New("the terminal command is empty")
	}
	return terminalLaunch{name: args[0], args: args[1:], newConsole: true}, nil
}

// command builds the exec.Cmd that opens this terminal in dir.
func (launch terminalLaunch) command(dir string) *exec.Cmd {
	args := make([]string, len(launch.args))
	for i, arg := range launch.args {
		args[i] = strings.ReplaceAll(arg, "{dir}", dir)
	}
	cmd := exec.Command(launch.name, args...)
	cmd.Dir = dir
	if launch.newConsole {
		useNewConsole(cmd)
	}
	return cmd
}

// OpenTerminal opens a terminal window in the root of the open repository, so
// the user can run git commands by hand. The terminal is not tied to GitGo:
// it stays open when GitGo exits.
// OpenRepository must be called before this method.
func (app *App) OpenTerminal() error {
	app.mutex.Lock()
	state := app.repoState
	app.mutex.Unlock()

	if state == nil {
		return fmt.Errorf("no repository is open; call OpenRepository first")
	}

	// A terminal command in the settings replaces the automatic choice, with
	// no fallback, so a mistake in it shows instead of another terminal.
	if command := app.GetSettings().TerminalCommand; command != "" {
		launch, err := customTerminalLaunch(command)
		if err != nil {
			return err
		}
		cmd := launch.command(state.Path)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("could not run the terminal command from Settings: %w", err)
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}

	launches := terminalLaunches(runtime.GOOS, exec.LookPath, os.Getenv)
	if len(launches) == 0 {
		return errors.New("no terminal program found; set a terminal command in Settings")
	}

	var startErrors []error
	for _, launch := range launches {
		cmd := launch.command(state.Path)
		if err := cmd.Start(); err != nil {
			startErrors = append(startErrors, fmt.Errorf("%s: %w", launch.name, err))
			continue
		}
		// Reap the process when it exits; nothing waits for the result.
		go func() { _ = cmd.Wait() }()
		return nil
	}
	return fmt.Errorf("could not open a terminal: %w", errors.Join(startErrors...))
}
