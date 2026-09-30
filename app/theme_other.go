//go:build !windows

package app

// systemPrefersDark has no cheap answer before the window exists on macOS and
// Linux, so the window starts dark, as GitGo always did. The frontend follows
// the system theme once it has loaded.
func systemPrefersDark() bool {
	return true
}
