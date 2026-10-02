import { useEffect, useRef, useState } from 'react'
import {
  ChangeTrailers,
  GetAffectedRefs,
  GetCommitDetail,
  GetCommitLog,
  GetGitIdentity,
  GetSignedCommits,
  OpenCommitOnWeb,
  RefreshLog,
  UpdateCommit,
} from '../../wailsjs/go/app/App'
import { app } from '../../wailsjs/go/models'
import ConfirmDialog, { CommitComparison, ConfirmValues } from './ConfirmDialog'
import DateShiftButtons, { DATE_BUTTON_CLASS } from './DateShiftButtons'
import DateTimeField from './DateTimeField'
import Kbd from './Kbd'
import Spinner from './Spinner'
import {
  nowWallClock,
  shiftWallClock,
  splitRfc3339,
  toPreviewDateText,
  WALL_CLOCK_PATTERN,
} from '../dates'
import { errorText, friendlyError } from '../errors'
import { identityErrors, NO_IDENTITY_ERRORS, type CommitterMode } from '../identity'
import { lineLength, messageGuideHints } from '../messageGuides'
import { useRepoStore } from '../store/repoStore'
import { CO_AUTHORED_BY, formatPerson, formatTrailer, SIGNED_OFF_BY } from '../trailers'
import CoAuthorPicker from './CoAuthorPicker'
import CommitterFields from './CommitterFields'
import UseMyIdentityButton from './UseMyIdentityButton'

interface EditFormState {
  message: string
  authorName: string
  authorEmail: string
  // Wall-clock time in the commit's own time zone, in datetime-local input
  // format with seconds: YYYY-MM-DDTHH:mm:ss
  dateLocal: string
  // Time zone offset of the commit date: +HH:MM or -HH:MM
  offset: string
  // Also set the committer date to the author date.
  syncCommitterDate: boolean
  // What happens to the committer name and email; committerName and
  // committerEmail are only used with 'set'.
  committerMode: CommitterMode
  committerName: string
  committerEmail: string
}

// Committer of the loaded commit, with its date split like the form.
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
  committerMode: 'keep',
  committerName: '',
  committerEmail: '',
}

// Outer box of the edit panels. On wide windows it fits a 72-character line
// of the monospace message field, the usual wrap width for commit message
// bodies, but never takes more than half the window. The panel uses the
// message field's font so 72ch measures the same characters; PANEL_BODY_CLASS
// switches its content back to the normal font. 5rem covers the paddings,
// borders and scrollbars.
export const PANEL_CLASS =
  'h-full min-h-0 border-t lg:border-t-0 lg:border-l border-gray-800 bg-gray-900/60 font-mono text-sm lg:w-[calc(72ch_+_5rem)] lg:max-w-[50vw]'
export const PANEL_BODY_CLASS = 'h-full overflow-y-auto p-4 sm:p-5 font-sans text-base'

// The committer name and email the form would give the commit.
function newCommitter(form: EditFormState, committer: CommitterInfo): { name: string; email: string } {
  switch (form.committerMode) {
    case 'author':
      return { name: form.authorName, email: form.authorEmail }
    case 'set':
      return { name: form.committerName, email: form.committerEmail }
    default:
      return { name: committer.name, email: committer.email }
  }
}

// Compares the resulting commits rather than the committer controls, so
// picking "Same as author" when the two already match is not a change.
function formsEqual(left: EditFormState, right: EditFormState, committer: CommitterInfo): boolean {
  const leftCommitter = newCommitter(left, committer)
  const rightCommitter = newCommitter(right, committer)
  return (
    left.message === right.message &&
    left.authorName === right.authorName &&
    left.authorEmail === right.authorEmail &&
    left.dateLocal === right.dateLocal &&
    left.offset === right.offset &&
    left.syncCommitterDate === right.syncCommitterDate &&
    leftCommitter.name === rightCommitter.name &&
    leftCommitter.email === rightCommitter.email
  )
}

