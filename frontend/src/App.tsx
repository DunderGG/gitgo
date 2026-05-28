import RepoSelector from './components/RepoSelector'
import StatusBar from './components/StatusBar'
import CommitList from './components/CommitList'
import { useRepoStore } from './store/repoStore'

function App() {
  const repoInfo = useRepoStore((s) => s.repoInfo)

  return (
    <div className="flex flex-col h-screen bg-gray-900 text-gray-100">
      <header className="flex items-center px-4 py-3 bg-gray-800 border-b border-gray-700 shrink-0">
        <h1 className="text-lg font-semibold text-white tracking-tight">GitGo</h1>
        {repoInfo && (
          <span className="ml-4 text-sm text-gray-400 truncate">
            {repoInfo.path}
          </span>
        )}
      </header>

      <main className="flex-1 overflow-hidden">
        {!repoInfo ? (
          <RepoSelector />
        ) : (
          <CommitList />
        )}
      </main>

      <StatusBar />
    </div>
  )
}

export default App
