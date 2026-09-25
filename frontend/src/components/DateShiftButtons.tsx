import type { ReactNode } from 'react'
import { DATE_SHIFTS } from '../dates'

export const DATE_BUTTON_CLASS =
  'rounded border border-gray-700 px-1.5 py-0.5 font-mono text-[11px] text-gray-400 transition hover:border-gray-600 hover:bg-gray-800 hover:text-gray-200 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-transparent'

interface DateShiftButtonsProps {
  disabled: boolean
  onShift: (minutes: number) => void
  // Extra buttons shown after the shifts (use DATE_BUTTON_CLASS to match).
  children?: ReactNode
}

// The row of tiny −1d / −1h / +1h / +1d buttons used by both edit panels.
export default function DateShiftButtons({ disabled, onShift, children }: DateShiftButtonsProps) {
  return (
    <div className="flex flex-wrap gap-1">
      {DATE_SHIFTS.map(({ label, minutes, title }) => (
        <button
          key={label}
          type="button"
          title={title}
          disabled={disabled}
          onClick={() => onShift(minutes)}
          className={DATE_BUTTON_CLASS}
        >
          {label}
        </button>
      ))}
      {children}
    </div>
  )
}
