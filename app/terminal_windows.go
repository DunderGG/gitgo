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

// createNoWindow is CREATE_NO_WINDOW from the Windows API.
const createNoWindow = 0x08000000

// hideConsole keeps a console program such as git.exe from flashing a console
// window while GitGo runs it in the background.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
