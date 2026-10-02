import { useEffect } from 'react'
import { useRepoStore } from '../store/repoStore'

// How often to look for outside changes while the window is visible. The
// backend only reads refs and a few small files, so this is cheap.
const pollIntervalMs = 3000

// useExternalChangeWatcher checks for changes made outside GitGo when the
// window gets focus (the usual case: back from a terminal) and every few
// seconds while it is visible (a terminal next to the window).
function useExternalChangeWatcher() {
  const checkForExternalChanges = useRepoStore((s) => s.checkForExternalChanges)

  useEffect(() => {
    const check = () => {
      if (!document.hidden) {
        void checkForExternalChanges()
      }
    }
    const interval = window.setInterval(check, pollIntervalMs)
    window.addEventListener('focus', check)
    document.addEventListener('visibilitychange', check)
    return () => {
      window.clearInterval(interval)
      window.removeEventListener('focus', check)
      document.removeEventListener('visibilitychange', check)
    }
  }, [checkForExternalChanges])
}

// ExternalChangeBar sits under the header while a repository is open and,
// when the repository changed outside GitGo, offers to reload it. Reloading
// keeps the selected commit and unsaved form edits, so it is safe to click.
export default function ExternalChangeBar() {
  const hasExternalChanges = useRepoStore((s) => s.hasExternalChanges)
  const activity = useRepoStore((s) => s.activity)
  const reloadRepository = useRepoStore((s) => s.reloadRepository)
  useExternalChangeWatcher()

  if (!hasExternalChanges) {
    return null
  }

  return (
    <div
      className="flex items-center gap-3 border-b border-sky-900/60 bg-sky-950/40 px-4 py-1.5 text-sm text-sky-200 shrink-0"
      role="status"
    >
      <span className="min-w-0 truncate">The repository changed outside GitGo.</span>
      <button
        type="button"
        onClick={reloadRepository}
        disabled={activity !== null}
        title="Reload the repository from disk (F5)"
        className="shrink-0 rounded-md border border-sky-800 px-2 py-0.5 text-xs text-sky-100 transition hover:bg-sky-900/60 disabled:cursor-not-allowed disabled:opacity-60"
      >
        Reload
      </button>
    </div>
  )
}
