import { useCallback, useState } from 'react'
import { CreateBackup } from '../../wailsjs/go/app/App'
import { errorText } from '../errors'
import { useRepoStore } from '../store/repoStore'
import BackupsDialog from './BackupsDialog'

const backupActivityLabel = 'Backing up…'

// BackupsMenu is the header's pair of backup buttons: Back up saves the viewed
// branch's tip at once, and the list button opens BackupsDialog to restore or
// delete backups.
export default function BackupsMenu() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const [isOpen, setIsOpen] = useState(false)
  const close = useCallback(() => setIsOpen(false), [])

  async function backUp() {
    try {
      setError(null)
      const backup = await runGitOperation(backupActivityLabel, () => CreateBackup())
      if (backup) {
        const saved = backup.branches[0]
        setStatus(`Backed up ${saved.name} at ${saved.shortHash}`)
      }
    } catch (error) {
      setError(errorText(error))
    }
  }

  const branch = repoInfo?.branch ?? ''
  return (
    <>
      <div className="flex rounded-md border border-gray-700 text-sm text-gray-300">
        <button
          type="button"
          onClick={backUp}
          disabled={activity !== null}
          title={`Save the current state of ${branch} as a backup you can restore later`}
          className="rounded-l-md px-2 py-1 transition hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Back up
        </button>
        <button
          type="button"
          onClick={() => setIsOpen(true)}
          title="Show backups to restore or delete"
          aria-label="Backups"
          className="rounded-r-md border-l border-gray-700 px-2 py-1 transition hover:bg-gray-700"
        >
          {/* A clock with a back arrow; the emoji alternatives are in colour. */}
          <svg viewBox="0 0 16 16" className="h-5 w-4" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
            <path d="M2.5 8a5.5 5.5 0 1 0 1.6-3.9" strokeLinecap="round" />
            <path d="M2 2.5v2.5h2.5" strokeLinecap="round" strokeLinejoin="round" />
            <path d="M8 5v3.2l2 1.3" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>
      {isOpen && <BackupsDialog onClose={close} />}
    </>
  )
}
