import { useEffect } from 'react'
import { OpenTerminal } from '../wailsjs/go/app/App'
import { WindowSetTitle } from '../wailsjs/runtime/runtime'
import BranchSelector from './components/BranchSelector'
import RepoSelector from './components/RepoSelector'
import StatusBar from './components/StatusBar'
import CommitList from './components/CommitList'
import EditPanel from './components/EditPanel'
import BulkDatePanel from './components/BulkDatePanel'
import HelpDialog from './components/HelpDialog'
import Spinner from './components/Spinner'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { reloadActivityLabel, useRepoStore } from './store/repoStore'

// windowTitle names the open repository and branch, e.g. "GitGo — myrepo (main)",
// so the window is easy to find in the taskbar / window switcher.
function windowTitle(path: string | undefined, branch: string | undefined): string {
  if (!path || !branch) {
    return 'GitGo'
  }
  const repoName = path.split(/[\\/]/).filter(Boolean).pop() ?? path
  return `GitGo — ${repoName} (${branch})`
}

function App() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const activity = useRepoStore((s) => s.activity)
  const isMultiSelect = useRepoStore((s) => s.selectedHashes.length > 1)
  const reloadRepository = useRepoStore((s) => s.reloadRepository)
  const setHelpOpen = useRepoStore((s) => s.setHelpOpen)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  useKeyboardShortcuts()

  async function openTerminal() {
    try {
      await OpenTerminal()
      setError(null)
      setStatus('Opened a terminal in the repository folder')
    } catch (error) {
      setError(String(error))
    }
  }

  const title = windowTitle(repoInfo?.path, repoInfo?.branch)
  useEffect(() => {
    document.title = title
    try {
      WindowSetTitle(title)
    } catch {
      // The Wails runtime is missing when the frontend runs in a plain browser
      // (npm run dev); document.title is enough there.
    }
  }, [title])

  return (
    <div className="flex flex-col h-screen bg-gray-900 text-gray-100">
      <header className="flex items-center px-4 py-3 bg-gray-800 border-b border-gray-700 shrink-0">
        <h1 className="text-lg font-semibold text-white tracking-tight">GitGo</h1>
        {repoInfo && (
          <span className="ml-4 text-sm text-gray-400 truncate">
            {repoInfo.path}
          </span>
        )}
        <div className="ml-auto flex items-center gap-3 pl-4">
          {repoInfo && (
            <>
              <BranchSelector />
              <button
                type="button"
                onClick={reloadRepository}
                disabled={activity !== null}
                title="Reload the repository from disk (F5)"
                aria-label="Reload repository"
                className="rounded-md border border-gray-700 px-2 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {activity === reloadActivityLabel ? <Spinner className="h-4 w-4" /> : '↻'}
              </button>
              <button
                type="button"
                onClick={openTerminal}
                title="Open a terminal in the repository folder"
                aria-label="Open terminal"
                className="rounded-md border border-gray-700 px-2 py-1 font-mono text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
              >
                &gt;_
              </button>
            </>
          )}
          <button
            type="button"
            onClick={() => setHelpOpen(true)}
            title="How to use GitGo (F1)"
            aria-label="Help"
            className="rounded-md border border-gray-700 px-2.5 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
          >
            ?
          </button>
        </div>
      </header>

      <main className="flex-1 overflow-hidden">
        {!repoInfo ? (
          <RepoSelector />
        ) : (
          <div className="grid h-full grid-cols-1 lg:grid-cols-[minmax(0,1fr)_380px] overflow-hidden">
            <CommitList />
            {isMultiSelect ? <BulkDatePanel /> : <EditPanel />}
          </div>
        )}
      </main>

      <StatusBar />
      <HelpDialog />
    </div>
  )
}

export default App
