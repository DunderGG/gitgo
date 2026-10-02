import { useEffect, useState } from 'react'
import { Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../../wailsjs/runtime/runtime'

// Shaped like the Windows 11 caption buttons: wide, full height and square.
const buttonClass =
  'flex w-[46px] items-center justify-center text-gray-300 transition-colors hover:bg-gray-700 hover:text-gray-100'

// WindowControls are the minimise, maximise/restore and close buttons of the
// frameless window, at the right end of the title bar.
export default function WindowControls() {
  const [maximised, setMaximised] = useState(false)

  useEffect(() => {
    // Maximising can also come from a double-click, Win+Up or snapping, so
    // follow the window rather than the button.
    const update = () => {
      WindowIsMaximised()
        .then(setMaximised)
        .catch(() => {})
    }
    update()
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])

  return (
    <div className="flex shrink-0 self-stretch">
      <button type="button" onClick={WindowMinimise} title="Minimise" aria-label="Minimise" className={buttonClass}>
        <svg viewBox="0 0 10 10" className="h-2.5 w-2.5" stroke="currentColor" aria-hidden="true">
          <path d="M0 5.5h10" />
        </svg>
      </button>
      <button
        type="button"
        onClick={WindowToggleMaximise}
        title={maximised ? 'Restore' : 'Maximise'}
        aria-label={maximised ? 'Restore' : 'Maximise'}
        className={buttonClass}
      >
        {maximised ? (
          <svg viewBox="0 0 10 10" className="h-2.5 w-2.5" fill="none" stroke="currentColor" aria-hidden="true">
            <path d="M0.5 2.5h7v7h-7z M2.5 2.5v-2h7v7h-2" />
          </svg>
        ) : (
          <svg viewBox="0 0 10 10" className="h-2.5 w-2.5" fill="none" stroke="currentColor" aria-hidden="true">
            <path d="M0.5 0.5h9v9h-9z" />
          </svg>
        )}
      </button>
      <button
        type="button"
        onClick={Quit}
        title="Close"
        aria-label="Close"
        className={`${buttonClass} hover:bg-[#c42b1c] hover:text-white`}
      >
        <svg viewBox="0 0 10 10" className="h-2.5 w-2.5" stroke="currentColor" aria-hidden="true">
          <path d="M0 0l10 10M10 0L0 10" />
        </svg>
      </button>
    </div>
  )
}
