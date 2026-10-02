import { useEffect, useState } from 'react'
import {
  EditCommits,
  GetAffectedRefs,
  GetCommitLog,
  GetGitIdentity,
  GetSignedCommits,
  PreviewTrailers,
  RefreshLog,
  SpreadDates,
} from '../../wailsjs/go/app/App'
import { app } from '../../wailsjs/go/models'
import ConfirmDialog from './ConfirmDialog'
import DateShiftButtons, { DATE_BUTTON_CLASS } from './DateShiftButtons'
import DateTimeField from './DateTimeField'
import { PANEL_BODY_CLASS, PANEL_CLASS } from './EditPanel'
import UseMyIdentityButton from './UseMyIdentityButton'
import {
  formatGap,
  formatShift,
  shiftWallClock,
  splitRfc3339,
  toPreviewDateText,
  WALL_CLOCK_PATTERN,
  type WallClockDate,
} from '../dates'
import { errorText } from '../errors'
import { identityErrors, NO_IDENTITY_ERRORS, type CommitterMode } from '../identity'
import CommitterFields from './CommitterFields'
import { useRepoStore, type CommitSummary } from '../store/repoStore'
import { formatOfficeHours } from '../officeHours'
import { CO_AUTHORED_BY, formatPerson, parsePerson, SIGNED_OFF_BY, trailerDiff, type Person } from '../trailers'
import CoAuthorPicker from './CoAuthorPicker'

// How the dates change: every commit moves by the same amount, or the
// commits are fitted between a first and last date (see SpreadDates).
type DateMode = 'shift' | 'spread'
type Spacing = 'keep' | 'even' | 'random'

// The latest SpreadDates answer, for the request in key. A changed input
// gives a new key, so an answer for older inputs is never shown as current.
interface SpreadAnswer {
  key: string
  dates: Record<string, string>
  fellBack: boolean
  error: string | null
}

// The latest PreviewTrailers answer, for the request in key (as SpreadAnswer).
interface TrailerAnswer {
  key: string
  previews: Record<string, app.TrailerPreview>
  error: string | null
}

// A co-author to add or remove; a removal without a person removes them all.
interface CoAuthorChange {
  action: 'add' | 'remove'
  person: Person | null
}

// What happens to Signed-off-by lines: kept, the user's own added, or all
// removed.
type SignOffMode = 'keep' | 'add' | 'remove'

interface DatePreview {
  commit: CommitSummary
  before: string
  // Empty when the date does not change (or is not known yet).
  after: string
  // Time since the next older selected commit, after the change; spread only.
  gap: string
}

// A random seed for random spacing, which Re-roll replaces.
const newSeed = () => Math.floor(Math.random() * 2 ** 32)

// What the spacing options do, shown under them so the result is never a
// surprise.
const SPACING_DESCRIPTIONS: Record<Spacing, string> = {
  keep: 'Keeps the gaps between the commits, scaled to fit between the first and last date: commits made close together stay close together.',
  even: 'Puts the same time between every commit.',
  random:
    'Places the commits at random, never closer together than the minimum gap, so some end up close together and others far apart. Re-roll for other dates.',
}

// Default minimum gap for random spacing, in minutes.
const DEFAULT_MIN_GAP_MINUTES = 10

// Author dates of the selected commits before and after the change, each in
// its own time zone. newDates holds the spread's dates by hash; without it
// the dates are shifted by minutes.
function datePreviews(
  selected: CommitSummary[],
  minutes: number,
  newDates: Record<string, string> | null,
): DatePreview[] {
  return selected.map((commit, index) => {
    const date = splitRfc3339(commit.date)
    let after = ''
    let gap = ''
    if (newDates) {
      const newDate = newDates[commit.hash]
      const older = selected[index + 1]
      after = newDate ? toPreviewDateText(splitRfc3339(newDate)) : ''
      if (newDate && older && newDates[older.hash]) {
        gap = formatGap((Date.parse(newDate) - Date.parse(newDates[older.hash])) / 1000)
      }
    } else if (minutes !== 0) {
      const shifted = shiftWallClock(date.dateLocal, minutes)
      after = shifted ? toPreviewDateText({ dateLocal: shifted, offset: date.offset }) : ''
    }
    return { commit, before: toPreviewDateText(date), after, gap }
  })
}

