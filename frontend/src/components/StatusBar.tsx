import { useRepoStore } from '../store/repoStore'
import Spinner from './Spinner'

export default function StatusBar() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const status = useRepoStore((s) => s.status)
  const error = useRepoStore((s) => s.error)
  const canUndo = useRepoStore((s) => s.canUndo)
  const activity = useRepoStore((s) => s.activity)
  const undoLastOperation = useRepoStore((s) => s.undoLastOperation)

  return (
    <footer className="flex items-center justify-between px-4 py-1.5 bg-gray-800 border-t border-gray-700 text-xs shrink-0">
      <div className="flex items-center gap-3">
        {repoInfo && (
          <>
            <span className="text-indigo-400 font-medium">{repoInfo.branch}</span>
            {!repoInfo.isCheckedOut && (
              <span
                className="text-sky-400"
                title="Edits move this branch only; your working tree is not touched"
              >
                Not checked out
              </span>
            )}
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
        {/* A running operation takes precedence over the last status or error. */}
        {activity ? (
          <span className="flex items-center gap-2 text-indigo-300" role="status">
            <Spinner />
            {activity}
          </span>
        ) : error ? (
          <span className="text-red-400">{error}</span>
        ) : (
          <span className="text-gray-400">{status}</span>
        )}
        {repoInfo && canUndo && (
          <button
            type="button"
            onClick={undoLastOperation}
            disabled={activity !== null}
            title="Restore the branch to how it was before the last edit (Ctrl+Z)"
            className="rounded border border-gray-600 px-2 py-0.5 text-gray-200 transition hover:border-gray-500 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Undo
          </button>
        )}
      </div>
    </footer>
  )
}
