import type { ReactNode } from 'react'
import type { CommitterMode, IdentityErrors } from '../identity'
import UseMyIdentityButton from './UseMyIdentityButton'

interface CommitterFieldsProps {
  mode: CommitterMode
  name: string
  email: string
  errors: IdentityErrors
  disabled: boolean
  // Label of the "author" option: one commit has one author, several may not.
  authorOptionLabel: string
  onModeChange: (mode: CommitterMode) => void
  onIdentityChange: (name: string, email: string) => void
  // Shown under the fields, e.g. the resulting committer.
  children?: ReactNode
}

const INPUT_CLASS =
  'mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60'

// Committer section shared by EditPanel and BulkEditPanel: keep the committer
// name and email, copy them from the author, or type new ones. The committer
// date is handled by each panel's date controls.
export default function CommitterFields({
  mode,
  name,
  email,
  errors,
  disabled,
  authorOptionLabel,
  onModeChange,
  onIdentityChange,
  children,
}: CommitterFieldsProps) {
  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <label
          htmlFor="committer-mode"
          className="block text-xs font-medium uppercase tracking-wide text-gray-400"
        >
          Committer
        </label>
        {mode === 'set' && (
          <UseMyIdentityButton disabled={disabled} onIdentity={onIdentityChange} />
        )}
      </div>
      <select
        id="committer-mode"
        value={mode}
        onChange={(e) => onModeChange(e.target.value as CommitterMode)}
        disabled={disabled}
        className="mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-2 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
      >
        <option value="keep">Keep name and email</option>
        <option value="author">{authorOptionLabel}</option>
        <option value="set">Set name and email</option>
      </select>
      {mode === 'set' && (
        <>
          <input
            type="text"
            value={name}
            onChange={(e) => onIdentityChange(e.target.value, email)}
            disabled={disabled}
            aria-label="Committer name"
            className={INPUT_CLASS}
            placeholder="Committer name"
          />
          {!disabled && errors.name && <p className="mt-1 text-xs text-red-300">{errors.name}</p>}
          <input
            type="email"
            value={email}
            onChange={(e) => onIdentityChange(name, e.target.value)}
            disabled={disabled}
            aria-label="Committer email"
            className={INPUT_CLASS}
            placeholder="committer@example.com"
          />
          {!disabled && errors.email && <p className="mt-1 text-xs text-red-300">{errors.email}</p>}
        </>
      )}
      {children}
    </div>
  )
}