// Reports whether the change makes a commit older than the one listed below
// it, where it was not before. newInstant gives each commit's new date in
// milliseconds. The list is newest first, so this is usually a commit dated
// before its parent, which Git allows but `git log` shows out of order.
function breaksDateOrder(commits: CommitSummary[], newInstant: (commit: CommitSummary) => number): boolean {
  return commits.slice(0, -1).some((newer, index) => {
    const older = commits[index + 1]
    return Date.parse(newer.date) >= Date.parse(older.date) && newInstant(newer) < newInstant(older)
  })
}

// The trailers to add and remove for the co-author changes and sign-off mode;
// me is the user's identity, for 'add'.
function trailerChange(coAuthors: CoAuthorChange[], signOff: SignOffMode, me: Person | null) {
  const add: app.Trailer[] = []
  const remove: app.Trailer[] = []
  for (const { action, person } of coAuthors) {
    const trailer = { key: CO_AUTHORED_BY, value: person ? formatPerson(person) : '' }
    if (action === 'add') {
      add.push(trailer)
    } else {
      remove.push(trailer)
    }
  }
  if (signOff === 'add' && me) {
    add.push({ key: SIGNED_OFF_BY, value: formatPerson(me) })
  } else if (signOff === 'remove') {
    remove.push({ key: SIGNED_OFF_BY, value: '' })
  }
  return { add, remove }
}

// "+ Key: value" lines a change adds and "− Key: value" lines it removes, for
// one commit's preview.
function TrailerLines({ preview }: { preview: app.TrailerPreview | undefined }) {
  if (!preview) {
    return null
  }
  if (!preview.changed) {
    return <div className="text-gray-500">Trailers unchanged</div>
  }
  const { removed, added } = trailerDiff(preview)
  return (
    <>
      {removed.map((line) => (
        <div key={`-${line}`} className="break-words text-red-300 line-through">
          − {line}
        </div>
      ))}
      {added.map((line) => (
        <div key={`+${line}`} className="break-words text-indigo-300">
          + {line}
        </div>
      ))}
    </>
  )
}

// The time since the next older selected commit, after a spread, so the
// spacing can be checked before applying.
function GapNote({ gap }: { gap: string }) {
  return (
    <span className="ml-1.5 font-mono text-gray-500" title="Time since the next older selected commit">
      ({gap})
    </span>
  )
}

// Joins the given words as "a", "a and b" or "a, b and c", skipping false.
function joinWords(words: (string | false)[]): string {
  const kept = words.filter((word): word is string => word !== false)
  return kept.length <= 1 ? (kept[0] ?? '') : `${kept.slice(0, -1).join(', ')} and ${kept[kept.length - 1]}`
}

const INPUT_CLASS =
  'mt-1 w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60'

const DATE_MODES: { mode: DateMode; label: string }[] = [
  { mode: 'shift', label: 'Shift' },
  { mode: 'spread', label: 'Spread' },
]

const SPACINGS: { spacing: Spacing; label: string }[] = [
  { spacing: 'keep', label: 'Keep relative spacing' },
  { spacing: 'even', label: 'Even' },
  { spacing: 'random', label: 'Random' },
]

