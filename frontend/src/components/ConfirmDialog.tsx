import { useEffect, type ReactNode } from 'react'
import Spinner from './Spinner'
import type { app } from '../../wailsjs/go/models'

interface ConfirmDialogProps {
  isOpen: boolean
  isSubmitting: boolean
  title: string
  // The before/after comparison, e.g. CommitComparison.
  children: ReactNode
  // Other branches and tags the edit affects (see GetAffectedRefs).
  affectedRefs: app.AffectedRef[]
  moveBranches: boolean
  onMoveBranchesChange: (moveBranches: boolean) => void
  onCancel: () => void
  onConfirm: () => void
}

export interface ConfirmValues {
  message: string
  authorName: string
  authorEmail: string
  dateText: string
  committerDateText: string
}

interface CompareRowProps {
  label: string
  before: string
  after: string
  multiline?: boolean
}

function CompareRow({ label, before, after, multiline = false }: CompareRowProps) {
  const changed = before !== after
  const valueClassName = multiline
    ? 'min-h-24 whitespace-pre-wrap break-words'
    : 'break-words'

  return (
    <div className="space-y-2 rounded-lg border border-gray-800 bg-gray-900/70 p-3">
      <div className="text-xs font-medium uppercase tracking-wide text-gray-400">{label}</div>
      <div className="grid gap-3 md:grid-cols-2">
        <div>
          <div className="mb-1 text-[11px] uppercase tracking-wide text-gray-500">Current</div>
          <div className={`rounded-md border border-gray-800 bg-gray-950/60 px-3 py-2 text-sm text-gray-300 ${valueClassName}`}>
            {before || <span className="text-gray-600">Empty</span>}
          </div>
        </div>
        <div>
          <div className="mb-1 text-[11px] uppercase tracking-wide text-gray-500">New</div>
          <div
            className={`rounded-md border px-3 py-2 text-sm ${valueClassName} ${
              changed
                ? 'border-indigo-700 bg-indigo-950/30 text-indigo-100'
                : 'border-gray-800 bg-gray-950/60 text-gray-300'
            }`}
          >
            {after || <span className="text-gray-600">Empty</span>}
          </div>
        </div>
      </div>
    </div>
  )
}

// Current and new values of a single commit's editable fields.
export function CommitComparison({ before, after }: { before: ConfirmValues; after: ConfirmValues }) {
  return (
    <>
      <CompareRow label="Message" before={before.message} after={after.message} multiline />
      <CompareRow label="Author Date" before={before.dateText} after={after.dateText} />
      <CompareRow label="Committer Date" before={before.committerDateText} after={after.committerDateText} />
      <CompareRow label="Author Name" before={before.authorName} after={after.authorName} />
      <CompareRow label="Author Email" before={before.authorEmail} after={after.authorEmail} />
    </>
  )
}

interface AffectedRefsNoticeProps {
  refs: app.AffectedRef[]
  moveBranches: boolean
  onMoveBranchesChange: (moveBranches: boolean) => void
  disabled: boolean
}

function RefNames({ refs }: { refs: app.AffectedRef[] }) {
  return (
    <span className="font-mono text-gray-100">
      {refs.map((ref) => ref.name).join(', ')}
    </span>
  )
}

// Lists the other branches and tags that point at (or build on) a commit the
// edit rewrites. Branches pointing at a rewritten commit can be moved along,
// like `git rebase --update-refs`; tags and forked branches are only warned about.
function AffectedRefsNotice({ refs, moveBranches, onMoveBranchesChange, disabled }: AffectedRefsNoticeProps) {
  const branches = refs.filter((ref) => ref.kind === 'branch')
  const forks = refs.filter((ref) => ref.kind === 'forked-branch')
  const tags = refs.filter((ref) => ref.kind === 'tag')
  if (refs.length === 0) {
    return null
  }

  return (
    <div className="space-y-2 rounded-lg border border-yellow-900/60 bg-yellow-950/20 p-3 text-sm text-yellow-200">
      <div className="text-xs font-medium uppercase tracking-wide text-yellow-300/80">Other branches and tags</div>
      {branches.length > 0 && (
        <label className="flex items-start gap-2">
          <input
            type="checkbox"
            checked={moveBranches}
            onChange={(e) => onMoveBranchesChange(e.target.checked)}
            disabled={disabled}
            className="mt-1 accent-indigo-500 disabled:cursor-not-allowed"
          />
          <span>
            Also move <RefNames refs={branches} /> to the edited {branches.length === 1 ? 'commit' : 'commits'}.
            {!moveBranches && ' Otherwise they keep pointing at the old commits.'}
          </span>
        </label>
      )}
      {forks.length > 0 && (
        <p>
          <RefNames refs={forks} /> {forks.length === 1 ? 'has its' : 'have their'} own commits on top of a rewritten
          commit and will keep the old version in {forks.length === 1 ? 'its' : 'their'} history. Rebase{' '}
          {forks.length === 1 ? 'it' : 'them'} yourself if needed.
        </p>
      )}
      {tags.length > 0 && (
        <p>
          {tags.length === 1 ? 'Tag' : 'Tags'} <RefNames refs={tags} /> will keep pointing at the old{' '}
          {tags.length === 1 ? 'commit' : 'commits'}; GitGo never moves tags.
        </p>
      )}
    </div>
  )
}

export default function ConfirmDialog({
  isOpen,
  isSubmitting,
  title,
  children,
  affectedRefs,
  moveBranches,
  onMoveBranchesChange,
  onCancel,
  onConfirm,
}: ConfirmDialogProps) {
  // While open, the dialog owns Escape (cancel) and blocks Ctrl+Z so the
  // app-wide shortcuts cannot clear the selection or undo behind the modal.
  // The listener runs in the capture phase so it fires before the app-wide
  // window listener, and stops the event from reaching it.
  useEffect(() => {
    if (!isOpen) {
      return
    }

    function handleKeyDown(event: KeyboardEvent) {
      const isUndoShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z'

      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        if (!isSubmitting) {
          onCancel()
        }
      } else if (isUndoShortcut) {
        event.preventDefault()
        event.stopPropagation()
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)
    return () => window.removeEventListener('keydown', handleKeyDown, true)
  }, [isOpen, isSubmitting, onCancel])

  if (!isOpen) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/75 p-4">
      <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 shadow-2xl">
        <div className="border-b border-gray-800 px-5 py-4">
          <h2 className="text-lg font-semibold text-gray-100">{title}</h2>
          <p className="mt-1 text-sm text-gray-400">
            Review the current values against the new values before rewriting history.
          </p>
        </div>

        <div className="space-y-4 p-5">
          {children}
          <AffectedRefsNotice
            refs={affectedRefs}
            moveBranches={moveBranches}
            onMoveBranchesChange={onMoveBranchesChange}
            disabled={isSubmitting}
          />
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-gray-800 px-5 py-4">
          <button
            type="button"
            onClick={onCancel}
            disabled={isSubmitting}
            className="rounded-md border border-gray-700 px-4 py-2 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={isSubmitting}
            className="flex items-center gap-2 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isSubmitting && <Spinner />}
            {isSubmitting ? 'Rewriting history…' : 'Apply'}
          </button>
        </div>
      </div>
    </div>
  )
}
