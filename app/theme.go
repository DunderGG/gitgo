package app

// The colour themes a user can pick. ThemeSystem follows the operating
// system's light or dark setting.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

func validTheme(theme string) bool {
	return theme == ThemeSystem || theme == ThemeLight || theme == ThemeDark
}

// PrefersDark reports whether the given theme shows dark colours, asking the
// operating system for ThemeSystem. main.go uses it to pick the window
// colour shown before the frontend has loaded.
func PrefersDark(theme string) bool {
	switch theme {
	case ThemeLight:
		return false
	case ThemeDark:
		return true
	default:
		return systemPrefersDark()
	}
}
