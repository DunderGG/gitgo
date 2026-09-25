import { useCallback, useEffect, useState } from 'react'
import { GetCommitLog, ListBranches, SwitchBranch } from '../../wailsjs/go/app/App'
import { useRepoStore } from '../store/repoStore'
import Spinner from './Spinner'

export default function BranchSelector() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)

  const [branches, setBranches] = useState<string[]>([])
  // Branch being switched to, so the spinner only shows for this control.
  const [switchingTo, setSwitchingTo] = useState<string | null>(null)

  const repoPath = repoInfo?.path

  const loadBranches = useCallback(async () => {
    try {
      setBranches(await ListBranches())
    } catch (error) {
      setError(String(error))
    }
  }, [setError])

  // Reload the list whenever a different repository is opened.
  useEffect(() => {
    if (repoPath) {
      loadBranches()
    }
  }, [repoPath, loadBranches])

  async function handleChange(branch: string) {
    if (!repoInfo || branch === repoInfo.branch) {
      return
    }

    setSwitchingTo(branch)

    try {
      await runGitOperation(`Switching to ${branch}…`, async () => {
        const info = await SwitchBranch(branch)
        const commits = await GetCommitLog()
        setRepo(info, commits)
        setStatus(
          info.isCheckedOut ? `Viewing branch ${info.branch}` : `Viewing branch ${info.branch} (not checked out)`,
        )
      })
    } catch (error) {
      setError(String(error))
      // The branch may have been deleted outside the app; refresh the list.
      loadBranches()
    } finally {
      setSwitchingTo(null)
    }
  }

  if (!repoInfo) {
    return null
  }

  // Keep the current branch selectable even before the list has loaded.
  const options = branches.includes(repoInfo.branch) ? branches : [repoInfo.branch, ...branches]

  return (
    <label className="flex items-center gap-2 text-sm text-gray-400 shrink-0">
      {switchingTo ? <Spinner className="h-3.5 w-3.5 text-indigo-300" /> : <span>Branch</span>}
      <select
        // Show the branch being switched to while the switch is in progress.
        value={switchingTo ?? repoInfo.branch}
        onChange={(e) => handleChange(e.target.value)}
        // Pick up branches created outside the app since the last load.
        onFocus={loadBranches}
        disabled={activity !== null}
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
