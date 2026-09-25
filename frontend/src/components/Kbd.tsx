import type { ReactNode } from 'react'

// A keyboard key, e.g. <Kbd>Enter</Kbd>.
export default function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="rounded border border-gray-700 bg-gray-800 px-1 font-mono text-[11px] text-gray-300">
      {children}
    </kbd>
  )
}
