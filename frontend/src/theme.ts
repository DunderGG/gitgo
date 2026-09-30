import {
  WindowSetBackgroundColour,
  WindowSetDarkTheme,
  WindowSetLightTheme,
  WindowSetSystemDefaultTheme,
} from '../wailsjs/runtime/runtime'

// ThemePreference is the theme the user picked; 'system' follows the
// operating system's light or dark setting. The Go side keeps it in the
// settings file (Settings.Theme).
export type ThemePreference = 'system' | 'light' | 'dark'

export const themePreferences: ThemePreference[] = ['system', 'light', 'dark']

export function isThemePreference(value: string): value is ThemePreference {
  return (themePreferences as string[]).includes(value)
}

const systemDarkQuery = window.matchMedia('(prefers-color-scheme: dark)')
let currentPreference: ThemePreference = 'system'

function resolvedTheme(preference: ThemePreference): 'light' | 'dark' {
  if (preference === 'system') {
    return systemDarkQuery.matches ? 'dark' : 'light'
  }
  return preference
}

// applyTheme switches the page's colours (the CSS variables in
// tailwind.config.ts), the window background and, on Windows, the title bar.
export function applyTheme(preference: ThemePreference) {
  currentPreference = preference
  const theme = resolvedTheme(preference)
  document.documentElement.dataset.theme = theme
  try {
    if (preference === 'system') {
      WindowSetSystemDefaultTheme()
    } else if (preference === 'light') {
      WindowSetLightTheme()
    } else {
      WindowSetDarkTheme()
    }
    // gray-900 of each theme, as in main.go.
    if (theme === 'light') {
      WindowSetBackgroundColour(249, 250, 251, 255)
    } else {
      WindowSetBackgroundColour(17, 24, 39, 255)
    }
  } catch {
    // The Wails runtime is missing when the frontend runs in a plain browser
    // (npm run dev); the page colours are enough there.
  }
}

// Follow the operating system when it switches between light and dark.
systemDarkQuery.addEventListener('change', () => {
  if (currentPreference === 'system') {
    applyTheme('system')
  }
})
