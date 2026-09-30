package app

import "golang.org/x/sys/windows/registry"

// systemPrefersDark reads the "Choose your app mode" setting. Without it
// (older Windows) apps are light.
func systemPrefersDark() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	lightTheme, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return err == nil && lightTheme == 0
}
