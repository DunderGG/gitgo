package app

import (
	"os/exec"
	"syscall"
)

// createNewConsole is CREATE_NEW_CONSOLE from the Windows API.
const createNewConsole = 0x00000010

// useNewConsole makes cmd open in a console window of its own. GitGo is a GUI
// program without a console, so a console program such as cmd.exe would
// otherwise have no window.
func useNewConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
}
