import { useRepoStore } from '../store/repoStore'

export default function StatusBar() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const status = useRepoStore((s) => s.status)
  const error = useRepoStore((s) => s.error)

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
      <div>
        {error ? (
          <span className="text-red-400">{error}</span>
        ) : (
          <span className="text-gray-400">{status}</span>
        )}
      </div>
    </footer>
  )
}
