import { create } from 'zustand'
import {
  CanUndo,
  CloseRepository,
  GetCommitLog,
  GetSettings,
  OpenRepository,
  RefreshLog,
  ReloadRepository,
  SetMessageGuides,
  SetOfficeHours,
  SetRecentRepos,
  SetBackupSettings,
  SetStaleFetchDays,
  SetTerminalCommand,
  SetTheme,
  UndoLastOperation,
} from '../../wailsjs/go/app/App'
import { errorText, friendlyError } from '../errors'
import { applyTheme, isThemePreference, type ThemePreference } from '../theme'
import { DEFAULT_MESSAGE_GUIDES, type MessageGuides } from '../messageGuides'
import { DEFAULT_OFFICE_HOURS, type OfficeHours } from '../officeHours'
import { DEFAULT_STALE_FETCH_DAYS } from '../lastFetch'

// Where earlier versions kept the recent repositories, in the WebView's
// localStorage. loadSettings moves the list to the backend's settings file.
const legacyRecentReposStorageKey = 'gitgo.recentRepos'
const maxRecentRepos = 10

// Activity label used while reloadRepository runs; the header's reload button
// compares against it to show its own spinner.
export const reloadActivityLabel = 'Reloading repository…'

function loadLegacyRecentRepos(): string[] {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(legacyRecentReposStorageKey) ?? '[]')
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed.filter((path): path is string => typeof path === 'string').slice(0, maxRecentRepos)
  } catch {
    return []
  }
}

function removeLegacyRecentRepos() {
  try {
    window.localStorage.removeItem(legacyRecentReposStorageKey)
  } catch {
    // Nothing to clean up without storage.
  }
}

// loadSettings reads the saved settings into the store. main.tsx awaits it
// before the first render so the start screen never flashes an empty recent
// list. A list left in localStorage by an earlier version is moved to the
// settings file, unless the settings file already has one.
export async function loadSettings(): Promise<void> {
  const settings = await GetSettings()
  let recentRepos = settings.recentRepos
  const legacy = loadLegacyRecentRepos()
  if (legacy.length > 0 && recentRepos.length === 0) {
    await SetRecentRepos(legacy)
    recentRepos = legacy
  }
  removeLegacyRecentRepos()
  const theme = isThemePreference(settings.theme) ? settings.theme : 'system'
  applyTheme(theme)
  const messageGuides = { subject: settings.subjectGuide, body: settings.bodyGuide }
  const terminalCommand = settings.terminalCommand
  const officeHours = settings.officeHours
  const staleFetchDays = settings.staleFetchDays
  const backupSettings = { backupBeforeApply: settings.backupBeforeApply, autoBackupsKept: settings.autoBackupsKept }
  useRepoStore.setState({
    recentRepos,
    theme,
    messageGuides,
    terminalCommand,
    officeHours,
    staleFetchDays,
    backupSettings,
  })
}

// The last SetTerminalCommand call; see setTerminalCommand.
let terminalSave: Promise<unknown> = Promise.resolve()

function persistRecentRepos(paths: string[]) {
  // The list in the store is already up to date, so a failed save only loses
  // it for the next run.
  SetRecentRepos(paths).catch((error) => console.error('Saving recent repositories failed:', error))
}

function looksLikeMissingPathError(errorText: string): boolean {
  return /does not exist|cannot find|no such file|cannot resolve path/i.test(errorText)
}

function withRecentRepo(paths: string[], path: string): string[] {
  const deduped = [path, ...paths.filter((candidate) => candidate !== path)]
  return deduped.slice(0, maxRecentRepos)
}

export interface BackupSettings {
  backupBeforeApply: boolean
  autoBackupsKept: number
}

// Same as the Go defaults, used until the settings have loaded.
export const DEFAULT_BACKUP_SETTINGS: BackupSettings = { backupBeforeApply: true, autoBackupsKept: 20 }
export const MAX_AUTO_BACKUPS_KEPT = 1000
// The longest backup name, as in git/backup.go.
export const MAX_BACKUP_NAME_LENGTH = 100