// Shown instead of EditPanel while several commits are selected: moves their
// author dates (and optionally committer dates) by the same amount or spreads
// them over a range, and/or gives them the same author or committer, in one
// rewrite which a single undo reverts.
export default function BulkEditPanel() {
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
  const officeHours = useRepoStore((s) => s.officeHours)
  const setSettingsOpen = useRepoStore((s) => s.setSettingsOpen)

  const [dateMode, setDateMode] = useState<DateMode>('shift')
  // The shift adds up across button clicks and is only applied on confirm.
  const [shiftMinutes, setShiftMinutes] = useState(0)
  // The first and last dates of the spread; null follows the oldest and
  // newest selected commit until the user changes them.
  const [firstInput, setFirstInput] = useState<WallClockDate | null>(null)
  const [lastInput, setLastInput] = useState<WallClockDate | null>(null)
  const [spacing, setSpacing] = useState<Spacing>('keep')
  const [minGapText, setMinGapText] = useState(String(DEFAULT_MIN_GAP_MINUTES))
  const [onlyOfficeHours, setOnlyOfficeHours] = useState(false)
  const [seed, setSeed] = useState(newSeed)
  const [spreadAnswer, setSpreadAnswer] = useState<SpreadAnswer | null>(null)
  const [shiftCommitter, setShiftCommitter] = useState(true)
  const [setAuthor, setSetAuthor] = useState(false)
  const [authorName, setAuthorName] = useState('')
  const [authorEmail, setAuthorEmail] = useState('')
  const [committerMode, setCommitterMode] = useState<CommitterMode>('keep')
  const [committerName, setCommitterName] = useState('')
  const [committerEmail, setCommitterEmail] = useState('')
  const [coAuthorChanges, setCoAuthorChanges] = useState<CoAuthorChange[]>([])
  const [signOffMode, setSignOffMode] = useState<SignOffMode>('keep')
  // The user's identity for 'add' sign-offs, read when that mode is chosen.
  const [me, setMe] = useState<Person | null>(null)
  const [trailerAnswer, setTrailerAnswer] = useState<TrailerAnswer | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)
  const [affectedRefs, setAffectedRefs] = useState<app.AffectedRef[]>([])
  const [signedCommits, setSignedCommits] = useState<app.SignedCommit[]>([])
  const [moveBranches, setMoveBranches] = useState(true)
  // "Back up first" in ConfirmDialog; starts from the setting each time it opens.
  const [backup, setBackup] = useState(true)

  const selectedSet = new Set(selectedHashes)
  // Newest first, like the commit list.
  const selected = commits.filter((commit) => selectedSet.has(commit.hash))
  // Commits can be pushed from a terminal after they were selected.
  const hasPushed = selected.some((commit) => !commit.isUnpushed)

  const isSpread = dateMode === 'spread'
  const first = firstInput ?? splitRfc3339(selected[selected.length - 1]?.date ?? '')
  const last = lastInput ?? splitRfc3339(selected[0]?.date ?? '')
  const minGapMinutes = /^\d+$/.test(minGapText.trim()) ? Number(minGapText.trim()) : null
  // Why the spread cannot be computed yet, checked before asking the backend.
  const inputError =
    !WALL_CLOCK_PATTERN.test(first.dateLocal) || !WALL_CLOCK_PATTERN.test(last.dateLocal)
      ? 'Enter a complete first and last date.'
      : spacing === 'random' && minGapMinutes === null
        ? 'Enter the minimum gap as a whole number of minutes.'
        : null
  const inputsReady = inputError === null
  const spreadKey = JSON.stringify({
    hashes: selected.map((commit) => commit.hash),
    first: first.dateLocal + first.offset,
    last: last.dateLocal + last.offset,
    spacing,
    minGapMinutes: spacing === 'random' ? minGapMinutes : 0,
    officeHours: onlyOfficeHours ? officeHours : null,
    seed,
  })

  // Asks the backend for the spread's dates whenever its inputs change. The
  // answer is kept with its key, so a slow answer for older inputs is ignored.
  useEffect(() => {
    if (!isSpread || !inputsReady) {
      return
    }
    let isCurrent = true
    SpreadDates(JSON.parse(spreadKey))
      .then((result) => {
        if (isCurrent) {
          setSpreadAnswer({ key: spreadKey, dates: result.dates, fellBack: result.fellBack, error: null })
        }
      })
      .catch((error) => {
        if (isCurrent) {
          setSpreadAnswer({ key: spreadKey, dates: {}, fellBack: false, error: errorText(error) })
        }
      })
    return () => {
      isCurrent = false
    }
  }, [isSpread, inputsReady, spreadKey])

  const pendingTrailers = trailerChange(coAuthorChanges, signOffMode, me)
  const hasTrailerChange = pendingTrailers.add.length > 0 || pendingTrailers.remove.length > 0
  const trailerKey = JSON.stringify({ hashes: selected.map((commit) => commit.hash), change: pendingTrailers })

  // Asks the backend for each commit's trailers before and after the change.
  // It also runs without a change, so the co-authors the selected commits
  // already have can be offered for removal.
  useEffect(() => {
    let isCurrent = true
    const { hashes, change } = JSON.parse(trailerKey)
    PreviewTrailers(hashes, app.TrailerChange.createFrom(change))
      .then((previews) => {
        if (isCurrent) {
          const byHash = Object.fromEntries(previews.map((preview) => [preview.hash, preview]))
          setTrailerAnswer({ key: trailerKey, previews: byHash, error: null })
        }
      })
      .catch((error) => {
        if (isCurrent) {
          setTrailerAnswer({ key: trailerKey, previews: {}, error: errorText(error) })
        }
      })
    return () => {
      isCurrent = false
    }
  }, [trailerKey])

  const currentTrailers = trailerAnswer?.key === trailerKey ? trailerAnswer : null
  const trailerPreviews = currentTrailers && currentTrailers.error === null ? currentTrailers.previews : null
  const trailerError = hasTrailerChange ? (currentTrailers?.error ?? null) : null
  const trailersChange =
    hasTrailerChange && trailerPreviews !== null && selected.some((commit) => trailerPreviews[commit.hash]?.changed)
  const trailersPending = hasTrailerChange && trailerPreviews === null
  // Co-authors the selected commits have now, for the remove picker. The
  // "before" trailers do not depend on the change, so any answer will do.
  const currentCoAuthors: Person[] = []
  for (const commit of selected) {
    for (const trailer of trailerAnswer?.previews[commit.hash]?.before ?? []) {
      const person = trailer.key.toLowerCase() === CO_AUTHORED_BY.toLowerCase() ? parsePerson(trailer.value) : null
      const email = person?.email.toLowerCase()
      if (person && !currentCoAuthors.some((other) => other.email.toLowerCase() === email)) {
        currentCoAuthors.push(person)
      }
    }
  }

  // Adds a co-author change, replacing an earlier change for the same person
  // (or every earlier removal, for a removal of all co-authors).
  function changeCoAuthor(change: CoAuthorChange) {
    const email = change.person?.email.toLowerCase()
    setCoAuthorChanges((current) => [
      ...current.filter((other) =>
        change.person === null
          ? other.action !== 'remove'
          : other.person === null || other.person.email.toLowerCase() !== email,
      ),
      change,
    ])
  }

  async function changeSignOffMode(mode: SignOffMode) {
    if (mode === 'add' && !me) {
      try {
        const identity = await GetGitIdentity()
        if (!identity.name || !identity.email) {
          setError('Git needs both user.name and user.email set to sign off. Set them with git config.')
          return
        }
        setMe({ name: identity.name, email: identity.email })
      } catch (error) {
        setError(errorText(error))
        return
      }
    }
    setSignOffMode(mode)
  }

  const spread = isSpread && inputsReady && spreadAnswer?.key === spreadKey ? spreadAnswer : null
  const spreadDates = spread && spread.error === null ? spread.dates : null
  const spreadError = isSpread ? (inputError ?? spread?.error ?? null) : null
  const datesChange = isSpread
    ? spreadDates !== null &&
      selected.some((commit) => Date.parse(spreadDates[commit.hash] ?? commit.date) !== Date.parse(commit.date))
    : shiftMinutes !== 0
  // Spread dates are moments, so a commit in another time zone than the
  // range shows them at its own clock time.
  const otherTimeZones =
    isSpread && selected.some((commit) => ![first.offset, last.offset].includes(splitRfc3339(commit.date).offset))

  const previews = datePreviews(selected, isSpread ? 0 : shiftMinutes, isSpread ? spreadDates : null)
  const outOfOrder =
    datesChange &&
    breaksDateOrder(commits, (commit) => {
      if (!selectedSet.has(commit.hash)) {
        return Date.parse(commit.date)
      }
      return spreadDates ? Date.parse(spreadDates[commit.hash]) : Date.parse(commit.date) + shiftMinutes * 60_000
    })
  const fieldErrors = setAuthor ? identityErrors(authorName, authorEmail) : NO_IDENTITY_ERRORS
  const committerErrors =
    committerMode === 'set' ? identityErrors(committerName, committerEmail, 'Committer') : NO_IDENTITY_ERRORS
  const isValid = [fieldErrors, committerErrors].every((errors) => errors.name === null && errors.email === null)
  const changesCommitter = committerMode !== 'keep'
  const hasChanges = datesChange || setAuthor || changesCommitter || trailersChange
  // Nothing can be reviewed while the spread's dates or the trailer preview
  // are loading or invalid: the change sent must be the one shown.
  const spreadPending = isSpread && spreadDates === null
  const canReview =
    hasChanges && isValid && !spreadPending && !trailersPending && !hasPushed && !isSubmitting && activity === null
  const newAuthorText = `${authorName} <${authorEmail}>`
  // The new committer of commit. The list only has names, so a committer
  // copied from an unchanged author is shown by name alone.
  const newCommitterText = (commit: CommitSummary) =>
    committerMode === 'set'
      ? `${committerName} <${committerEmail}>`
      : setAuthor
        ? newAuthorText
        : commit.author

  async function openConfirmDialog() {
    try {
      const [refs, signed] = await Promise.all([GetAffectedRefs(selectedHashes), GetSignedCommits(selectedHashes)])
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
    if (!repoInfo) {
      return
    }
    setIsSubmitting(true)

    try {
      await runGitOperation('Rewriting commit history…', async () => {
        let result
        try {
          result = await EditCommits(app.BulkEditRequest.createFrom({
            hashes: selectedHashes,
            minutes: isSpread ? 0 : shiftMinutes,
            dates: isSpread && datesChange && spreadDates ? spreadDates : {},
            shiftCommitter,
            setAuthor,
            authorName: setAuthor ? authorName : '',
            authorEmail: setAuthor ? authorEmail : '',
            committer: committerMode,
            committerName: committerMode === 'set' ? committerName : '',
            committerEmail: committerMode === 'set' ? committerEmail : '',
            trailers: trailersChange ? pendingTrailers : { add: [], remove: [] },
            moveBranches: moveBranches
              ? affectedRefs.filter((ref) => ref.kind === 'branch').map((ref) => ref.name)
              : [],
            backup,
          }))
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

  // What the confirm dialog says is left alone.
  const unchangedParts = joinWords([!trailersChange && 'messages', !setAuthor && 'authors', !changesCommitter && 'committers'])
  const keptNote = [
    !datesChange
      ? 'Dates are kept.'
      : !shiftCommitter
        ? 'Committer dates are kept.'
        : isSpread
          ? 'Committer dates move by as much as their author dates.'
          : `Committer dates are shifted by ${formatShift(shiftMinutes)} as well.`,
    unchangedParts && `${unchangedParts[0].toUpperCase()}${unchangedParts.slice(1)} are not changed.`,
    trailersChange && 'Messages are kept apart from their trailers.',
  ]
    .filter(Boolean)
    .join(' ')

  // Heading of the confirm dialog's table.
  const changedLabel = joinWords([
    datesChange && (isSpread ? 'Author Dates (spread)' : `Author Dates (${formatShift(shiftMinutes)})`),
    setAuthor && 'Authors',
    changesCommitter && 'Committers',
    trailersChange && 'Trailers',
  ])

  return (
    <>
      <aside className={PANEL_CLASS}>
        <div className={PANEL_BODY_CLASS}>
          <h2 className="text-base font-semibold text-gray-100">Edit Several Commits</h2>
          <p className="mt-1 text-xs text-gray-400">
            Change the dates, set the author and committer, or add and remove co-authors and sign-offs of the{' '}
            {selected.length} selected commits in one rewrite. Each commit keeps its own time zone, and its message apart
            from the trailers.
          </p>

          <div className="mt-5">
            <div className="flex items-center justify-between gap-2">
              <span id="bulk-date-mode" className="text-xs font-medium uppercase tracking-wide text-gray-400">
                Dates
              </span>
              <div role="group" aria-labelledby="bulk-date-mode" className="flex rounded-md border border-gray-700">
                {DATE_MODES.map(({ mode, label }) => (
                  <button
                    key={mode}
                    type="button"
                    aria-pressed={dateMode === mode}
                    disabled={isSubmitting}
                    onClick={() => setDateMode(mode)}
                    className={`px-3 py-1 text-sm transition first:rounded-l-md last:rounded-r-md disabled:cursor-not-allowed ${
                      dateMode === mode ? 'bg-indigo-600 text-white' : 'text-gray-300 hover:bg-gray-800'
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>

            {isSpread ? (
              <div className="mt-2 space-y-3">
                <p className="text-xs text-gray-400">
                  The oldest commit gets the first date and the newest the last date, exactly. The commits in between
                  keep their order.
                </p>
                <div>
                  <span className="text-xs text-gray-400">First (oldest commit)</span>
                  <DateTimeField value={first} onChange={setFirstInput} disabled={isSubmitting} label="First date" />
                </div>
                <div>
                  <span className="text-xs text-gray-400">Last (newest commit)</span>
                  <DateTimeField value={last} onChange={setLastInput} disabled={isSubmitting} label="Last date" />
                </div>
                <div role="radiogroup" aria-label="Spacing" className="space-y-1">
                  {SPACINGS.map((option) => (
                    <label key={option.spacing} className="flex items-center gap-2 text-sm text-gray-300">
                      <input
                        type="radio"
                        name="bulk-spacing"
                        checked={spacing === option.spacing}
                        onChange={() => setSpacing(option.spacing)}
                        disabled={isSubmitting}
                        className="accent-indigo-500 disabled:cursor-not-allowed"
                      />
                      {option.label}
                    </label>
                  ))}
                </div>
                {spacing === 'random' && (
                  <div className="flex flex-wrap items-center gap-2">
                    <label htmlFor="bulk-min-gap" className="text-sm text-gray-300">
                      Minimum gap
                    </label>
                    <input
                      id="bulk-min-gap"
                      type="number"
                      min={0}
                      step={1}
                      value={minGapText}
                      onChange={(e) => setMinGapText(e.target.value)}
                      disabled={isSubmitting}
                      className="w-20 rounded-md border border-gray-700 bg-gray-800 px-2 py-1 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
                    />
                    <span className="text-sm text-gray-400">minutes</span>
                    <button
                      type="button"
                      title="Draw new random dates"
                      disabled={isSubmitting}
                      onClick={() => setSeed(newSeed())}
                      className={`ml-auto ${DATE_BUTTON_CLASS}`}
                    >
                      Re-roll
                    </button>
                  </div>
                )}
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <label className="flex items-center gap-2 text-sm text-gray-300">
                    <input
                      type="checkbox"
                      checked={onlyOfficeHours}
                      onChange={(e) => setOnlyOfficeHours(e.target.checked)}
                      disabled={isSubmitting}
                      className="accent-indigo-500 disabled:cursor-not-allowed"
                    />
                    Only office hours
                  </label>
                  <span className="text-xs text-gray-400">({formatOfficeHours(officeHours)})</span>
                  <button
                    type="button"
                    title="Change the office hours in the settings"
                    disabled={isSubmitting}
                    onClick={() => setSettingsOpen(true)}
                    className={DATE_BUTTON_CLASS}
                  >
                    Change
                  </button>
                </div>
                <p className="text-xs text-gray-400">
                  {SPACING_DESCRIPTIONS[spacing]}
                  {onlyOfficeHours
                    ? ' Time outside office hours is skipped, so every gap counts office time only. The first and last date must be within office hours, which are read in the first date’s time zone.'
                    : ' Any time of day counts, so over several days a commit can land at night.'}
                </p>
                {spreadError && <p className="text-xs text-red-300">{spreadError}</p>}
                {spread?.fellBack && (
                  <p className="text-xs text-yellow-300">
                    The current dates are all the same or out of order, so their gaps cannot be kept. The commits are
                    spaced evenly instead.
                  </p>
                )}
                {otherTimeZones && (
                  <p className="text-xs text-gray-400">
                    Commits in another time zone get the same moments, shown at their own clock time.
                  </p>
                )}
              </div>
            ) : (
              <>
                <div className="mt-2 flex items-baseline justify-between">
                  <span className="text-xs text-gray-400">Move every commit by the same amount</span>
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
              </>
            )}
            <label className="mt-2 flex items-center gap-2 text-sm text-gray-300">
              <input
                type="checkbox"
                checked={shiftCommitter}
                onChange={(e) => setShiftCommitter(e.target.checked)}
                disabled={isSubmitting}
                className="accent-indigo-500 disabled:cursor-not-allowed"
              />
              Also move committer dates
            </label>
          </div>

          <div className="mt-5">
            <div className="flex items-center justify-between gap-2">
              <label className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-gray-400">
                <input
                  type="checkbox"
                  checked={setAuthor}
                  onChange={(e) => setSetAuthor(e.target.checked)}
                  disabled={isSubmitting}
                  className="accent-indigo-500 disabled:cursor-not-allowed"
                />
                Set Author
              </label>
              <UseMyIdentityButton
                disabled={isSubmitting}
                onIdentity={(name, email) => {
                  setAuthorName(name)
                  setAuthorEmail(email)
                  setSetAuthor(true)
                }}
              />
            </div>
            <input
              type="text"
              value={authorName}
              onChange={(e) => setAuthorName(e.target.value)}
              disabled={isSubmitting || !setAuthor}
              aria-label="Author name"
              className={INPUT_CLASS}
              placeholder="Author name"
            />
            {fieldErrors.name && <p className="mt-1 text-xs text-red-300">{fieldErrors.name}</p>}
            <input
              type="email"
              value={authorEmail}
              onChange={(e) => setAuthorEmail(e.target.value)}
              disabled={isSubmitting || !setAuthor}
              aria-label="Author email"
              className={INPUT_CLASS}
              placeholder="author@example.com"
            />
            {fieldErrors.email && <p className="mt-1 text-xs text-red-300">{fieldErrors.email}</p>}
          </div>

          <div className="mt-5">
            <CommitterFields
              mode={committerMode}
              name={committerName}
              email={committerEmail}
              errors={committerErrors}
              disabled={isSubmitting}
              authorOptionLabel="Same as each commit's author"
              onModeChange={setCommitterMode}
              onIdentityChange={(name, email) => {
                setCommitterName(name)
                setCommitterEmail(email)
              }}
            />
          </div>

          <div className="mt-5">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium uppercase tracking-wide text-gray-400">Trailers</span>
              <div className="flex gap-1">
                <CoAuthorPicker
                  label="Add co-author"
                  title="Add a Co-authored-by line to every selected commit"
                  disabled={isSubmitting}
                  onPick={(person) => changeCoAuthor({ action: 'add', person })}
                />
                <CoAuthorPicker
                  label="Remove co-author"
                  title={
                    currentCoAuthors.length > 0
                      ? 'Remove a Co-authored-by line from the selected commits that have it'
                      : 'None of the selected commits has a co-author'
                  }
                  disabled={isSubmitting || currentCoAuthors.length === 0}
                  people={currentCoAuthors}
                  allLabel="All co-authors"
                  onPickAll={() => changeCoAuthor({ action: 'remove', person: null })}
                  onPick={(person) => changeCoAuthor({ action: 'remove', person })}
                />
              </div>
            </div>
            {coAuthorChanges.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-1.5">
                {coAuthorChanges.map((change, index) => {
                  const text = change.person ? formatPerson(change.person) : 'all co-authors'
                  return (
                    <li
                      key={`${change.action} ${text}`}
                      className={`flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-xs ${
                        change.action === 'add'
                          ? 'border-indigo-800 bg-indigo-950/40 text-indigo-200'
                          : 'border-red-900/70 bg-red-950/30 text-red-200'
                      }`}
                    >
                      <span className="truncate" title={text}>
                        {change.action === 'add' ? '+ ' : '− '}
                        {text}
                      </span>
                      <button
                        type="button"
                        aria-label={`Undo: ${change.action} ${text}`}
                        title="Undo this change"
                        disabled={isSubmitting}
                        onClick={() => setCoAuthorChanges((current) => current.filter((_, i) => i !== index))}
                        className="text-gray-400 hover:text-gray-100 disabled:cursor-not-allowed"
                      >
                        ×
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
            <label className="mt-2 flex items-center gap-2 text-sm text-gray-300">
              <span className="shrink-0">Sign-offs</span>
              <select
                value={signOffMode}
                onChange={(e) => void changeSignOffMode(e.target.value as SignOffMode)}
                disabled={isSubmitting}
                className="w-full rounded-md border border-gray-700 bg-gray-800 px-2 py-1 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <option value="keep">Keep</option>
                <option value="add">Add mine{me ? ` (${formatPerson(me)})` : ''}</option>
                <option value="remove">Remove all</option>
              </select>
            </label>
            {trailerError && <p className="mt-1 text-xs text-red-300">{trailerError}</p>}
            {hasTrailerChange && trailerPreviews && !trailersChange && (
              <p className="mt-1 text-xs text-gray-400">
                The selected commits already have these trailers, so their messages stay as they are.
              </p>
            )}
          </div>

          {hasPushed && (
            <div className="mt-4 rounded-lg border border-yellow-900/60 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-300">
              Some selected commits have been pushed and cannot be edited. Ctrl-click them to remove them from the
              selection.
            </div>
          )}
          {outOfOrder && !hasPushed && (
            <div className="mt-4 rounded-lg border border-yellow-900/60 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-300">
              After this change, some commits will be dated earlier than the commit below them in the list. Git allows
              this, but tools that sort by date will show them out of order.
            </div>
          )}

          <ul className="mt-4 space-y-2">
            {previews.map(({ commit, before, after, gap }) => (
              <li key={commit.hash} className="rounded-lg border border-gray-800 bg-gray-900 px-3 py-2 text-xs">
                <div className="flex gap-2">
                  <span className="font-mono text-gray-500">{commit.shortHash}</span>
                  <span className="truncate text-gray-200" title={commit.message}>
                    {commit.message}
                  </span>
                </div>
                <div className="mt-1 text-gray-400">
                  {before} · {commit.author}
                </div>
                {datesChange && (
                  <div className="text-indigo-300">
                    → {after}
                    {gap && <GapNote gap={gap} />}
                  </div>
                )}
                {setAuthor && <div className="break-words text-indigo-300">→ {newAuthorText}</div>}
                {changesCommitter && (
                  <div className="break-words text-indigo-300">→ committed by {newCommitterText(commit)}</div>
                )}
                {trailersChange && <TrailerLines preview={trailerPreviews?.[commit.hash]} />}
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
        title={`Update ${selected.length} Commits`}
        signedCommits={signedCommits}
        affectedRefs={affectedRefs}
        moveBranches={moveBranches}
        onMoveBranchesChange={setMoveBranches}
        backup={backup}
        onBackupChange={setBackup}
        onCancel={() => setShowConfirmDialog(false)}
        onConfirm={handleConfirmApply}
      >
        <div className="rounded-lg border border-gray-800 bg-gray-900/70 p-3">
          <div className="text-xs font-medium uppercase tracking-wide text-gray-400">
            {changedLabel}
          </div>
          <table className="mt-2 w-full text-left text-sm">
            <thead className="text-[11px] uppercase tracking-wide text-gray-500">
              <tr>
                <th className="pb-1 pr-3 font-normal">Commit</th>
                <th className="pb-1 pr-3 font-normal">Current</th>
                <th className="pb-1 font-normal">New</th>
              </tr>
            </thead>
            <tbody>
              {previews.map(({ commit, before, after, gap }) => (
                <tr key={commit.hash} className="border-t border-gray-800 align-top">
                  <td className="py-1.5 pr-3">
                    <span className="font-mono text-xs text-gray-500">{commit.shortHash}</span>{' '}
                    <span className="text-gray-300">{commit.message}</span>
                  </td>
                  <td className="py-1.5 pr-3 text-gray-400">
                    {datesChange && <div>{before}</div>}
                    {setAuthor && <div>{commit.author}</div>}
                    {changesCommitter && <div>Committer: {commit.committer}</div>}
                  </td>
                  <td className="break-words py-1.5 text-indigo-200">
                    {datesChange && (
                      <div>
                        {after}
                        {gap && <GapNote gap={gap} />}
                      </div>
                    )}
                    {setAuthor && <div>{newAuthorText}</div>}
                    {changesCommitter && <div>Committer: {newCommitterText(commit)}</div>}
                    {trailersChange && <TrailerLines preview={trailerPreviews?.[commit.hash]} />}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="mt-2 text-xs text-gray-400">{keptNote}</p>
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
