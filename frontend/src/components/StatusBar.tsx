import { useState } from 'react'
import { CanUndo, RefreshLog, UndoLastOperation } from '../../wailsjs/go/app/App'
import { useRepoStore } from '../store/repoStore'

export default function StatusBar() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const status = useRepoStore((s) => s.status)
  const error = useRepoStore((s) => s.error)
  const canUndo = useRepoStore((s) => s.canUndo)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const setCanUndo = useRepoStore((s) => s.setCanUndo)

  const [isUndoing, setIsUndoing] = useState(false)

  async function handleUndo() {
    if (!repoInfo || isUndoing) {
      return
    }

    setIsUndoing(true)

    try {
      const result = await UndoLastOperation()
      const refreshedCommits = await RefreshLog()
      // setRepo clears canUndo, which is correct: only one level is kept.
      setRepo(repoInfo, refreshedCommits)
      setError(null)
      setStatus(result.message)
    } catch (err) {
      setError(String(err))
      // The backend drops the undo record when it can never succeed (e.g. the
      // branch moved), so ask it whether the button should stay.
      setCanUndo(await CanUndo().catch(() => false))
    } finally {
      setIsUndoing(false)
    }
  }

  return (
    <footer className="flex items-center justify-between px-4 py-1.5 bg-gray-800 border-t border-gray-700 text-xs shrink-0">
      <div className="flex items-center gap-3">
        {repoInfo && (
          <>
            <span className="text-indigo-400 font-medium">{repoInfo.branch}</span>
            {!repoInfo.hasRemote && (
              <span className="text-yellow-400">No remote configured</span>
            )}
            {repoInfo.hasRemote && !repoInfo.hasUpstream && (
              <span className="text-yellow-400">No upstream set</span>
            )}
          </>
        )}
      </div>
      <div className="flex items-center gap-3">
        {error ? (
          <span className="text-red-400">{error}</span>
        ) : (
          <span className="text-gray-400">{status}</span>
        )}
        {repoInfo && canUndo && (
          <button
            type="button"
            onClick={handleUndo}
            disabled={isUndoing}
            title="Restore the branch to how it was before the last edit"
            className="rounded border border-gray-600 px-2 py-0.5 text-gray-200 transition hover:border-gray-500 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isUndoing ? 'Undoing…' : 'Undo'}
          </button>
        )}
      </div>
    </footer>
  )
}
