import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { DEFAULT_STALE_FETCH_DAYS, MAX_STALE_FETCH_DAYS } from '../lastFetch'
import { DEFAULT_MESSAGE_GUIDES, MAX_GUIDE_COLUMN, type MessageGuides } from '../messageGuides'
import { DEFAULT_OFFICE_HOURS, formatOfficeHours, officeHoursError, WEEKDAYS, type OfficeHours } from '../officeHours'
import { DEFAULT_BACKUP_SETTINGS, MAX_AUTO_BACKUPS_KEPT, useRepoStore } from '../store/repoStore'
import { themePreferences, type ThemePreference } from '../theme'

const themeLabels: Record<ThemePreference, string> = { system: 'System', light: 'Light', dark: 'Dark' }

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h3 className="text-xs font-medium uppercase tracking-wide text-gray-400">{title}</h3>
      {children}
    </section>
  )
}

// The number typed in a NumberField, or null when it is not a whole number
// from 0 to max.
function parseWholeNumber(text: string, max: number, min = 0): number | null {
  if (!/^\d+$/.test(text.trim())) {
    return null
  }
  const value = Number(text)
  return value >= min && value <= max ? value : null
}

interface NumberFieldProps {
  id: string
  label: string
  description: string
  // The smallest value allowed; 0 when left out.
  min?: number
  max: number
  text: string
  onChange: (text: string) => void
}

function NumberField({ id, label, description, min = 0, max, text, onChange }: NumberFieldProps) {
  const isValid = parseWholeNumber(text, max, min) !== null
  return (
    <div>
      <div className="flex items-center justify-between gap-3">
        <label htmlFor={id} className="text-sm text-gray-200">
          {label}
        </label>
        <input
          id={id}
          type="number"
          min={min}
          max={max}
          value={text}
          onChange={(event) => onChange(event.target.value)}
          aria-invalid={!isValid}
          className={`w-20 rounded-md border bg-gray-800 px-2 py-1 text-right font-mono text-sm text-gray-100 outline-none transition focus:border-indigo-500 ${isValid ? 'border-gray-700' : 'border-red-500'}`}
        />
      </div>
      <p className="mt-1 text-xs text-gray-500">{description}</p>
      {!isValid && (
        <p className="mt-1 text-xs text-red-300">Enter a whole number from {min} to {max}.</p>
      )}
    </div>
  )
}

// What the automatic terminal choice does on this system (terminalLaunches in
// app/terminal.go), and example commands for the terminal field.
function terminalHelp(): { automatic: string; examples: string[] } {
  const platform = navigator.userAgent
  if (platform.includes('Windows')) {
    return {
      automatic: 'Windows Terminal, or cmd when it is not installed',
      examples: ['wt.exe -d {dir} pwsh', '"C:\\Program Files\\Git\\git-bash.exe" --cd={dir}'],
    }
  }
  if (platform.includes('Mac')) {
    return { automatic: 'Terminal', examples: ['open -a iTerm {dir}', 'open -a Ghostty {dir}'] }
  }
  return {
    automatic: '$TERMINAL, or the first common terminal found',
    examples: ['kitty --directory {dir}', 'wezterm start --cwd {dir}'],
  }
}

function TerminalField() {
  const terminalCommand = useRepoStore((s) => s.terminalCommand)
  const setTerminalCommand = useRepoStore((s) => s.setTerminalCommand)
  const [text, setText] = useState(terminalCommand)
  const [error, setError] = useState<string | null>(null)
  const help = terminalHelp()

  function change(value: string) {
    setText(value)
    setTerminalCommand(value).then(setError)
  }

  return (
    <div>
      <label htmlFor="settings-terminal" className="text-sm text-gray-200">
        Terminal command
      </label>
      <input
        id="settings-terminal"
        type="text"
        value={text}
        onChange={(event) => change(event.target.value)}
        placeholder={`Automatic: ${help.automatic}`}
        spellCheck={false}
        aria-invalid={error !== null}
        className={`mt-1 w-full rounded-md border bg-gray-800 px-2 py-1 font-mono text-sm text-gray-100 outline-none transition focus:border-indigo-500 ${error ? 'border-red-500' : 'border-gray-700'}`}
      />
      {error && <p className="mt-1 text-xs text-red-300">{error}</p>}
      <p className="mt-1 text-xs text-gray-500">
        What the <span className="font-mono">&gt;_</span> button runs. <span className="font-mono">{'{dir}'}</span> stands
        for the repository folder; quote paths with spaces. Leave empty to use {help.automatic}. For example:
      </p>
      <ul className="mt-1 space-y-0.5 font-mono text-xs text-gray-400">
        {help.examples.map((example) => (
          <li key={example}>{example}</li>
        ))}
      </ul>
    </div>
  )
}

