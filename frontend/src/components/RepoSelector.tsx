import { OpenRepository, GetCommitLog, SelectDirectory } from '../../wailsjs/go/app/App'
import { useState } from 'react'
import { useRepoStore } from '../store/repoStore'
import Spinner from './Spinner'

function looksLikeMissingPathError(errorText: string): boolean {
  return /does not exist|cannot find|no such file|cannot resolve path/i.test(errorText)
}

export default function RepoSelector() {
  const { recentRepos, activity, runGitOperation, setRepo, setError, removeRecentRepo } = useRepoStore()

  // Path currently being opened, so its button can show a spinner.
  const [openingPath, setOpeningPath] = useState<string | null>(null)
  const isBusy = activity !== null

  async function openRepo(path: string) {
    setError(null)
    setOpeningPath(path)

    try {
      await runGitOperation('Opening repository…', async () => {
        const repoInfo = await OpenRepository(path)
        const commits = await GetCommitLog()
        setRepo(repoInfo, commits)
      })
    } catch (error) {
      const errorText = String(error)
      setError(errorText)

      // Remove stale recent entries when the folder no longer exists.
      if (looksLikeMissingPathError(errorText)) {
        removeRecentRepo(path)
      }
    } finally {
      setOpeningPath(null)
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
    <div className="flex flex-col items-center justify-center h-full gap-6 px-4">
      <div className="text-center max-w-md">
        <h2 className="text-2xl font-semibold text-gray-200 mb-2">
          Open a Repository
        </h2>
        <p className="text-gray-400 text-sm">
          Select a local Git repository folder to get started.
        </p>
        <p className="mt-2 text-gray-500 text-xs">
          GitGo lets you edit the message, date, and author of commits you have not pushed yet.
          Pushed commits are shown read-only.
        </p>
      </div>
      <button
        onClick={handleOpen}
        disabled={isBusy}
        className="flex items-center gap-2 px-6 py-3 bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 text-white font-medium rounded-lg transition-colors disabled:cursor-not-allowed disabled:opacity-60"
      >
        {/* Recent entries show their own spinner; this one covers picker opens. */}
        {openingPath && !recentRepos.includes(openingPath) ? (
          <>
            <Spinner className="h-4 w-4" />
            Opening…
          </>
        ) : (
          'Open Repository'
        )}
      </button>

      {recentRepos.length === 0 && (
        <p className="text-xs text-gray-600">Repositories you open will be listed here for quick access.</p>
      )}

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
                  disabled={isBusy}
                  className="flex flex-1 min-w-0 items-center gap-2 rounded-md border border-gray-800 bg-gray-800/40 px-3 py-2 text-left text-sm text-gray-300 transition hover:border-indigo-700 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
                  title={path}
                >
                  <span className="block truncate">{path}</span>
                  {openingPath === path && <Spinner className="ml-auto h-3.5 w-3.5 text-indigo-300" />}
                </button>
                <button
                  type="button"
                  onClick={() => removeRecentRepo(path)}
                  disabled={isBusy}
                  className="rounded-md border border-gray-700 px-2 py-1 text-xs text-gray-400 transition hover:border-gray-500 hover:text-gray-200 disabled:cursor-not-allowed disabled:opacity-60"
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
