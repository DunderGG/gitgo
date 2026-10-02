import { useCallback, useEffect, useRef, useState } from 'react'
import {
  CanUndo,
  DeleteBackup,
  ExportBackup,
  GetGitStatus,
  ListBackups,
  PlanRestore,
  RefreshLog,
  RenameBackup,
  RestoreBackup,
} from '../../wailsjs/go/app/App'
import type { app } from '../../wailsjs/go/models'
import { errorText, friendlyError } from '../errors'
import { formatAge } from '../lastFetch'
import { MAX_BACKUP_NAME_LENGTH, useRepoStore } from '../store/repoStore'
import Spinner from './Spinner'

// When a backup was made, e.g. "25 Sep 2026, 14:03 (3 hours ago)".
function formatCreated(created: string): string {
  const date = new Date(created)
  const text = date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  return `${text} (${formatAge(Date.now() - date.getTime())})`
}

function plural(count: number, word: string): string {
  return `${count} ${word}${count === 1 ? '' : 's'}`
}

// A backup can be restored when at least one of its branches still exists
// and points somewhere else.
function hasChanges(backup: app.BackupInfo): boolean {
  return backup.branches.some((branch) => branch.exists && !branch.current)
}

function KindBadge({ kind }: { kind: string }) {
  const isManual = kind === 'manual'
  return (
    <span
      title={isManual ? 'Made with Back up; kept until deleted' : 'Made by GitGo before an edit or restore'}
      className={`rounded px-1.5 py-0.5 text-[11px] font-medium uppercase tracking-wide ${
        isManual ? 'bg-indigo-950/60 text-indigo-200' : 'bg-gray-800 text-gray-400'
      }`}
    >
      {isManual ? 'Manual' : 'Automatic'}
    </span>
  )
}

interface CommitListProps {
  label: string
  commits: app.RestoreCommitInfo[]
  count: number
}

// The commits a restore removes from or brings back to a branch.
function RestoreCommits({ label, commits, count }: CommitListProps) {
  if (count === 0) {
    return null
  }
  return (
    <div className="mt-2">
      <div className="text-xs text-gray-400">{label}</div>
      <ul className="mt-1 space-y-0.5 text-sm">
        {commits.map((commit) => (
          <li key={commit.hash} className="flex gap-2">
            <span className="font-mono text-xs leading-5 text-gray-500">{commit.shortHash}</span>
            <span className="truncate text-gray-300">{commit.subject}</span>
          </li>
        ))}
        {count > commits.length && <li className="text-xs text-gray-500">and {count - commits.length} more</li>}
      </ul>
    </div>
  )
}

function RestoreBranch({ branch }: { branch: app.RestoreBranchInfo }) {
  return (
    <div className="rounded-lg border border-gray-800 bg-gray-900/70 p-3">
      <div className="flex items-center gap-2">
        <span className="font-mono text-sm text-gray-100">{branch.name}</span>
        {branch.checkedOut && <span className="text-xs text-gray-500">checked out</span>}
      </div>
      {branch.missing ? (
        <p className="mt-1 text-sm text-gray-400">Deleted since the backup. It is not recreated.</p>
      ) : branch.unchanged ? (
        <p className="mt-1 text-sm text-gray-400">Already matches the backup.</p>
      ) : (
        <>
          <RestoreCommits
            label={`${plural(branch.removedCount, 'commit')} on the branch now will be removed from it:`}
            commits={branch.removed}
            count={branch.removedCount}
          />
          <RestoreCommits
            label={`${plural(branch.returnedCount, 'commit')} from the backup will come back:`}
            commits={branch.returned}
            count={branch.returnedCount}
          />
        </>
      )}
      {branch.problem && <p className="mt-2 text-sm text-red-300">{friendlyError(branch.problem)}</p>}
    </div>
  )
}

interface BackupsDialogProps {
  onClose: () => void
}

