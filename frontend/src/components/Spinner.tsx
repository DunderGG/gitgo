interface SpinnerProps {
  className?: string
}

// Spinner is a small inline loading indicator that inherits the text colour.
export default function Spinner({ className = 'h-3.5 w-3.5' }: SpinnerProps) {
  return (
    <svg className={`animate-spin shrink-0 ${className}`} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4H4z" />
    </svg>
  )
}
