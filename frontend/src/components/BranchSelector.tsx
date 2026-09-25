import { useEffect, useState } from 'react'
import { GetCommitLog, ListBranches, SwitchBranch } from '../../wailsjs/go/app/App'
import { useRepoStore } from '../store/repoStore'

export default function BranchSelector() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)

  const [branches, setBranches] = useState<string[]>([])
  const [isSwitching, setIsSwitching] = useState(false)

  const repoPath = repoInfo?.path

  async function loadBranches() {
    try {
      setBranches(await ListBranches())
    } catch (error) {
      setError(String(error))
    }
  }

  // Reload the list whenever a different repository is opened.
  useEffect(() => {
    if (repoPath) {
      loadBranches()
    }
  }, [repoPath])

  async function handleChange(branch: string) {
    if (!repoInfo || branch === repoInfo.branch) {
      return
    }

    setIsSwitching(true)

    try {
      const info = await SwitchBranch(branch)
      const commits = await GetCommitLog()
      setRepo(info, commits)
      setStatus(
        info.isCheckedOut ? `Viewing branch ${info.branch}` : `Viewing branch ${info.branch} (not checked out)`,
      )
    } catch (error) {
      setError(String(error))
      // The branch may have been deleted outside the app; refresh the list.
      loadBranches()
    } finally {
      setIsSwitching(false)
    }
  }

  if (!repoInfo) {
    return null
  }

  // Keep the current branch selectable even before the list has loaded.
  const options = branches.includes(repoInfo.branch) ? branches : [repoInfo.branch, ...branches]

  return (
    <label className="flex items-center gap-2 text-sm text-gray-400 shrink-0">
      <span>Branch</span>
      <select
        value={repoInfo.branch}
        onChange={(e) => handleChange(e.target.value)}
        // Pick up branches created outside the app since the last load.
        onFocus={loadBranches}
        disabled={isSwitching}
        className="max-w-56 rounded-md border border-gray-700 bg-gray-900 px-2 py-1 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {options.map((branch) => (
          <option key={branch} value={branch}>
            {branch}
          </option>
        ))}
      </select>
    </label>
  )
}