const TIME_INPUT_CLASS =
  'rounded-md border border-gray-700 bg-gray-800 px-2 py-1 font-mono text-sm text-gray-100 outline-none transition focus:border-indigo-500'

// The working hours used by "Only office hours" when spreading commits. Like
// the guide fields, valid hours are saved at once and invalid ones stay here
// until they are fixed, or are dropped on close.
function OfficeHoursField() {
  const officeHours = useRepoStore((s) => s.officeHours)
  const setOfficeHours = useRepoStore((s) => s.setOfficeHours)
  const [draft, setDraft] = useState<OfficeHours>(officeHours)
  const error = officeHoursError(draft)

  function change(hours: OfficeHours) {
    setDraft(hours)
    if (officeHoursError(hours) === null) {
      setOfficeHours(hours)
    }
  }

  function toggleDay(day: number) {
    const days = draft.days.includes(day) ? draft.days.filter((d) => d !== day) : [...draft.days, day]
    change({ ...draft, days: days.sort((left, right) => left - right) })
  }

  const isDefault = formatOfficeHours(draft) === formatOfficeHours(DEFAULT_OFFICE_HOURS)

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        <label htmlFor="settings-office-start" className="text-sm text-gray-200">
          From
        </label>
        <input
          id="settings-office-start"
          type="time"
          value={draft.start}
          onChange={(event) => change({ ...draft, start: event.target.value })}
          className={TIME_INPUT_CLASS}
        />
        <label htmlFor="settings-office-end" className="text-sm text-gray-200">
          to
        </label>
        <input
          id="settings-office-end"
          type="time"
          value={draft.end}
          onChange={(event) => change({ ...draft, end: event.target.value })}
          className={TIME_INPUT_CLASS}
        />
      </div>
      <div role="group" aria-label="Working days" className="mt-2 flex flex-wrap gap-1">
        {WEEKDAYS.map(({ day, short }) => {
          const isWorking = draft.days.includes(day)
          return (
            <button
              key={day}
              type="button"
              aria-pressed={isWorking}
              onClick={() => toggleDay(day)}
              className={`rounded-md border px-2 py-1 text-xs transition ${
                isWorking
                  ? 'border-indigo-500 bg-indigo-600 text-white'
                  : 'border-gray-700 text-gray-400 hover:bg-gray-800'
              }`}
            >
              {short}
            </button>
          )
        })}
      </div>
      {error && <p className="mt-1 text-xs text-red-300">{error}</p>}
      <p className="mt-1 text-xs text-gray-500">
        Used by <span className="text-gray-300">Only office hours</span> when spreading the dates of several
        commits: commits are only placed within these hours, in the time zone of the first date.
      </p>
      <div className="mt-2 flex justify-end">
        <button
          type="button"
          onClick={() => change(DEFAULT_OFFICE_HOURS)}
          disabled={isDefault}
          className="rounded-md border border-gray-700 px-3 py-1 text-xs text-gray-300 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Restore default ({formatOfficeHours(DEFAULT_OFFICE_HOURS)})
        </button>
      </div>
    </div>
  )
}

