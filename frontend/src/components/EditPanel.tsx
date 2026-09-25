import { useEffect, useRef, useState, type ReactNode } from 'react'
import { GetCommitDetail, RefreshLog, UpdateCommit } from '../../wailsjs/go/app/App'
import ConfirmDialog, { ConfirmValues } from './ConfirmDialog'
import Spinner from './Spinner'
import { errorText, friendlyError } from '../errors'
import { useRepoStore } from '../store/repoStore'

interface EditFormState {
  message: string
  authorName: string
  authorEmail: string
  // Wall-clock time in the commit's own time zone, in datetime-local input
  // format with seconds: YYYY-MM-DDTHH:mm:ss
  dateLocal: string
  // Time zone offset of the commit date: +HH:MM or -HH:MM
  offset: string
  // Also set the committer date to the author date. The committer name and
  // email are always kept.
  syncCommitterDate: boolean
}

// Read-only committer of the loaded commit, with its date split like the form.
interface CommitterInfo {
  name: string
  email: string
  dateLocal: string
  offset: string
}

const EMPTY_FORM: EditFormState = {
  message: '',
  authorName: '',
  authorEmail: '',
  dateLocal: '',
  offset: '+00:00',
  syncCommitterDate: false,
}

// UTC offsets in use around the world. The commit's original offset is added
// to the list when it is not one of these.
const COMMON_OFFSETS = [
  '-12:00', '-11:00', '-10:00', '-09:30', '-09:00', '-08:00', '-07:00', '-06:00',
  '-05:00', '-04:00', '-03:30', '-03:00', '-02:00', '-01:00', '+00:00', '+01:00',
  '+02:00', '+03:00', '+03:30', '+04:00', '+04:30', '+05:00', '+05:30', '+05:45',
  '+06:00', '+06:30', '+07:00', '+08:00', '+08:45', '+09:00', '+09:30', '+10:00',
  '+10:30', '+11:00', '+12:00', '+12:45', '+13:00', '+14:00',
]

const WALL_CLOCK_PATTERN = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/

// Splits an RFC 3339 date from the backend into its wall-clock part and
// offset, keeping the commit's own time zone instead of converting to local.
function splitRfc3339(rfc3339: string): Pick<EditFormState, 'dateLocal' | 'offset'> {
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/.exec(rfc3339)
  if (!match) {
    return { dateLocal: '', offset: EMPTY_FORM.offset }
  }
  return { dateLocal: match[1], offset: match[2] === 'Z' ? '+00:00' : match[2] }
}

// datetime-local inputs leave out ":00" seconds in their value, so add them back.
function normalizeWallClock(inputValue: string): string {
  return inputValue.length === 16 ? `${inputValue}:00` : inputValue.slice(0, 19)
}

function formatOffset(totalMinutes: number): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  const sign = totalMinutes < 0 ? '-' : '+'
  const absolute = Math.abs(totalMinutes)
  return `${sign}${pad(Math.floor(absolute / 60))}:${pad(absolute % 60)}`
}

function offsetMinutes(offset: string): number {
  const sign = offset.startsWith('-') ? -1 : 1
  const [hours, minutes] = offset.slice(1).split(':').map(Number)
  return sign * (hours * 60 + minutes)
}

// Offset of this computer's time zone at the given wall-clock time, used to
// label the matching option.
function localOffsetAt(dateLocal: string): string | null {
  const date = new Date(dateLocal)
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return formatOffset(-date.getTimezoneOffset())
}

function toPreviewDateText({ dateLocal, offset }: { dateLocal: string; offset: string }): string {
  if (!dateLocal) {
    return ''
  }

  // Parse the wall-clock time as UTC and format it in UTC so the browser's own
  // time zone does not shift it; the commit's offset is shown separately.
  const date = new Date(`${dateLocal}Z`)
  if (Number.isNaN(date.getTime())) {
    return `${dateLocal} (UTC${offset})`
  }

  const wallClock = date.toLocaleString(undefined, {
    timeZone: 'UTC',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
  })
  return `${wallClock} (UTC${offset})`
}

function formsEqual(left: EditFormState, right: EditFormState): boolean {
  return (
    left.message === right.message &&
    left.authorName === right.authorName &&
    left.authorEmail === right.authorEmail &&
    left.dateLocal === right.dateLocal &&
    left.offset === right.offset &&
    left.syncCommitterDate === right.syncCommitterDate
  )
}

function formToConfirmValues(form: EditFormState, committer: CommitterInfo): ConfirmValues {
  return {
    message: form.message,
    authorName: form.authorName,
    authorEmail: form.authorEmail,
    dateText: toPreviewDateText(form),
    committerDateText: toPreviewDateText(form.syncCommitterDate ? form : committer),
  }
}

function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="rounded border border-gray-700 bg-gray-800 px-1 font-mono text-[11px] text-gray-300">
      {children}
    </kbd>
  )
}