// BackupsDialog lists the open repository's backups, those of the viewed
// branch by default, with Restore and Delete. Restore first shows what it
// would do (PlanRestore) and only moves branches once confirmed.
export default function BackupsDialog({ onClose }: BackupsDialogProps) {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setCanUndo = useRepoStore((s) => s.setCanUndo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const closeRef = useRef<HTMLButtonElement>(null)
  const [backups, setBackups] = useState<app.BackupInfo[] | null>(null)
  const [showAll, setShowAll] = useState(false)
  // The restore being reviewed; the dialog shows it instead of the list.
  const [plan, setPlan] = useState<app.RestorePlanInfo | null>(null)
  const [confirmingDelete, setConfirmingDelete] = useState<string | null>(null)
  // The backup whose export options are shown, and whether the bundle leaves
  // out the commits already on a remote.
  const [choosingExport, setChoosingExport] = useState<string | null>(null)
  const [partialExport, setPartialExport] = useState(false)
  // The backup being renamed and the name typed for it.
  const [renaming, setRenaming] = useState<string | null>(null)
  const [renameText, setRenameText] = useState('')
  const [problem, setProblem] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setBackups(await ListBackups())
    } catch (error) {
      setProblem(friendlyError(errorText(error)))
      setBackups([])
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  // Export runs the native git program (go-git cannot write bundles), so it
  // is disabled with the Run menu's explanation when git is missing.
  const [gitStatus, setGitStatus] = useState<app.GitStatus | null>(null)
  useEffect(() => {
    GetGitStatus()
      .then(setGitStatus)
      .catch((error) => setGitStatus({ available: false, version: '', problem: String(error) }))
  }, [])
  const exportTitle = !gitStatus
    ? 'Looking for git…'
    : gitStatus.available
      ? 'Save this backup, with its whole history, to a git bundle file outside the repository'
      : gitStatus.problem

  // Like the other dialogs, it owns Escape while open (going back from a
  // restore review to the list first) and blocks Ctrl+Z and Ctrl+A so the
  // app-wide shortcuts cannot act behind it.
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeRef.current?.focus()
    return () => previousFocus?.focus()
  }, [])

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const key = event.key.toLowerCase()
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        if (renaming) {
          setRenaming(null)
        } else if (plan) {
          setPlan(null)
        } else {
          onClose()
        }
      } else if (
        (event.ctrlKey || event.metaKey) &&
        (key === 'z' || key === 'a') &&
        !(event.target instanceof HTMLInputElement && event.target.type === 'text')
      ) {
        // The name fields keep the browser's own text undo and select all.
        event.preventDefault()
        event.stopPropagation()
      }
    }
    window.addEventListener('keydown', handleKeyDown, true)
    return () => window.removeEventListener('keydown', handleKeyDown, true)
  }, [renaming, plan, onClose])

  async function review(id: string) {
    setProblem(null)
    setConfirmingDelete(null)
    try {
      const planned = await runGitOperation('Checking backup…', () => PlanRestore(id))
      if (planned) {
        setPlan(planned)
      }
    } catch (error) {
      setProblem(friendlyError(errorText(error)))
      await load()
    }
  }

  async function restore() {
    if (!plan || !repoInfo) {
      return
    }
    const tips: Record<string, string> = {}
    for (const branch of plan.branches) {
      tips[branch.name] = branch.currentTip
    }
    try {
      const result = await runGitOperation('Restoring backup…', async () => {
        const restored = await RestoreBackup(plan.backup.id, tips)
        setRepo(repoInfo, await RefreshLog())
        setCanUndo(await CanUndo().catch(() => false))
        return restored
      })
      if (!result) {
        return
      }
      if (result.success) {
        setStatus(friendlyError(result.message))
      } else {
        setError(result.message)
      }
      onClose()
    } catch (error) {
      // Refused (for example the branch moved or was pushed meanwhile): show
      // why and the up-to-date plan.
      setProblem(friendlyError(errorText(error)))
      const replanned = await PlanRestore(plan.backup.id).catch(() => null)
      setPlan(replanned)
      if (!replanned) {
        await load()
      }
    }
  }

  // Only one row shows extra controls at a time: export options, delete
  // confirmation or the rename field.
  function closeRowControls() {
    setChoosingExport(null)
    setConfirmingDelete(null)
    setRenaming(null)
  }

  function chooseExport(id: string) {
    setProblem(null)
    closeRowControls()
    setChoosingExport(id)
  }

  function chooseDelete(id: string) {
    closeRowControls()
    setConfirmingDelete(id)
  }

  function startRename(backup: app.BackupInfo) {
    setProblem(null)
    closeRowControls()
    setRenameText(backup.name)
    setRenaming(backup.id)
  }

  async function saveName(id: string) {
    setProblem(null)
    try {
      await RenameBackup(id, renameText)
      setRenaming(null)
      setStatus(renameText.trim() ? `Backup named “${renameText.trim()}”` : 'Backup name removed')
    } catch (error) {
      setProblem(friendlyError(errorText(error)))
    }
    await load()
  }

  async function exportBackup(id: string, partial: boolean) {
    setProblem(null)
    try {
      const path = await runGitOperation('Exporting backup…', () => ExportBackup(id, partial))
      // An empty path means the save dialog was cancelled; the options stay
      // open to try again.
      if (path) {
        setChoosingExport(null)
        setStatus(`Exported the backup${partial ? ' (only commits not on a remote)' : ''} to ${path}`)
      }
    } catch (error) {
      setProblem(friendlyError(errorText(error)))
    }
  }

  async function remove(id: string) {
    setProblem(null)
    setConfirmingDelete(null)
    try {
      await DeleteBackup(id)
      setStatus('Backup deleted')
    } catch (error) {
      setProblem(friendlyError(errorText(error)))
    }
    await load()
  }

  const branch = repoInfo?.branch ?? ''
  const shown = (backups ?? []).filter(
    (backup) => showAll || backup.branches.some((saved) => saved.name === branch),
  )
  const hiddenCount = (backups?.length ?? 0) - shown.length
  const isBusy = activity !== null
  // Without a remote, a partial bundle would be the same as a full one.
  const canExportPartial = repoInfo?.hasRemote === true
  const canRestore = plan !== null && !plan.problem && plan.branches.some((entry) => !entry.missing && !entry.unchanged)


  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
      onClick={(event) => event.target === event.currentTarget && onClose()}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="backups-title"
        className="flex max-h-[85vh] w-full max-w-3xl flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-800 px-5 py-4">
          <div className="min-w-0">
            <h2 id="backups-title" className="text-lg font-semibold text-gray-100">
              {plan ? 'Restore this backup?' : 'Backups'}
            </h2>
            <p className="mt-1 text-sm text-gray-400">
              {plan
                ? `${plan.backup.name ? `“${plan.backup.name}”, made` : 'Made'} ${formatCreated(plan.backup.created)}`
                : 'Saved states of your branches. Restoring one moves the branches back to it.'}
            </p>
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            title="Close (Escape)"
            aria-label="Close backups"
            className="rounded-md px-2 py-1 text-gray-400 transition hover:bg-gray-800 hover:text-gray-200"
          >
            ×
          </button>
        </div>

        {problem && (
          <p role="alert" className="border-b border-gray-800 bg-red-950/30 px-5 py-2 text-sm text-red-300">
            {problem}
          </p>
        )}

        {plan ? (
          <>
            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-4">
              {plan.branches.map((entry) => (
                <RestoreBranch key={entry.name} branch={entry} />
              ))}
              <p className="text-xs text-gray-500">
                The branches’ current state is saved as an automatic backup first, and{' '}
                <span className="font-mono">Ctrl+Z</span> undoes the restore. Uncommitted changes and tags are not
                touched.
              </p>
            </div>
            <div className="flex justify-end gap-3 border-t border-gray-800 px-5 py-3">
              <button
                type="button"
                onClick={() => {
                  setPlan(null)
                  setProblem(null)
                  load()
                }}
                disabled={isBusy}
                className="rounded-md border border-gray-700 px-3 py-1.5 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                Back
              </button>
              <button
                type="button"
                onClick={restore}
                disabled={!canRestore || isBusy}
                className="flex items-center gap-2 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {activity === 'Restoring backup…' && <Spinner className="h-4 w-4" />}
                Restore
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
              {backups === null ? (
                <p className="flex items-center gap-2 text-sm text-gray-400">
                  <Spinner className="h-4 w-4" /> Loading backups…
                </p>
              ) : shown.length === 0 ? (
                <p className="text-sm text-gray-400">
                  {showAll || backups.length === 0
                    ? 'No backups yet. Click Back up in the header to save the current state of the branch.'
                    : `No backups of ${branch} yet.`}
                </p>
              ) : (
                <ul className="space-y-3">
                  {shown.map((backup) => (
                    <li key={backup.id} className="rounded-lg border border-gray-800 bg-gray-900/70 p-3">
                      <div className="flex flex-wrap items-center gap-2">
                        {renaming === backup.id ? (
                          <form
                            className="flex min-w-0 flex-1 items-center gap-2"
                            onSubmit={(event) => {
                              event.preventDefault()
                              saveName(backup.id)
                            }}
                          >
                            <input
                              type="text"
                              value={renameText}
                              onChange={(event) => setRenameText(event.target.value)}
                              maxLength={MAX_BACKUP_NAME_LENGTH}
                              placeholder="Name (empty for none)"
                              aria-label="Backup name"
                              autoFocus
                              className="min-w-0 flex-1 rounded-md border border-gray-700 bg-gray-800 px-2 py-1 text-sm text-gray-100 outline-none focus:border-indigo-500"
                            />
                            <button
                              type="submit"
                              className="rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white transition hover:bg-indigo-500"
                            >
                              Save
                            </button>
                            <button
                              type="button"
                              onClick={() => setRenaming(null)}
                              className="rounded-md border border-gray-700 px-2.5 py-1 text-xs text-gray-300 transition hover:bg-gray-800"
                            >
                              Cancel
                            </button>
                          </form>
                        ) : (
                          <>
                            {backup.name ? (
                              <>
                                <span className="min-w-0 truncate text-sm font-medium text-gray-100" title={backup.name}>
                                  {backup.name}
                                </span>
                                <span className="text-xs text-gray-400">{formatCreated(backup.created)}</span>
                              </>
                            ) : (
                              <span className="text-sm text-gray-200">{formatCreated(backup.created)}</span>
                            )}
                            <KindBadge kind={backup.kind} />
                          </>
                        )}
                        <div className="ml-auto flex items-center gap-2">
                          {renaming === backup.id ? null : choosingExport === backup.id ? (
                            <>
                              <label
                                className="flex items-center gap-1.5 text-xs text-gray-300"
                                title={
                                  canExportPartial
                                    ? 'A much smaller file, but it can only be fetched into a clone that already has the pushed history, so it does not protect against losing the repository'
                                    : 'This repository has no remote, so the bundle always holds the whole history'
                                }
                              >
                                <input
                                  type="checkbox"
                                  checked={partialExport && canExportPartial}
                                  onChange={(event) => setPartialExport(event.target.checked)}
                                  disabled={isBusy || !canExportPartial}
                                  className="accent-indigo-500 disabled:cursor-not-allowed"
                                />
                                Only commits not on a remote
                              </label>
                              <button
                                type="button"
                                onClick={() => exportBackup(backup.id, partialExport && canExportPartial)}
                                disabled={isBusy}
                                className="rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
                              >
                                Save…
                              </button>
                              <button
                                type="button"
                                onClick={() => setChoosingExport(null)}
                                disabled={isBusy}
                                className="rounded-md border border-gray-700 px-2.5 py-1 text-xs text-gray-300 transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
                              >
                                Cancel
                              </button>
                            </>
                          ) : confirmingDelete === backup.id ? (
                            <>
                              <span className="text-xs text-gray-400">Delete this backup?</span>
                              <button
                                type="button"
                                onClick={() => remove(backup.id)}
                                className="rounded-md bg-red-600 px-2.5 py-1 text-xs font-medium text-white transition hover:bg-red-500"
                              >
                                Delete
                              </button>
                              <button
                                type="button"
                                onClick={() => setConfirmingDelete(null)}
                                className="rounded-md border border-gray-700 px-2.5 py-1 text-xs text-gray-300 transition hover:bg-gray-800"
                              >
                                Keep
                              </button>
                            </>
                          ) : (
                            <>
                              <button
                                type="button"
                                onClick={() => review(backup.id)}
                                disabled={isBusy || !hasChanges(backup)}
                                title={
                                  hasChanges(backup)
                                    ? 'Review what restoring this backup would change'
                                    : 'The branches already match this backup'
                                }
                                className="rounded-md border border-gray-700 px-2.5 py-1 text-xs text-gray-200 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
                              >
                                Restore…
                              </button>
                              {/* The title is on a wrapper, since a disabled control may not show one. */}
                              <span title={exportTitle} className="flex">
                                <button
                                  type="button"
                                  onClick={() => chooseExport(backup.id)}
                                  disabled={isBusy || !gitStatus?.available}
                                  className="rounded-md border border-gray-700 px-2.5 py-1 text-xs text-gray-200 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
                                >
                                  Export…
                                </button>
                              </span>
                              <button
                                type="button"
                                onClick={() => startRename(backup)}
                                disabled={isBusy}
                                title={backup.name ? 'Change or remove the name' : 'Give this backup a name'}
                                className="rounded-md px-2.5 py-1 text-xs text-gray-400 transition hover:bg-gray-800 hover:text-gray-200 disabled:cursor-not-allowed disabled:opacity-60"
                              >
                                {backup.name ? 'Rename' : 'Name'}
                              </button>
                              <button
                                type="button"
                                onClick={() => chooseDelete(backup.id)}
                                disabled={isBusy}
                                className="rounded-md px-2.5 py-1 text-xs text-gray-400 transition hover:bg-gray-800 hover:text-red-300 disabled:cursor-not-allowed disabled:opacity-60"
                              >
                                Delete
                              </button>
                            </>
                          )}
                        </div>
                      </div>
                      <ul className="mt-2 space-y-1">
                        {backup.branches.map((saved) => (
                          <li key={saved.name} className="flex items-baseline gap-2 text-sm">
                            <span className="shrink-0 font-mono text-gray-100">{saved.name}</span>
                            <span className="shrink-0 font-mono text-xs text-gray-500">{saved.shortHash}</span>
                            <span className="min-w-0 truncate text-gray-400" title={saved.subject}>
                              {saved.subject}
                            </span>
                            {!saved.exists ? (
                              <span className="ml-auto shrink-0 text-xs text-gray-500">deleted</span>
                            ) : (
                              saved.current && (
                                <span className="ml-auto shrink-0 text-xs text-gray-500">same as now</span>
                              )
                            )}
                          </li>
                        ))}
                      </ul>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className="flex items-center gap-3 border-t border-gray-800 px-5 py-3 text-sm">
              <label className="flex items-center gap-2 text-gray-300">
                <input type="checkbox" checked={showAll} onChange={(event) => setShowAll(event.target.checked)} />
                Show all branches
              </label>
              {!showAll && hiddenCount > 0 && (
                <span className="text-xs text-gray-500">{plural(hiddenCount, 'backup')} of other branches hidden</span>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
