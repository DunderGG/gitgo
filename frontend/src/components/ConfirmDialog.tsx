interface ConfirmDialogProps {
  isOpen: boolean
  isSubmitting: boolean
  before: ConfirmValues
  after: ConfirmValues
  onCancel: () => void
  onConfirm: () => void
}

export interface ConfirmValues {
  message: string
  authorName: string
  authorEmail: string
  dateText: string
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

export default function ConfirmDialog({
  isOpen,
  isSubmitting,
  before,
  after,
  onCancel,
  onConfirm,
}: ConfirmDialogProps) {
  if (!isOpen) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/75 p-4">
      <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 shadow-2xl">
        <div className="border-b border-gray-800 px-5 py-4">
          <h2 className="text-lg font-semibold text-gray-100">Confirm Commit Update</h2>
          <p className="mt-1 text-sm text-gray-400">
            Review the current values against the new values before rewriting history.
          </p>
        </div>

        <div className="space-y-4 p-5">
          <CompareRow label="Message" before={before.message} after={after.message} multiline />
          <CompareRow label="Date & Time" before={before.dateText} after={after.dateText} />
          <CompareRow label="Author Name" before={before.authorName} after={after.authorName} />
          <CompareRow label="Author Email" before={before.authorEmail} after={after.authorEmail} />
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
            className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isSubmitting ? 'Applying…' : 'Apply'}
          </button>
        </div>
      </div>
    </div>
  )
}
