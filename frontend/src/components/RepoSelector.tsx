import { OpenRepository, GetCommitLog, SelectDirectory } from '../../wailsjs/go/app/App'
import { useRepoStore } from '../store/repoStore'

function looksLikeMissingPathError(errorText: string): boolean {
  return /does not exist|cannot find|no such file|cannot resolve path/i.test(errorText)
}

export default function RepoSelector() {
  const { recentRepos, setRepo, setError, setStatus, removeRecentRepo } = useRepoStore()

  async function openRepo(path: string) {
    setStatus('Opening repository…')
    setError(null)

    try {
      const repoInfo = await OpenRepository(path)
      const commits = await GetCommitLog()
      setRepo(repoInfo, commits)
    } catch (error) {
      const errorText = String(error)
      setError(errorText)

      // Remove stale recent entries when the folder no longer exists.
      if (looksLikeMissingPathError(errorText)) {
        removeRecentRepo(path)
      }
    }
  }

  async function handleOpen() {
    const path = await SelectDirectory()
    if (!path) {
      return
    }

    await openRepo(path)
  }

  return (
    <div className="flex flex-col items-center justify-center h-full gap-6">
      <div className="text-center">
        <h2 className="text-2xl font-semibold text-gray-200 mb-2">
          Open a Repository
        </h2>
        <p className="text-gray-400 text-sm">
          Select a local Git repository folder to get started.
        </p>
      </div>
      <button
        onClick={handleOpen}
        className="px-6 py-3 bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 text-white font-medium rounded-lg transition-colors"
      >
        Open Repository
      </button>

      {recentRepos.length > 0 && (
        <div className="w-full max-w-3xl rounded-lg border border-gray-800 bg-gray-900/70 p-4">
          <h3 className="text-xs font-medium uppercase tracking-wide text-gray-400">
            Recent Repositories
          </h3>
          <div className="mt-3 space-y-2">
            {recentRepos.map((path) => (
              <div key={path} className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => openRepo(path)}
                  className="flex-1 rounded-md border border-gray-800 bg-gray-800/40 px-3 py-2 text-left text-sm text-gray-300 transition hover:border-indigo-700 hover:bg-gray-800"
                  title={path}
                >
                  <span className="block truncate">{path}</span>
                </button>
                <button
                  type="button"
                  onClick={() => removeRecentRepo(path)}
                  className="rounded-md border border-gray-700 px-2 py-1 text-xs text-gray-400 transition hover:border-gray-500 hover:text-gray-200"
                  title="Remove from recent list"
                >
                  Remove
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