// How many days after the last fetch the status bar and commit list warn.
// Like the guide fields, a valid value is saved at once.
function StaleFetchField() {
  const staleFetchDays = useRepoStore((s) => s.staleFetchDays)
  const setStaleFetchDays = useRepoStore((s) => s.setStaleFetchDays)
  const [text, setText] = useState(String(staleFetchDays))

  function change(value: string) {
    setText(value)
    const days = parseWholeNumber(value, MAX_STALE_FETCH_DAYS)
    if (days !== null && days !== staleFetchDays) {
      setStaleFetchDays(days)
    }
  }

  return (
    <NumberField
      id="settings-stale-fetch"
      label="Warn after days without a fetch"
      description={`Pushed and unpushed commits are worked out from your last git fetch, which GitGo never runs itself. When it is older than this, the status bar and commit list warn. 0 turns the warning off. Default ${DEFAULT_STALE_FETCH_DAYS}.`}
      max={MAX_STALE_FETCH_DAYS}
      text={text}
      onChange={change}
    />
  )
}

// Whether "Back up first" starts ticked in the confirm dialog, and how many
// automatic backups to keep per branch. Like the guide fields, a valid count
// is saved at once.
function BackupFields() {
  const backupSettings = useRepoStore((s) => s.backupSettings)
  const setBackupSettings = useRepoStore((s) => s.setBackupSettings)
  const [text, setText] = useState(String(backupSettings.autoBackupsKept))

  function changeKept(value: string) {
    setText(value)
    const count = parseWholeNumber(value, MAX_AUTO_BACKUPS_KEPT, 1)
    if (count !== null && count !== backupSettings.autoBackupsKept) {
      setBackupSettings({ ...backupSettings, autoBackupsKept: count })
    }
  }

  return (
    <div className="space-y-4">
      <label className="flex items-start gap-2 text-sm text-gray-200">
        <input
          type="checkbox"
          checked={backupSettings.backupBeforeApply}
          onChange={(event) => setBackupSettings({ ...backupSettings, backupBeforeApply: event.target.checked })}
          className="mt-1 accent-indigo-500"
        />
        <span>
          Back up before applying
          <span className="mt-1 block text-xs text-gray-500">
            Whether “Back up first” starts ticked when you review an edit. The backup saves every branch the edit
            moves, so a restore brings them all back.
          </span>
        </span>
      </label>
      <NumberField
        id="settings-backups-kept"
        label="Automatic backups kept per branch"
        description={`When GitGo makes an automatic backup, older ones of the same branch beyond this number are deleted. Backups you make with Back up are kept until you delete them. Default ${DEFAULT_BACKUP_SETTINGS.autoBackupsKept}.`}
        min={1}
        max={MAX_AUTO_BACKUPS_KEPT}
        text={text}
        onChange={changeKept}
      />
    </div>
  )
}

