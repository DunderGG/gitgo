import { OpenRepository, GetCommitLog, SelectDirectory } from '../../wailsjs/go/app/App'
import { useRepoStore } from '../store/repoStore'

export default function RepoSelector() {
  const { setRepo, setError, setStatus } = useRepoStore()

  async function handleOpen() {
    try {
      const path = await SelectDirectory()
      if (!path) return

      setStatus('Opening repository…')
      const repoInfo = await OpenRepository(path)
      const commits = await GetCommitLog()
      setRepo(repoInfo, commits)
    } catch (err) {
      setError(String(err))
    }
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
    </div>
  )
}
