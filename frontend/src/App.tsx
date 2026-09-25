import { useEffect } from 'react'
import { WindowSetTitle } from '../wailsjs/runtime/runtime'
import BranchSelector from './components/BranchSelector'
import RepoSelector from './components/RepoSelector'
import StatusBar from './components/StatusBar'
import CommitList from './components/CommitList'
import EditPanel from './components/EditPanel'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { useRepoStore } from './store/repoStore'

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
  useKeyboardShortcuts()

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
          <>
            <span className="ml-4 text-sm text-gray-400 truncate">
              {repoInfo.path}
            </span>
            <div className="ml-auto pl-4">
              <BranchSelector />
            </div>
          </>
        )}
      </header>

      <main className="flex-1 overflow-hidden">
        {!repoInfo ? (
          <RepoSelector />
        ) : (
          <div className="grid h-full grid-cols-1 lg:grid-cols-[minmax(0,1fr)_380px] overflow-hidden">
            <CommitList />
            <EditPanel />
          </div>
        )}
      </main>

      <StatusBar />
    </div>
  )
}

export default App
