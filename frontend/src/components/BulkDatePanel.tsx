import { useState } from 'react'
import { GetAffectedRefs, GetCommitLog, RefreshLog, ShiftCommitDates } from '../../wailsjs/go/app/App'
import type { app } from '../../wailsjs/go/models'
import ConfirmDialog from './ConfirmDialog'
import DateShiftButtons, { DATE_BUTTON_CLASS } from './DateShiftButtons'
import { formatShift, shiftWallClock, splitRfc3339, toPreviewDateText } from '../dates'
import { errorText } from '../errors'
import { useRepoStore, type CommitSummary } from '../store/repoStore'

interface ShiftPreview {
  commit: CommitSummary
  before: string
  after: string
}

// Author dates of the selected commits before and after the shift, each in
// its own time zone.
function shiftPreviews(selected: CommitSummary[], minutes: number): ShiftPreview[] {
  return selected.map((commit) => {
    const date = splitRfc3339(commit.date)
    const shifted = shiftWallClock(date.dateLocal, minutes)
    return {
      commit,
      before: toPreviewDateText(date),
      after: shifted ? toPreviewDateText({ dateLocal: shifted, offset: date.offset }) : '',
    }
  })
}

// Reports whether the shift makes a commit older than the one listed below
// it, where it was not before. The list is newest first, so this is usually
// a commit dated before its parent, which Git allows but `git log` shows
// out of order.
function breaksDateOrder(commits: CommitSummary[], selected: Set<string>, minutes: number): boolean {
  const instant = (commit: CommitSummary, shift: number) =>
    Date.parse(commit.date) + (selected.has(commit.hash) ? shift * 60_000 : 0)
  return commits.slice(0, -1).some((newer, index) => {
    const older = commits[index + 1]
    if (!selected.has(newer.hash) && !selected.has(older.hash)) {
      return false
    }
    return instant(newer, 0) >= instant(older, 0) && instant(newer, minutes) < instant(older, minutes)
  })
}

