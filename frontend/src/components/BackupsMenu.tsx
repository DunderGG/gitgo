import { useCallback, useEffect, useRef, useState } from 'react'
import { CreateBackup } from '../../wailsjs/go/app/App'
import { errorText, friendlyError } from '../errors'
import { MAX_BACKUP_NAME_LENGTH, useRepoStore } from '../store/repoStore'
import BackupsDialog from './BackupsDialog'

const backupActivityLabel = 'Backing up…'

interface BackUpPopoverProps {
  branch: string
  // The Back up button, which toggles the popover itself.
  anchorRef: React.RefObject<HTMLButtonElement | null>
  onClose: () => void
}

// BackUpPopover asks for an optional name and makes the backup. The field has
// focus at once, so Enter alone makes an unnamed backup; Escape or a click
// outside cancels.
function BackUpPopover({ branch, anchorRef, onClose }: BackUpPopoverProps) {
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const rootRef = useRef<HTMLFormElement>(null)
  const [name, setName] = useState('')
  // Why the backup was refused (for example a name that is too long). Shown
  // here so the typed name is not lost.
  const [problem, setProblem] = useState<string | null>(null)

  useEffect(() => {
    // Escape is handled in the capture phase so the app-wide shortcut does
    // not also clear the commit selection.
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      }
    }
    function handlePointerDown(event: PointerEvent) {
      const target = event.target as Node
      if (rootRef.current?.contains(target) || anchorRef.current?.contains(target)) {
        return
      }
      onClose()
    }
    window.addEventListener('keydown', handleKeyDown, true)
    document.addEventListener('pointerdown', handlePointerDown)
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      document.removeEventListener('pointerdown', handlePointerDown)
    }
  }, [anchorRef, onClose])

  async function backUp() {
    setProblem(null)
    try {
      const backup = await runGitOperation(backupActivityLabel, () => CreateBackup(name))
      if (backup) {
        const saved = backup.branches[0]
        setError(null)
        setStatus(`Backed up ${saved.name} at ${saved.shortHash}${backup.name ? ` as “${backup.name}”` : ''}`)
        onClose()
      }
    } catch (error) {
      setProblem(friendlyError(errorText(error)))
    }
  }

  return (
    <form
      ref={rootRef}
      onSubmit={(event) => {
        event.preventDefault()
        backUp()
      }}
      aria-label={`Back up ${branch}`}
      className="absolute right-0 top-full z-40 mt-2 w-80 space-y-2 rounded-lg border border-gray-700 bg-gray-900 p-3 shadow-2xl"
    >
      <label htmlFor="backup-name" className="block text-sm text-gray-200">
        Back up <span className="font-mono">{branch}</span>
      </label>
      <input
        id="backup-name"
        type="text"
        value={name}
        onChange={(event) => setName(event.target.value)}
        maxLength={MAX_BACKUP_NAME_LENGTH}
        placeholder="Name (optional)"
        autoFocus
        className="w-full rounded-md border border-gray-700 bg-gray-800 px-2 py-1.5 text-sm text-gray-100 outline-none focus:border-indigo-500"
      />
      <p className="text-xs text-gray-500">
        For example “before date spread”. Restore or delete backups with the clock button.
      </p>
      {problem && <p className="text-xs text-red-300">{problem}</p>}
      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-md border border-gray-700 px-3 py-1 text-sm text-gray-300 transition hover:bg-gray-800"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={activity !== null}
          className="rounded-md bg-indigo-600 px-3 py-1 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Back up
        </button>
      </div>
    </form>
  )
}

// BackupsMenu is the header's pair of backup buttons: Back up asks for an
// optional name in a popover and saves the viewed branch's tip, and the clock
// button opens BackupsDialog to restore, export, name or delete backups.
export default function BackupsMenu() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const activity = useRepoStore((s) => s.activity)
  const [isListOpen, setIsListOpen] = useState(false)
  const [isPopoverOpen, setIsPopoverOpen] = useState(false)
  const closeList = useCallback(() => setIsListOpen(false), [])
  const closePopover = useCallback(() => setIsPopoverOpen(false), [])
  const backUpRef = useRef<HTMLButtonElement>(null)

  const branch = repoInfo?.branch ?? ''
  return (
    <div className="relative">
      <div className="flex rounded-md border border-gray-700 text-sm text-gray-300">
        <button
          ref={backUpRef}
          type="button"
          onClick={() => setIsPopoverOpen((isOpen) => !isOpen)}
          disabled={activity !== null}
          title={`Save the current state of ${branch} as a backup you can restore later`}
          aria-expanded={isPopoverOpen}
          className="rounded-l-md px-2 py-1 transition hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Back up
        </button>
        <button
          type="button"
          onClick={() => {
            setIsPopoverOpen(false)
            setIsListOpen(true)
          }}
          title="Show backups to restore, export, name or delete"
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
      {isPopoverOpen && <BackUpPopover branch={branch} anchorRef={backUpRef} onClose={closePopover} />}
      {isListOpen && <BackupsDialog onClose={closeList} />}
    </div>
  )
}