export interface RepoInfo {
  path: string
  branch: string
  // False when viewing a branch other than the checked-out one.
  isCheckedOut: boolean
  hasRemote: boolean
  hasUpstream: boolean
  // When the repository was last fetched, as RFC 3339, or empty when no
  // fetch is recorded (see lastFetch.ts).
  lastFetch: string
}

export interface CommitSummary {
  hash: string
  shortHash: string
  message: string
  author: string
  committer: string
  date: string
  isUnpushed: boolean
}

const NO_SELECTION = { selectedHash: null, selectedHashes: [] as string[], selectionAnchor: null }

// The unpushed commits among hashes, in commit list order.
function unpushedInOrder(commits: CommitSummary[], hashes: string[]): string[] {
  const wanted = new Set(hashes)
  return commits.filter((commit) => commit.isUnpushed && wanted.has(commit.hash)).map((commit) => commit.hash)
}

interface RepoStore {
  repoInfo: RepoInfo | null
  commits: CommitSummary[]
  recentRepos: string[]
  // Colour theme the user picked (header theme button), saved in the settings file.
  theme: ThemePreference
  // Commit message guide columns (0 turns a guide off), saved in the
  // settings file.
  messageGuides: MessageGuides
  // Command the header's terminal button runs, with {dir} for the repository
  // folder; empty picks a terminal automatically. Saved in the settings file.
  terminalCommand: string
  // Working hours for spreading commits with "Only office hours". Saved in
  // the settings file.
  officeHours: OfficeHours
  // Days after the last fetch before the status bar and commit list warn
  // that the pushed / unpushed split may be out of date; 0 never warns.
  // Saved in the settings file.
  staleFetchDays: number
  // Whether the confirm dialog's "Back up first" checkbox starts ticked, and
  // how many automatic backups the backend keeps per branch. Saved in the
  // settings file.
  backupSettings: BackupSettings
  // Hash of the commit currently selected in CommitList; null when nothing is
  // selected. EditPanel reads this to know which commit to load. With several
  // commits selected it is the one last clicked or moved to.
  selectedHash: string | null
  // Every selected commit, in list order: [selectedHash] for a single
  // selection, or several unpushed commits selected with Ctrl/Shift-click or
  // Shift+arrow keys, which BulkEditPanel edits together.
  selectedHashes: string[]
  // Commit a Shift-click or Shift+arrow range starts from.
  selectionAnchor: string | null
  // True after a successful rewrite that the backend can still undo. Any
  // setRepo call (open, refresh, undo) resets it; EditPanel sets it again
  // after a rewrite.
  canUndo: boolean
  // Label of the git operation currently running (e.g. "Switching branch…"),
  // or null when idle. StatusBar shows it with a spinner, and controls that
  // start another git operation are disabled while it is set.
  activity: string | null
  // Set by the Enter shortcut on a commit row. EditPanel consumes it once the
  // commit has loaded and focuses the first field if the commit is editable.
  pendingEditFocus: boolean
  // Whether HelpDialog is open (header button or F1).
  isHelpOpen: boolean
  // Whether SettingsDialog is open (header button or Ctrl+,).
  isSettingsOpen: boolean
  status: string
  // User-facing error message (see friendlyError), or null.
  error: string | null
  // Raw error text behind error, shown as a tooltip; null when error is
  // already the raw text or there is no error.
  errorDetail: string | null
  setRepo: (info: RepoInfo, commits: CommitSummary[]) => void
  // Opens the repository at path (start screen or the header's repository
  // dropdown). On failure the open repository, if any, stays open, and a
  // recent entry whose folder no longer exists is removed.
  openRepository: (path: string) => Promise<void>
  removeRecentRepo: (path: string) => void
  setTheme: (theme: ThemePreference) => void
  setMessageGuides: (messageGuides: MessageGuides) => void
  setOfficeHours: (officeHours: OfficeHours) => void
  setStaleFetchDays: (staleFetchDays: number) => void
  setBackupSettings: (backupSettings: BackupSettings) => void
  // Saves the terminal command. Resolves to an error message when the backend
  // rejects it (for example an unclosed quote), or null.
  setTerminalCommand: (command: string) => Promise<string | null>
  selectCommit: (hash: string | null) => void
  // Ctrl/Cmd-click: add or remove an unpushed commit from the selection.
  toggleCommitSelection: (hash: string) => void
  // Shift-click / Shift+arrow: select the unpushed commits from the anchor to hash.
  extendSelection: (hash: string) => void
  // Button above CommitList / Ctrl+A: select every unpushed commit. Returns
  // the commit that ends up primary, or null when there is nothing to select.
  selectAllUnpushed: () => string | null
  requestEditFocus: () => void
  consumeEditFocus: () => void
  setHelpOpen: (isHelpOpen: boolean) => void
  setSettingsOpen: (isSettingsOpen: boolean) => void
  setCanUndo: (canUndo: boolean) => void
  runGitOperation: <T>(label: string, operation: () => Promise<T>) => Promise<T | undefined>
  undoLastOperation: () => Promise<void>
  reloadRepository: () => Promise<void>
  setStatus: (message: string) => void
  // Accepts raw error text; stores a friendly message plus the raw detail.
  setError: (error: string | null) => void
  // Closes the repository and returns to RepoSelector. Git history is not
  // touched, but the undo record is dropped.
  closeRepository: () => void
}