// Shown instead of EditPanel while several commits are selected: moves their
// author dates (and optionally committer dates) by the same amount in one
// rewrite, which a single undo reverts.
export default function BulkDatePanel() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const commits = useRepoStore((s) => s.commits)
  const selectedHashes = useRepoStore((s) => s.selectedHashes)
  const selectCommit = useRepoStore((s) => s.selectCommit)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const setCanUndo = useRepoStore((s) => s.setCanUndo)
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)

  // The shift adds up across button clicks and is only applied on confirm.
  const [shiftMinutes, setShiftMinutes] = useState(0)
  const [shiftCommitter, setShiftCommitter] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)
  const [affectedRefs, setAffectedRefs] = useState<app.AffectedRef[]>([])
  const [moveBranches, setMoveBranches] = useState(true)

  const selectedSet = new Set(selectedHashes)
  const selected = commits.filter((commit) => selectedSet.has(commit.hash))
  // Commits can be pushed from a terminal after they were selected.
  const hasPushed = selected.some((commit) => !commit.isUnpushed)
  const previews = shiftPreviews(selected, shiftMinutes)
  const outOfOrder = breaksDateOrder(commits, selectedSet, shiftMinutes)
  const canReview = shiftMinutes !== 0 && !hasPushed && !isSubmitting && activity === null

  async function openConfirmDialog() {
    try {
      setAffectedRefs(await GetAffectedRefs(selectedHashes))
    } catch (error) {
      setError(errorText(error))
      return
    }
    setMoveBranches(true)
    setShowConfirmDialog(true)
  }

  async function handleConfirmApply() {
    if (!repoInfo) {
      return
    }
    setIsSubmitting(true)

    try {
      await runGitOperation('Shifting commit dates…', async () => {
        let result
        try {
          result = await ShiftCommitDates({
            hashes: selectedHashes,
            minutes: shiftMinutes,
            shiftCommitter,
            moveBranches: moveBranches
              ? affectedRefs.filter((ref) => ref.kind === 'branch').map((ref) => ref.name)
              : [],
          })
        } catch (error) {
          // Show the fresh pushed/unpushed state so the pushed commits are
          // flagged and Review Changes is disabled.
          if (/already been pushed/i.test(errorText(error))) {
            const refreshed = await GetCommitLog().catch(() => null)
            if (refreshed) {
              useRepoStore.setState({ commits: refreshed })
            }
            setShowConfirmDialog(false)
          }
          throw error
        }

        // The rewritten commits have new hashes, so setRepo also clears the
        // selection, which closes this panel.
        const refreshedCommits = await RefreshLog()
        setRepo(repoInfo, refreshedCommits)
        setCanUndo(true)

        if (result.success) {
          setError(null)
          setStatus(result.message)
        } else {
          setError(result.message)
        }
        setShowConfirmDialog(false)
      })
    } catch (error) {
      setError(String(error))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <aside className="h-full border-t lg:border-t-0 lg:border-l border-gray-800 bg-gray-900/60">
        <div className="h-full overflow-y-auto p-4 sm:p-5">
          <h2 className="text-base font-semibold text-gray-100">Shift Commit Dates</h2>
          <p className="mt-1 text-xs text-gray-400">
            Move the dates of the {selected.length} selected commits by the same amount. Each commit keeps its own
            time zone.
          </p>

          <div className="mt-5">
            <div className="flex items-baseline justify-between">
              <span className="text-xs font-medium uppercase tracking-wide text-gray-400">Shift</span>
              <span
                className={`font-mono text-sm ${shiftMinutes === 0 ? 'text-gray-500' : 'text-indigo-300'}`}
                aria-live="polite"
              >
                {formatShift(shiftMinutes)}
              </span>
            </div>
            <div className="mt-1.5">
              <DateShiftButtons
                disabled={isSubmitting}
                onShift={(minutes) => setShiftMinutes((current) => current + minutes)}
              >
                <button
                  type="button"
                  title="Back to no shift"
                  disabled={isSubmitting || shiftMinutes === 0}
                  onClick={() => setShiftMinutes(0)}
                  className={DATE_BUTTON_CLASS}
                >
                  Reset
                </button>
              </DateShiftButtons>
            </div>
            <label className="mt-2 flex items-center gap-2 text-sm text-gray-300">
              <input
                type="checkbox"
                checked={shiftCommitter}
                onChange={(e) => setShiftCommitter(e.target.checked)}
                disabled={isSubmitting}
                className="accent-indigo-500 disabled:cursor-not-allowed"
              />
              Also shift committer dates
            </label>
          </div>

          {hasPushed && (
            <div className="mt-4 rounded-lg border border-yellow-900/60 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-300">
              Some selected commits have been pushed and cannot be edited. Ctrl-click them to remove them from the
              selection.
            </div>
          )}
          {outOfOrder && !hasPushed && (
            <div className="mt-4 rounded-lg border border-yellow-900/60 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-300">
              After this shift, some commits will be dated earlier than the commit below them in the list. Git allows
              this, but tools that sort by date will show them out of order.
            </div>
          )}

          <ul className="mt-4 space-y-2">
            {previews.map(({ commit, before, after }) => (
              <li key={commit.hash} className="rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-xs">
                <div className="flex gap-2">
                  <span className="font-mono text-gray-500">{commit.shortHash}</span>
                  <span className="truncate text-gray-200" title={commit.message}>
                    {commit.message}
                  </span>
                </div>
                <div className="mt-1 text-gray-400">{before}</div>
                {shiftMinutes !== 0 && <div className="text-indigo-300">→ {after}</div>}
              </li>
            ))}
          </ul>

          <div className="flex items-center justify-end gap-3 pt-4">
            <button
              type="button"
              disabled={isSubmitting}
              onClick={() => selectCommit(null)}
              className="rounded-md border border-gray-700 px-4 py-2 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Clear Selection
            </button>
            <button
              type="button"
              disabled={!canReview}
              onClick={openConfirmDialog}
              className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Review Changes
            </button>
          </div>
        </div>
      </aside>

      <ConfirmDialog
        isOpen={showConfirmDialog}
        isSubmitting={isSubmitting}
        title={`Shift ${selected.length} Commits ${formatShift(shiftMinutes)}`}
        affectedRefs={affectedRefs}
        moveBranches={moveBranches}
        onMoveBranchesChange={setMoveBranches}
        onCancel={() => setShowConfirmDialog(false)}
        onConfirm={handleConfirmApply}
      >
        <div className="rounded-lg border border-gray-800 bg-gray-900/70 p-3">
          <div className="text-xs font-medium uppercase tracking-wide text-gray-400">Author Dates</div>
          <table className="mt-2 w-full text-left text-sm">
            <thead className="text-[11px] uppercase tracking-wide text-gray-500">
              <tr>
                <th className="pb-1 pr-3 font-normal">Commit</th>
                <th className="pb-1 pr-3 font-normal">Current</th>
                <th className="pb-1 font-normal">New</th>
              </tr>
            </thead>
            <tbody>
              {previews.map(({ commit, before, after }) => (
                <tr key={commit.hash} className="border-t border-gray-800 align-top">
                  <td className="py-1.5 pr-3">
                    <span className="font-mono text-xs text-gray-500">{commit.shortHash}</span>{' '}
                    <span className="text-gray-300">{commit.message}</span>
                  </td>
                  <td className="py-1.5 pr-3 text-gray-400">{before}</td>
                  <td className="py-1.5 text-indigo-200">{after}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="mt-2 text-xs text-gray-400">
            {shiftCommitter
              ? `Committer dates are shifted by ${formatShift(shiftMinutes)} as well.`
              : 'Committer dates are kept.'}{' '}
            Messages and authors are not changed.
          </p>
          {outOfOrder && (
            <p className="mt-2 text-xs text-yellow-300">
              Some commits will be dated earlier than the commit below them in the list.
            </p>
          )}
        </div>
      </ConfirmDialog>
    </>
  )
}
