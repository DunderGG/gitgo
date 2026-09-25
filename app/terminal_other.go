//go:build !windows

package app

import "os/exec"

// useNewConsole is only needed on Windows; terminal emulators elsewhere open
// their own window.
func useNewConsole(*exec.Cmd) {}