export const useRepoStore = create<RepoStore>((set, get) => ({
  repoInfo: null,
  commits: [],
  recentRepos: [],
  theme: 'system',
  messageGuides: DEFAULT_MESSAGE_GUIDES,
  terminalCommand: '',
  officeHours: DEFAULT_OFFICE_HOURS,
  staleFetchDays: DEFAULT_STALE_FETCH_DAYS,
  backupSettings: DEFAULT_BACKUP_SETTINGS,
  selectedHash: null,
  selectedHashes: [],
  selectionAnchor: null,
  canUndo: false,
  activity: null,
  pendingEditFocus: false,
  isHelpOpen: false,
  isSettingsOpen: false,
  status: '',
  error: null,
  errorDetail: null,

  // Opening a new repo clears any previous selection so EditPanel doesn't
  // show stale data from the prior repository.
  setRepo: (info, commits) =>
    set((state) => {
      const recentRepos = withRecentRepo(state.recentRepos, info.path)
      persistRecentRepos(recentRepos)

      return {
        repoInfo: info,
        commits,
        recentRepos,
        ...NO_SELECTION,
        canUndo: false,
        error: null,
        errorDetail: null,
        status: `Opened: ${info.path}`,
      }
    }),

  openRepository: async (path) => {
    const { runGitOperation, setRepo, setError, removeRecentRepo, closeRepository } = get()
    setError(null)
    // Once OpenRepository succeeds the backend has replaced the previous
    // repository, so a later failure cannot go back to showing that one.
    let isBackendSwitched = false
    try {
      await runGitOperation('Opening repository…', async () => {
        const info = await OpenRepository(path)
        isBackendSwitched = true
        const commits = await GetCommitLog()
        setRepo(info, commits)
      })
    } catch (error) {
      const text = String(error)
      if (isBackendSwitched) {
        closeRepository()
      }
      setError(text)
      if (looksLikeMissingPathError(text)) {
        removeRecentRepo(path)
      }
    }
  },

  removeRecentRepo: (path) =>
    set((state) => {
      const recentRepos = state.recentRepos.filter((candidate) => candidate !== path)
      persistRecentRepos(recentRepos)
      return { recentRepos }
    }),

  setTheme: (theme) => {
    applyTheme(theme)
    set({ theme })
    // The theme is already showing, so a failed save only loses it for the next run.
    SetTheme(theme).catch((error) => console.error('Saving the theme failed:', error))
  },

  setMessageGuides: (messageGuides) => {
    set({ messageGuides })
    // Like the theme, the guides already apply, so a failed save only loses
    // them for the next run.
    SetMessageGuides(messageGuides.subject, messageGuides.body).catch((error) =>
      console.error('Saving the message guides failed:', error),
    )
  },

  setOfficeHours: (officeHours) => {
    set({ officeHours })
    // Like the guides, the hours already apply, so a failed save only loses
    // them for the next run.
    SetOfficeHours(officeHours).catch((error) => console.error('Saving the office hours failed:', error))
  },

  setStaleFetchDays: (staleFetchDays) => {
    set({ staleFetchDays })
    // Like the guides, the setting already applies, so a failed save only
    // loses it for the next run.
    SetStaleFetchDays(staleFetchDays).catch((error) => console.error('Saving the stale fetch days failed:', error))
  },

  setBackupSettings: (backupSettings) => {
    set({ backupSettings })
    // Like the guides, the settings already apply, so a failed save only
    // loses them for the next run.
    SetBackupSettings(backupSettings.backupBeforeApply, backupSettings.autoBackupsKept).catch((error) =>
      console.error('Saving the backup settings failed:', error),
    )
  },

  setTerminalCommand: (command) => {
    // Saves run one after another, so the last command typed is the one kept
    // even when an earlier save is slower.
    const save = terminalSave.then(() => SetTerminalCommand(command))
    terminalSave = save.catch(() => undefined)
    return save.then(
      () => {
        set({ terminalCommand: command.trim() })
        return null
      },
      (error) => errorText(error),
    )
  },

  selectCommit: (hash) =>
    set(hash ? { selectedHash: hash, selectedHashes: [hash], selectionAnchor: hash } : NO_SELECTION),

  toggleCommitSelection: (hash) => {
    const { commits, selectedHash, selectedHashes } = get()
    if (!commits.find((commit) => commit.hash === hash)?.isUnpushed) {
      set({ status: 'Only unpushed commits can be selected together' })
      return
    }

    // A pushed commit can be viewed on its own but never joins a
    // multi-selection, so start over from the clicked commit.
    const current = new Set(unpushedInOrder(commits, selectedHashes))
    if (current.has(hash)) {
      current.delete(hash)
    } else {
      current.add(hash)
    }
    const next = unpushedInOrder(commits, [...current])
    if (next.length === 0) {
      set(NO_SELECTION)
      return
    }
    set({
      selectedHashes: next,
      selectedHash: current.has(hash) ? hash : selectedHash && current.has(selectedHash) ? selectedHash : next[0],
      selectionAnchor: hash,
    })
  },

  extendSelection: (hash) => {
    const { commits, selectedHash, selectionAnchor, selectCommit } = get()
    const anchor = selectionAnchor ?? selectedHash ?? hash
    const from = commits.findIndex((commit) => commit.hash === anchor)
    const to = commits.findIndex((commit) => commit.hash === hash)
    if (from < 0 || to < 0) {
      selectCommit(hash)
      return
    }

    const range = commits
      .slice(Math.min(from, to), Math.max(from, to) + 1)
      .filter((commit) => commit.isUnpushed)
      .map((commit) => commit.hash)
    if (range.length === 0) {
      selectCommit(hash)
      return
    }
    set({
      selectedHashes: range,
      selectedHash: range.includes(hash) ? hash : range[range.length - 1],
      selectionAnchor: anchor,
    })
  },

  selectAllUnpushed: () => {
    const { commits, selectedHash } = get()
    const unpushed = commits.filter((commit) => commit.isUnpushed).map((commit) => commit.hash)
    if (unpushed.length === 0) {
      set({ status: 'There are no unpushed commits to select' })
      return null
    }
    // Keep the commit the user was on as the primary one when it is included.
    const primary = selectedHash && unpushed.includes(selectedHash) ? selectedHash : unpushed[0]
    set({ selectedHashes: unpushed, selectedHash: primary, selectionAnchor: primary })
    return primary
  },

  requestEditFocus: () => set({ pendingEditFocus: true }),

  consumeEditFocus: () => set({ pendingEditFocus: false }),

  setHelpOpen: (isHelpOpen) => set({ isHelpOpen }),

  setSettingsOpen: (isSettingsOpen) => set({ isSettingsOpen }),

  setCanUndo: (canUndo) => set({ canUndo }),

  // runGitOperation runs one git operation at a time. While it runs, activity
  // holds its label; if another operation is already running, the new one is
  // skipped and undefined is returned. Errors propagate to the caller.
  runGitOperation: async (label, operation) => {
    if (get().activity !== null) {
      return undefined
    }

    set({ activity: label })
    try {
      return await operation()
    } finally {
      set({ activity: null })
    }
  },

  // reloadRepository re-reads the repository from disk. Unlike setRepo it keeps
  // the selected commit (if it still exists, so unsaved form edits survive)
  // and the Undo button, since nothing was changed by the app.
  reloadRepository: async () => {
    const { repoInfo: previous, runGitOperation } = get()
    if (!previous) {
      return
    }

    try {
      await runGitOperation(reloadActivityLabel, async () => {
        const info = await ReloadRepository()
        const commits = await GetCommitLog()
        const { selectedHash, selectedHashes, selectionAnchor, canUndo } = get()
        const branchChanged = info.branch !== previous.branch
        const exists = (hash: string | null) => hash !== null && commits.some((commit) => commit.hash === hash)
        const keptHashes = selectedHashes.filter(exists)

        set({
          repoInfo: info,
          commits,
          selectedHash: exists(selectedHash) ? selectedHash : keptHashes.length > 1 ? keptHashes[0] : null,
          selectedHashes: exists(selectedHash) || keptHashes.length > 1 ? keptHashes : [],
          selectionAnchor: exists(selectionAnchor) ? selectionAnchor : null,
          // A different branch means the undo record belongs elsewhere.
          canUndo: branchChanged ? false : canUndo,
          error: null,
          errorDetail: null,
          status: branchChanged
            ? `Branch ${previous.branch} no longer exists; showing ${info.branch}`
            : 'Reloaded from disk',
        })
      })
    } catch (err) {
      get().setError(errorText(err))
    }
  },

  undoLastOperation: async () => {
    const { repoInfo, canUndo, runGitOperation, setRepo } = get()
    if (!repoInfo || !canUndo) {
      return
    }

    await runGitOperation('Undoing last rewrite…', async () => {
      try {
        const result = await UndoLastOperation()
        const refreshedCommits = await RefreshLog()
        // setRepo clears canUndo, which is correct: only one level is kept.
        setRepo(repoInfo, refreshedCommits)
        set({ error: null, errorDetail: null, status: result.message })
      } catch (err) {
        // The backend drops the undo record when it can never succeed (e.g.
        // the branch moved), so ask it whether the button should stay.
        const stillUndoable = await CanUndo().catch(() => false)
        // Undo can fail because the history changed outside GitGo (a commit
        // or a push), so show the current state of the log.
        const commits = await RefreshLog().catch(() => null)
        get().setError(errorText(err))
        set(commits ? { canUndo: stillUndoable, commits } : { canUndo: stillUndoable })
      }
    })
  },

  setStatus: (message) => set({ status: message }),

  setError: (error) => {
    if (error === null) {
      set({ error: null, errorDetail: null })
      return
    }
    const friendly = friendlyError(error)
    // Only keep the raw text when it says more than the friendly message.
    const isSameText = friendly.toLowerCase() === error.replace(/^Error:\s*/, '').trim().toLowerCase()
    set({ error: friendly, errorDetail: isSameText ? null : error })
  },

  closeRepository: () => {
    const path = get().repoInfo?.path
    // The frontend no longer uses the backend state, so a failed close only
    // keeps it in memory until the next OpenRepository replaces it.
    CloseRepository().catch((error) => console.error('Closing the repository failed:', error))
    set({
      repoInfo: null,
      commits: [],
      ...NO_SELECTION,
      canUndo: false,
      pendingEditFocus: false,
      status: path ? `Closed: ${path}` : '',
      error: null,
      errorDetail: null,
    })
  },
}))