export default function EditPanel() {
  const selectedHash = useRepoStore((s) => s.selectedHash)
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const setCanUndo = useRepoStore((s) => s.setCanUndo)
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)
  const hasUnpushedCommits = useRepoStore((s) => s.commits.some((commit) => commit.isUnpushed))
  const pendingEditFocus = useRepoStore((s) => s.pendingEditFocus)
  const consumeEditFocus = useRepoStore((s) => s.consumeEditFocus)

  const messageRef = useRef<HTMLTextAreaElement>(null)
  // Hash whose detail request has finished (successfully or not). Compared
  // with selectedHash so the focus effect never acts on the previous commit's
  // state in the render right after the selection changes.
  const [loadedHash, setLoadedHash] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  // Read from the commit list rather than the loaded detail, so a reload that
  // finds the commit was pushed in the meantime makes it read-only at once.
  const isUnpushed = useRepoStore(
    (s) => s.commits.find((commit) => commit.hash === selectedHash)?.isUnpushed ?? false,
  )
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)
  const [originalForm, setOriginalForm] = useState<EditFormState | null>(null)
  const [form, setForm] = useState<EditFormState>(EMPTY_FORM)
  const [committer, setCommitter] = useState<CommitterInfo | null>(null)

  useEffect(() => {
    let isActive = true

    if (!selectedHash) {
      setLoadedHash(null)
      setIsLoading(false)
      setLoadError(null)
      setShowConfirmDialog(false)
      setOriginalForm(null)
      setCommitter(null)
      setForm(EMPTY_FORM)
      return () => {
        isActive = false
      }
    }

    // Capture the hash once for the async request so TypeScript can prove
    // it is non-null inside the loader.
    const hashToLoad = selectedHash

    async function loadCommitDetail() {
      setIsLoading(true)
      setLoadError(null)

      try {
        const detail = await GetCommitDetail(hashToLoad)
        if (!isActive) {
          return
        }

        const loadedForm = {
          message: detail.message,
          authorName: detail.authorName,
          authorEmail: detail.authorEmail,
          ...splitRfc3339(detail.date),
          // Keep the dates together when they already match, which is the
          // usual case for commits nobody has rebased or amended.
          syncCommitterDate: detail.committerDate === detail.date,
        }
        setCommitter({
          name: detail.committerName,
          email: detail.committerEmail,
          ...splitRfc3339(detail.committerDate),
        })
        setOriginalForm(loadedForm)
        setForm(loadedForm)
      } catch (error) {
        if (!isActive) {
          return
        }
        setLoadError(friendlyError(errorText(error)))
      } finally {
        if (isActive) {
          setIsLoading(false)
          setLoadedHash(hashToLoad)
        }
      }
    }

    loadCommitDetail()

    return () => {
      isActive = false
    }
  }, [selectedHash])

  const fieldsDisabled = !selectedHash || isLoading || isSubmitting || !isUnpushed

  // Handle the Enter shortcut from CommitList: once the selected commit has
  // loaded, focus the message field if it is editable. The request is consumed
  // either way so a later click does not steal focus unexpectedly.
  useEffect(() => {
    if (!pendingEditFocus || isLoading || !selectedHash || loadedHash !== selectedHash) {
      return
    }
    consumeEditFocus()
    if (!fieldsDisabled && !loadError) {
      messageRef.current?.focus()
    }
  }, [pendingEditFocus, isLoading, selectedHash, loadedHash, fieldsDisabled, loadError, consumeEditFocus])
  const hasChanges = originalForm !== null && !formsEqual(form, originalForm)

  const offsetOptions = [...COMMON_OFFSETS]
  for (const offset of [originalForm?.offset, form.offset]) {
    if (offset && !offsetOptions.includes(offset)) {
      offsetOptions.push(offset)
    }
  }
  offsetOptions.sort((left, right) => offsetMinutes(left) - offsetMinutes(right))
  const localOffset = localOffsetAt(form.dateLocal)

  async function handleConfirmApply() {
    if (!selectedHash || !repoInfo || !originalForm) {
      return
    }

    if (!WALL_CLOCK_PATTERN.test(form.dateLocal)) {
      setError('Enter a valid date and time before applying changes.')
      setShowConfirmDialog(false)
      return
    }
    // An empty date tells the backend to keep the original author date, so
    // edits that do not touch the date leave it exactly as it was.
    const dateChanged =
      form.dateLocal !== originalForm.dateLocal || form.offset !== originalForm.offset
    const rfc3339Date = dateChanged ? form.dateLocal + form.offset : ''

    const hashToUpdate = selectedHash
    setIsSubmitting(true)

    try {
      await runGitOperation('Rewriting commit history…', async () => {
        const result = await UpdateCommit({
          hash: hashToUpdate,
          message: form.message,
          authorName: form.authorName,
          authorEmail: form.authorEmail,
          date: rfc3339Date,
          syncCommitterDate: form.syncCommitterDate,
        })

        const refreshedCommits = await RefreshLog()
        setRepo(repoInfo, refreshedCommits)
        // The rewrite itself succeeded even when result.success is false (only
        // the stash pop failed), so it can always be undone at this point.
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
        <h2 className="text-base font-semibold text-gray-100">Edit Commit</h2>
        <p className="mt-1 text-xs text-gray-400">
          Select a commit to load its metadata, then review changes before applying them.
        </p>

        {!selectedHash && (
          <div className="mt-5 rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-sm text-gray-500">
            {hasUnpushedCommits ? (
              <>
                No commit selected yet. Click a commit, or use <Kbd>↑</Kbd> <Kbd>↓</Kbd> and{' '}
                <Kbd>Enter</Kbd>.
              </>
            ) : (
              'There are no unpushed commits on this branch. Pushed commits can be viewed but not edited.'
            )}
          </div>
        )}

        {selectedHash && (
          <div className="mt-3 rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-xs text-gray-400">
            Commit: <span className="font-mono text-gray-300">{selectedHash.slice(0, 12)}</span>
          </div>
        )}

        {isLoading && (
          <div className="mt-4 flex items-center gap-2 text-sm text-gray-400" role="status">
            <Spinner />
            Loading commit details…
          </div>
        )}

        {loadError && (
          <div className="mt-4 rounded-lg border border-red-900/60 bg-red-950/30 px-3 py-2 text-sm text-red-300">
            Failed to load commit details: {loadError}
          </div>
        )}

        {selectedHash && !isLoading && !loadError && !isUnpushed && (
          <div className="mt-4 rounded-lg border border-yellow-900/60 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-300">
            This commit is pushed and cannot be edited.
          </div>
        )}

        <form
          className="mt-5 space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            if (!fieldsDisabled && hasChanges && activity === null) {
              setShowConfirmDialog(true)
            }
          }}
        >
          <div>
            <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
              Message
            </label>
            <textarea
              ref={messageRef}
              value={form.message}
              onChange={(e) => setForm((current) => ({ ...current, message: e.target.value }))}
              disabled={fieldsDisabled}
              rows={5}
              className="mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              placeholder="Commit message"
            />
          </div>

          <div>
            <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
              Author Date
            </label>
            <div className="mt-1 flex flex-wrap gap-2">
              <input
                type="datetime-local"
                step={1}
                value={form.dateLocal}
                onChange={(e) =>
                  setForm((current) => ({ ...current, dateLocal: normalizeWallClock(e.target.value) }))
                }
                disabled={fieldsDisabled}
                className="min-w-[13rem] flex-1 rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              />
              <select
                value={form.offset}
                onChange={(e) => setForm((current) => ({ ...current, offset: e.target.value }))}
                disabled={fieldsDisabled}
                aria-label="Time zone offset"
                className="flex-1 rounded-md border border-gray-700 bg-gray-800 px-2 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {offsetOptions.map((offset) => (
                  <option key={offset} value={offset}>
                    UTC{offset}
                    {offset === localOffset ? ' (local)' : ''}
                  </option>
                ))}
              </select>
            </div>
            <label className="mt-2 flex items-center gap-2 text-sm text-gray-300">
              <input
                type="checkbox"
                checked={form.syncCommitterDate}
                onChange={(e) =>
                  setForm((current) => ({ ...current, syncCommitterDate: e.target.checked }))
                }
                disabled={fieldsDisabled}
                className="accent-indigo-500 disabled:cursor-not-allowed"
              />
              Also set committer date
            </label>
            {committer && (
              <p className="mt-1 break-words text-xs text-gray-500">
                Committer: {committer.name} &lt;{committer.email}&gt;,{' '}
                {toPreviewDateText(form.syncCommitterDate ? form : committer)}
              </p>
            )}
          </div>

          <div>
            <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
              Author Name
            </label>
            <input
              type="text"
              value={form.authorName}
              onChange={(e) => setForm((current) => ({ ...current, authorName: e.target.value }))}
              disabled={fieldsDisabled}
              className="mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              placeholder="Author name"
            />
          </div>

          <div>
            <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
              Author Email
            </label>
            <input
              type="email"
              value={form.authorEmail}
              onChange={(e) => setForm((current) => ({ ...current, authorEmail: e.target.value }))}
              disabled={fieldsDisabled}
              className="mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              placeholder="author@example.com"
            />
          </div>

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              disabled={fieldsDisabled || !hasChanges}
              onClick={() => originalForm && setForm(originalForm)}
              className="rounded-md border border-gray-700 px-4 py-2 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Reset
            </button>
            <button
              type="submit"
              disabled={fieldsDisabled || !hasChanges || activity !== null}
              className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Review Changes
            </button>
          </div>
        </form>
        </div>
      </aside>

      {originalForm && committer && (
        <ConfirmDialog
          isOpen={showConfirmDialog}
          isSubmitting={isSubmitting}
          // "Current" always shows the stored committer date, whatever the
          // checkbox started as.
          before={formToConfirmValues({ ...originalForm, syncCommitterDate: false }, committer)}
          after={formToConfirmValues(form, committer)}
          onCancel={() => setShowConfirmDialog(false)}
          onConfirm={handleConfirmApply}
        />
      )}
    </>
  )
}
