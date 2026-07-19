import { useEffect, useState } from 'react'
import { GetCommitDetail, RefreshLog, UpdateCommit } from '../../wailsjs/go/app/App'
import ConfirmDialog, { ConfirmValues } from './ConfirmDialog'
import { useRepoStore } from '../store/repoStore'

interface EditFormState {
  message: string
  authorName: string
  authorEmail: string
  // datetime-local input format: YYYY-MM-DDTHH:mm
  dateLocal: string
}

function toLocalDateTimeInputValue(rfc3339: string): string {
  const date = new Date(rfc3339)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  const pad = (value: number) => String(value).padStart(2, '0')
  const year = date.getFullYear()
  const month = pad(date.getMonth() + 1)
  const day = pad(date.getDate())
  const hour = pad(date.getHours())
  const minute = pad(date.getMinutes())

  return `${year}-${month}-${day}T${hour}:${minute}`
}

function toRfc3339FromLocalDateTime(localValue: string): string {
  const date = new Date(localValue)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return date.toISOString()
}

function toPreviewDateText(localValue: string): string {
  if (!localValue) {
    return ''
  }

  const date = new Date(localValue)
  if (Number.isNaN(date.getTime())) {
    return localValue
  }

  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

function formsEqual(left: EditFormState, right: EditFormState): boolean {
  return (
    left.message === right.message &&
    left.authorName === right.authorName &&
    left.authorEmail === right.authorEmail &&
    left.dateLocal === right.dateLocal
  )
}

function formToConfirmValues(form: EditFormState): ConfirmValues {
  return {
    message: form.message,
    authorName: form.authorName,
    authorEmail: form.authorEmail,
    dateText: toPreviewDateText(form.dateLocal),
  }
}

export default function EditPanel() {
  const selectedHash = useRepoStore((s) => s.selectedHash)
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const setRepo = useRepoStore((s) => s.setRepo)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)

  const [isLoading, setIsLoading] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [isUnpushed, setIsUnpushed] = useState(false)
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)
  const [originalForm, setOriginalForm] = useState<EditFormState | null>(null)
  const [form, setForm] = useState<EditFormState>({
    message: '',
    authorName: '',
    authorEmail: '',
    dateLocal: '',
  })

  useEffect(() => {
    let isActive = true

    if (!selectedHash) {
      setIsLoading(false)
      setLoadError(null)
      setIsUnpushed(false)
      setShowConfirmDialog(false)
      setOriginalForm(null)
      setForm({ message: '', authorName: '', authorEmail: '', dateLocal: '' })
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

        setIsUnpushed(detail.isUnpushed)
        const loadedForm = {
          message: detail.message,
          authorName: detail.authorName,
          authorEmail: detail.authorEmail,
          dateLocal: toLocalDateTimeInputValue(detail.date),
        }
        setOriginalForm(loadedForm)
        setForm(loadedForm)
      } catch (error) {
        if (!isActive) {
          return
        }
        setLoadError(String(error))
      } finally {
        if (isActive) {
          setIsLoading(false)
        }
      }
    }

    loadCommitDetail()

    return () => {
      isActive = false
    }
  }, [selectedHash])

  const fieldsDisabled = !selectedHash || isLoading || isSubmitting || !isUnpushed
  const hasChanges = originalForm !== null && !formsEqual(form, originalForm)

  async function handleConfirmApply() {
    if (!selectedHash || !repoInfo || !originalForm) {
      return
    }

    const rfc3339Date = toRfc3339FromLocalDateTime(form.dateLocal)
    if (!rfc3339Date) {
      setError('Enter a valid date and time before applying changes.')
      setShowConfirmDialog(false)
      return
    }

    setIsSubmitting(true)

    try {
      const result = await UpdateCommit({
        hash: selectedHash,
        message: form.message,
        authorName: form.authorName,
        authorEmail: form.authorEmail,
        date: rfc3339Date,
      })

      const refreshedCommits = await RefreshLog()
      setRepo(repoInfo, refreshedCommits)

      if (result.success) {
        setError(null)
        setStatus(result.message)
      } else {
        setError(result.message)
      }

      setShowConfirmDialog(false)
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
            No commit selected yet.
          </div>
        )}

        {selectedHash && (
          <div className="mt-3 rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-xs text-gray-400">
            Commit: <span className="font-mono text-gray-300">{selectedHash.slice(0, 12)}</span>
          </div>
        )}

        {isLoading && (
          <div className="mt-4 text-sm text-gray-400">Loading commit details…</div>
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
            if (!fieldsDisabled && hasChanges) {
              setShowConfirmDialog(true)
            }
          }}
        >
          <div>
            <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
              Message
            </label>
            <textarea
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
              Date & Time
            </label>
            <input
              type="datetime-local"
              value={form.dateLocal}
              onChange={(e) => setForm((current) => ({ ...current, dateLocal: e.target.value }))}
              disabled={fieldsDisabled}
              className="mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            />
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
              disabled={fieldsDisabled || !hasChanges}
              className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Review Changes
            </button>
          </div>
        </form>
        </div>
      </aside>

      {originalForm && (
        <ConfirmDialog
          isOpen={showConfirmDialog}
          isSubmitting={isSubmitting}
          before={formToConfirmValues(originalForm)}
          after={formToConfirmValues(form)}
          onCancel={() => setShowConfirmDialog(false)}
          onConfirm={handleConfirmApply}
        />
      )}
    </>
  )
}