// The dialog's content, mounted only while it is open so the guide fields
// start from the saved values each time.
function SettingsContent({ onClose }: { onClose: () => void }) {
  const theme = useRepoStore((s) => s.theme)
  const setTheme = useRepoStore((s) => s.setTheme)
  const messageGuides = useRepoStore((s) => s.messageGuides)
  const setMessageGuides = useRepoStore((s) => s.setMessageGuides)
  const closeRef = useRef<HTMLButtonElement>(null)
  // What is typed in the guide fields. A valid value is saved at once; an
  // invalid one stays here until it is fixed, or is dropped on close.
  const [subjectText, setSubjectText] = useState(String(messageGuides.subject))
  const [bodyText, setBodyText] = useState(String(messageGuides.body))

  // Like HelpDialog, the dialog owns Escape while open and blocks Ctrl+Z
  // outside its fields so the app-wide shortcuts cannot act behind it. Focus
  // moves into the dialog and goes back to where it was on close.
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeRef.current?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      const isUndoShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z'
      const isSettingsShortcut = (event.ctrlKey || event.metaKey) && event.key === ','

      if (event.key === 'Escape' || isSettingsShortcut) {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      } else if (isUndoShortcut && !(event.target instanceof HTMLInputElement)) {
        event.preventDefault()
        event.stopPropagation()
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      previousFocus?.focus()
    }
  }, [onClose])

  function changeGuide(field: keyof MessageGuides, text: string) {
    if (field === 'subject') {
      setSubjectText(text)
    } else {
      setBodyText(text)
    }
    const column = parseWholeNumber(text, MAX_GUIDE_COLUMN)
    if (column !== null && column !== messageGuides[field]) {
      setMessageGuides({ ...messageGuides, [field]: column })
    }
  }

  function restoreDefaults() {
    setSubjectText(String(DEFAULT_MESSAGE_GUIDES.subject))
    setBodyText(String(DEFAULT_MESSAGE_GUIDES.body))
    setMessageGuides(DEFAULT_MESSAGE_GUIDES)
  }

  const guidesAreDefault =
    subjectText === String(DEFAULT_MESSAGE_GUIDES.subject) && bodyText === String(DEFAULT_MESSAGE_GUIDES.body)

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
      onClick={(event) => event.target === event.currentTarget && onClose()}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-title"
        className="flex max-h-[90vh] w-full max-w-md flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-800 px-5 py-4">
          <div>
            <h2 id="settings-title" className="text-lg font-semibold text-gray-100">
              Settings
            </h2>
            <p className="mt-1 text-sm text-gray-400">Changes apply at once and are saved for next time.</p>
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            title="Close (Escape)"
            aria-label="Close settings"
            className="rounded-md px-2 py-1 text-gray-400 transition hover:bg-gray-800 hover:text-gray-200"
          >
            ×
          </button>
        </div>

        <div className="space-y-6 overflow-y-auto p-5">
          <Section title="Appearance">
            <div className="flex items-center justify-between gap-3">
              <span id="settings-theme" className="text-sm text-gray-200">
                Theme
              </span>
              <div role="group" aria-labelledby="settings-theme" className="flex rounded-md border border-gray-700">
                {themePreferences.map((preference) => (
                  <button
                    key={preference}
                    type="button"
                    aria-pressed={theme === preference}
                    onClick={() => setTheme(preference)}
                    className={`px-3 py-1 text-sm transition first:rounded-l-md last:rounded-r-md ${
                      theme === preference
                        ? 'bg-indigo-600 text-white'
                        : 'text-gray-300 hover:bg-gray-800'
                    }`}
                  >
                    {themeLabels[preference]}
                  </button>
                ))}
              </div>
            </div>
            <p className="text-xs text-gray-500">System follows your operating system&apos;s light or dark setting.</p>
          </Section>

          <Section title="Commit message guides">
            <NumberField
              id="settings-subject-guide"
              label="Subject line length"
              description="Draws a ruler over the first line of the message and warns about longer subjects. 0 turns it off."
              max={MAX_GUIDE_COLUMN}
              text={subjectText}
              onChange={(text) => changeGuide('subject', text)}
            />
            <NumberField
              id="settings-body-guide"
              label="Body line length"
              description="Warns about body lines longer than this. 0 turns it off."
              max={MAX_GUIDE_COLUMN}
              text={bodyText}
              onChange={(text) => changeGuide('body', text)}
            />
            <div className="flex justify-end">
              <button
                type="button"
                onClick={restoreDefaults}
                disabled={guidesAreDefault}
                className="rounded-md border border-gray-700 px-3 py-1 text-xs text-gray-300 transition hover:border-gray-600 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                Restore defaults ({DEFAULT_MESSAGE_GUIDES.subject} / {DEFAULT_MESSAGE_GUIDES.body})
              </button>
            </div>
          </Section>

          <Section title="Office hours">
            <OfficeHoursField />
          </Section>

          <Section title="Backups">
            <BackupFields />
          </Section>

          <Section title="Remote">
            <StaleFetchField />
          </Section>

          <Section title="Terminal">
            <TerminalField />
          </Section>
        </div>
      </div>
    </div>
  )
}

// SettingsDialog holds the user's preferences: the colour theme, the
// commit message guide columns, the office hours and the terminal command.
// Changes apply and are saved at once, like the header's theme button.
export default function SettingsDialog() {
  const isOpen = useRepoStore((s) => s.isSettingsOpen)
  const setSettingsOpen = useRepoStore((s) => s.setSettingsOpen)
  const close = useCallback(() => setSettingsOpen(false), [setSettingsOpen])

  return isOpen ? <SettingsContent onClose={close} /> : null
}