function formToConfirmValues(form: EditFormState, committer: CommitterInfo): ConfirmValues {
  const { name, email } = newCommitter(form, committer)
  return {
    message: form.message,
    authorName: form.authorName,
    authorEmail: form.authorEmail,
    dateText: toPreviewDateText(form),
    committerText: `${name} <${email}>`,
    committerDateText: toPreviewDateText(form.syncCommitterDate ? form : committer),
  }
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
  const messageGuides = useRepoStore((s) => s.messageGuides)

  const messageRef = useRef<HTMLTextAreaElement>(null)
  // Scroll position of the message field, which the subject ruler follows.
  const [messageScrollTop, setMessageScrollTop] = useState(0)
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
  // Other branches and tags the edit would leave behind, loaded when the
  // confirm dialog opens.
  const [affectedRefs, setAffectedRefs] = useState<app.AffectedRef[]>([])
  const [signedCommits, setSignedCommits] = useState<app.SignedCommit[]>([])
  const [moveBranches, setMoveBranches] = useState(true)
  // "Back up first" in ConfirmDialog; starts from the setting each time it opens.
  const [backup, setBackup] = useState(true)

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
          // Likewise, a committer who is the author follows author changes,
          // since a wrong identity usually affects both.
          committerMode:
            detail.committerName === detail.authorName && detail.committerEmail === detail.authorEmail
              ? ('author' as const)
              : ('keep' as const),
          committerName: detail.committerName,
          committerEmail: detail.committerEmail,
        }
        setCommitter({
          name: detail.committerName,
          email: detail.committerEmail,
          ...splitRfc3339(detail.committerDate),
        })
        setOriginalForm(loadedForm)
        setForm(loadedForm)
        // Show the new message from its first line, with the subject ruler.
        if (messageRef.current) {
          messageRef.current.scrollTop = 0
        }
        setMessageScrollTop(0)
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
  const hasChanges = originalForm !== null && committer !== null && !formsEqual(form, originalForm, committer)
  const fieldErrors = identityErrors(form.authorName, form.authorEmail)
  const committerErrors =
    form.committerMode === 'set'
      ? identityErrors(form.committerName, form.committerEmail, 'Committer')
      : NO_IDENTITY_ERRORS
  const isValid = [fieldErrors, committerErrors].every((errors) => errors.name === null && errors.email === null)
  const committerPreview = committer ? newCommitter(form, committer) : null
  const subjectLength = lineLength(form.message.split('\n')[0])
  const messageHints = messageGuideHints(form.message, messageGuides)

  async function openConfirmDialog() {
    if (!selectedHash) {
      return
    }
    try {
      const [refs, signed] = await Promise.all([
        GetAffectedRefs([selectedHash]),
        GetSignedCommits([selectedHash]),
      ])
      setAffectedRefs(refs)
      setSignedCommits(signed)
    } catch (error) {
      setError(errorText(error))
      return
    }
    setMoveBranches(true)
    setBackup(useRepoStore.getState().backupSettings.backupBeforeApply)
    setShowConfirmDialog(true)
  }

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
        let result
        try {
          result = await UpdateCommit({
            hash: hashToUpdate,
            message: form.message,
            authorName: form.authorName,
            authorEmail: form.authorEmail,
            date: rfc3339Date,
            syncCommitterDate: form.syncCommitterDate,
            committer: form.committerMode,
            committerName: form.committerMode === 'set' ? form.committerName : '',
            committerEmail: form.committerMode === 'set' ? form.committerEmail : '',
            moveBranches: moveBranches
              ? affectedRefs.filter((ref) => ref.kind === 'branch').map((ref) => ref.name)
              : [],
            backup,
          })
        } catch (error) {
          // The backend re-reads the repository before editing. When the
          // commit turns out to have been pushed in the meantime, show the
          // fresh pushed/unpushed state so the panel becomes read-only.
          if (/already been pushed/i.test(errorText(error))) {
            const commits = await GetCommitLog().catch(() => null)
            if (commits) {
              useRepoStore.setState({ commits })
            }
            setShowConfirmDialog(false)
          }
          throw error
        }

        const refreshedCommits = await RefreshLog()
        setRepo(repoInfo, refreshedCommits)
        // The rewrite itself succeeded even when result.success is false (only
        // moving some other branches failed), so it can always be undone here.
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

  // Adds trailer to the end of the message (see ChangeTrailers), unless the
  // message already has it.
  async function addTrailer(trailer: app.Trailer) {
    try {
      const message = await ChangeTrailers(form.message, app.TrailerChange.createFrom({ add: [trailer], remove: [] }))
      if (message === form.message) {
        setStatus(`The message already has ${formatTrailer(trailer)}`)
        return
      }
      setForm((current) => ({ ...current, message }))
      setError(null)
    } catch (error) {
      setError(errorText(error))
    }
  }

  async function signOff() {
    try {
      const identity = await GetGitIdentity()
      if (!identity.name || !identity.email) {
        setError('Git needs both user.name and user.email set to sign off. Set them with git config.')
        return
      }
      await addTrailer({ key: SIGNED_OFF_BY, value: formatPerson(identity) })
    } catch (error) {
      setError(errorText(error))
    }
  }

  async function openOnWeb(hash: string) {
    try {
      await OpenCommitOnWeb(hash)
      setError(null)
      setStatus(`Opened the commit on ${repoInfo?.webHost}`)
    } catch (error) {
      setError(String(error))
    }
  }

  return (
    <>
      <aside className={PANEL_CLASS}>
        <div className={PANEL_BODY_CLASS}>
        <h2 className="text-base font-semibold text-gray-100">Edit Commit</h2>
        <p className="mt-1 text-xs text-gray-400">
          Select a commit to load its metadata, then review changes before applying them.
        </p>

        {!selectedHash && (
          <div className="mt-5 rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-sm text-gray-500">
            {hasUnpushedCommits ? (
              <>
                No commit selected yet. Click a commit, or use <Kbd>↑</Kbd> <Kbd>↓</Kbd> and{' '}
                <Kbd>Enter</Kbd>. Ctrl- or Shift-click to shift the dates or set the author of several unpushed commits at
                once.
              </>
            ) : (
              'There are no unpushed commits on this branch. Pushed commits can be viewed but not edited.'
            )}
          </div>
        )}

        {selectedHash && (
          <div className="mt-3 flex items-center justify-between gap-2 rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-xs text-gray-400">
            <span>
              Commit: <span className="font-mono text-gray-300">{selectedHash.slice(0, 12)}</span>
            </span>
            {/* Unpushed commits are not on the hosting service yet. */}
            {repoInfo?.webHost && !isUnpushed && (
              <button
                type="button"
                onClick={() => void openOnWeb(selectedHash)}
                title={`Open this commit on ${repoInfo.webHost} in your browser`}
                className="text-indigo-300 hover:text-indigo-200 hover:underline"
              >
                View on {repoInfo.webHost} ↗
              </button>
            )}
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
            if (!fieldsDisabled && hasChanges && isValid && activity === null) {
              openConfirmDialog()
            }
          }}
        >
          <div>
            <div className="flex items-center justify-between gap-2">
              <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
                Message
              </label>
              {!fieldsDisabled && messageGuides.subject > 0 && (
                <span
                  title="Subject line length"
                  className={`font-mono text-xs ${subjectLength > messageGuides.subject ? 'text-yellow-300' : 'text-gray-500'}`}
                >
                  {subjectLength}/{messageGuides.subject}
                </span>
              )}
            </div>
            {/* The ruler is measured in ch of the textarea's font, from its
                left border (1px) and padding (px-3). */}
            <div className="relative mt-1 overflow-hidden rounded-md font-mono text-sm">
              <textarea
                ref={messageRef}
                value={form.message}
                onChange={(e) => setForm((current) => ({ ...current, message: e.target.value }))}
                onScroll={(e) => setMessageScrollTop(e.currentTarget.scrollTop)}
                disabled={fieldsDisabled}
                rows={5}
                className="block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 font-mono text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
                placeholder="Commit message"
              />
              {/* The subject ruler only spans the first line (py-2 plus one
                  text-sm line), so it moves with the text when it scrolls.
                  There is no body ruler: the field is about 72ch wide and
                  wraps long lines, so they could never cross one; the hints
                  below report them instead. */}
              {!fieldsDisabled && messageGuides.subject > 0 && (
                <div
                  aria-hidden
                  className="pointer-events-none absolute h-5 w-px bg-gray-600"
                  style={{
                    left: `calc(0.75rem + 1px + ${messageGuides.subject}ch)`,
                    top: `calc(0.5rem + 1px - ${messageScrollTop}px)`,
                  }}
                />
              )}
            </div>
            {!fieldsDisabled && messageHints.length > 0 && (
              <ul className="mt-1 space-y-0.5 text-xs text-yellow-300">
                {messageHints.map((hint) => (
                  <li key={hint}>{hint}</li>
                ))}
              </ul>
            )}
            <div className="mt-1.5 flex flex-wrap items-center justify-end gap-1">
              <CoAuthorPicker
                label="Add co-author"
                title="Add a Co-authored-by line for someone who worked on this commit"
                disabled={fieldsDisabled}
                onPick={(person) => void addTrailer({ key: CO_AUTHORED_BY, value: formatPerson(person) })}
              />
              <button
                type="button"
                title="Add a Signed-off-by line with your user.name and user.email"
                disabled={fieldsDisabled}
                onClick={() => void signOff()}
                className={DATE_BUTTON_CLASS}
              >
                Sign off
              </button>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
              Author Date
            </label>
            <DateTimeField
              value={{ dateLocal: form.dateLocal, offset: form.offset }}
              onChange={({ dateLocal, offset }) => setForm((current) => ({ ...current, dateLocal, offset }))}
              disabled={fieldsDisabled}
              extraOffsets={originalForm ? [originalForm.offset] : []}
            />
            <div className="mt-1.5">
              <DateShiftButtons
                disabled={fieldsDisabled || !WALL_CLOCK_PATTERN.test(form.dateLocal)}
                onShift={(minutes) =>
                  setForm((current) => {
                    const dateLocal = shiftWallClock(current.dateLocal, minutes)
                    return dateLocal ? { ...current, dateLocal } : current
                  })
                }
              >
                <button
                  type="button"
                  title="Current time in this computer's time zone"
                  disabled={fieldsDisabled}
                  onClick={() => setForm((current) => ({ ...current, ...nowWallClock() }))}
                  className={DATE_BUTTON_CLASS}
                >
                  Now
                </button>
              </DateShiftButtons>
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
          </div>

          <div>
            <div className="flex items-center justify-between gap-2">
              <label className="block text-xs font-medium uppercase tracking-wide text-gray-400">
                Author Name
              </label>
              <UseMyIdentityButton
                disabled={fieldsDisabled}
                onIdentity={(authorName, authorEmail) =>
                  setForm((current) => ({ ...current, authorName, authorEmail }))
                }
              />
            </div>
            <input
              type="text"
              value={form.authorName}
              onChange={(e) => setForm((current) => ({ ...current, authorName: e.target.value }))}
              disabled={fieldsDisabled}
              className="mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              placeholder="Author name"
            />
            {!fieldsDisabled && fieldErrors.name && (
              <p className="mt-1 text-xs text-red-300">{fieldErrors.name}</p>
            )}
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
            {!fieldsDisabled && fieldErrors.email && (
              <p className="mt-1 text-xs text-red-300">{fieldErrors.email}</p>
            )}
          </div>

          <CommitterFields
            mode={form.committerMode}
            name={form.committerName}
            email={form.committerEmail}
            errors={committerErrors}
            disabled={fieldsDisabled}
            authorOptionLabel="Same as author"
            onModeChange={(committerMode) => setForm((current) => ({ ...current, committerMode }))}
            onIdentityChange={(committerName, committerEmail) =>
              setForm((current) => ({ ...current, committerName, committerEmail }))
            }
          >
            {committer && committerPreview && (
              <p className="mt-1 break-words text-xs text-gray-500">
                {committerPreview.name} &lt;{committerPreview.email}&gt;,{' '}
                {toPreviewDateText(form.syncCommitterDate ? form : committer)}
              </p>
            )}
          </CommitterFields>

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
              disabled={fieldsDisabled || !hasChanges || !isValid || activity !== null}
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
          title="Confirm Commit Update"
          signedCommits={signedCommits}
          affectedRefs={affectedRefs}
          moveBranches={moveBranches}
          onMoveBranchesChange={setMoveBranches}
          backup={backup}
          onBackupChange={setBackup}
          onCancel={() => setShowConfirmDialog(false)}
          onConfirm={handleConfirmApply}
        >
          <CommitComparison
            // "Current" always shows the stored committer date, whatever the
            // checkbox started as.
            before={formToConfirmValues({ ...originalForm, syncCommitterDate: false }, committer)}
            after={formToConfirmValues(form, committer)}
          />
        </ConfirmDialog>
      )}
    </>
  )
}
