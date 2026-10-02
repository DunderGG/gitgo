import { useEffect, useState } from 'react'
import { Environment, WindowToggleMaximise } from '../wailsjs/runtime/runtime'

// useFramelessWindow reports whether the app draws its own title bar. main.go
// makes the window frameless on Windows only; macOS and Linux keep the
// system title bar, and a plain browser (npm run dev) has no window to drive.
export function useFramelessWindow(): boolean {
  const [frameless, setFrameless] = useState(false)
  useEffect(() => {
    try {
      Environment()
        .then((environment) => setFrameless(environment.platform === 'windows'))
        .catch(() => {})
    } catch {
      // No Wails runtime.
    }
  }, [])
  return frameless
}

// toggleMaximiseOnDoubleClick gives the title bar the system behaviour of
// maximising or restoring on a double-click, but only on its empty parts,
// not on the buttons and menus in it.
export function toggleMaximiseOnDoubleClick(event: React.MouseEvent) {
  const target = event.target as Element
  if (getComputedStyle(target).getPropertyValue('--wails-draggable').trim() === 'drag') {
    WindowToggleMaximise()
  }
}
