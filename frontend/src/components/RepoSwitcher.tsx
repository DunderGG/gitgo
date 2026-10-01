import { useState } from 'react'
import { SelectDirectory } from '../../wailsjs/go/app/App'
import { useRepoStore } from '../store/repoStore'
import Spinner from './Spinner'

// Value of the last option, which opens the folder picker instead of a path.
const browseValue = '\0browse'

// RepoSwitcher is the header dropdown showing the open repository. Picking a
// recent repository opens it in place of the current one, without going back
// to the start screen.
export default function RepoSwitcher() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const recentRepos = useRepoStore((s) => s.recentRepos)
  const activity = useRepoStore((s) => s.activity)
  const openRepository = useRepoStore((s) => s.openRepository)

  // Repository being opened, so the dropdown shows it while it loads.
  const [openingPath, setOpeningPath] = useState<string | null>(null)

  if (!repoInfo) {
    return null
  }

  async function open(path: string) {
    setOpeningPath(path)
    try {
      await openRepository(path)
    } finally {
      setOpeningPath(null)
    }
  }

  async function handleChange(value: string) {
    if (value === browseValue) {
      const path = await SelectDirectory()
      if (path && path !== repoInfo?.path) {
        await open(path)
      }
      return
    }
    if (value !== repoInfo?.path) {
      await open(value)
    }
  }

  // setRepo puts the open repository first in the recent list, but keep it
  // selectable even if it was removed from there.
  const options = recentRepos.includes(repoInfo.path) ? [...recentRepos] : [repoInfo.path, ...recentRepos]
  if (openingPath && !options.includes(openingPath)) {
    options.push(openingPath)
  }

  return (
    <div className="ml-4 flex min-w-0 items-center gap-2">
      {openingPath && <Spinner className="h-3.5 w-3.5 shrink-0 text-indigo-300" />}
      <select
        value={openingPath ?? repoInfo.path}
        onChange={(e) => handleChange(e.target.value)}
        disabled={activity !== null}
        title={repoInfo.path}
        aria-label="Repository"
        className="min-w-0 max-w-xl truncate rounded-md border border-transparent bg-transparent px-1 py-1 text-sm text-gray-300 outline-none transition hover:border-gray-700 hover:bg-gray-900 focus:border-indigo-500 focus:bg-gray-900 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {options.map((path) => (
          <option key={path} value={path} className="bg-gray-900 text-gray-100">
            {path}
          </option>
        ))}
        <option value={browseValue} className="bg-gray-900 text-gray-100">
          Open another folder…
        </option>
      </select>
    </div>
  )
}
