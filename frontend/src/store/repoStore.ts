import { create } from 'zustand'
import { CanUndo, GetCommitLog, RefreshLog, ReloadRepository, UndoLastOperation } from '../../wailsjs/go/app/App'
import { errorText, friendlyError } from '../errors'

const recentReposStorageKey = 'gitgo.recentRepos'
const maxRecentRepos = 10

// Activity label used while reloadRepository runs; the header's reload button
// compares against it to show its own spinner.
export const reloadActivityLabel = 'Reloading repository…'

function loadRecentRepos(): string[] {
  if (typeof window === 'undefined') {
    return []
  }

  try {
    const raw = window.localStorage.getItem(recentReposStorageKey)
    if (!raw) {
      return []
    }

    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }

    return parsed.filter((path): path is string => typeof path === 'string').slice(0, maxRecentRepos)
  } catch {
    // If storage is corrupted, treat it as empty and continue safely.
    return []
  }
}

function persistRecentRepos(paths: string[]) {
  if (typeof window === 'undefined') {
    return
  }

  try {
    window.localStorage.setItem(recentReposStorageKey, JSON.stringify(paths))
  } catch {
    // Ignore storage write failures; app functionality should still work.
  }
}

function withRecentRepo(paths: string[], path: string): string[] {
  const deduped = [path, ...paths.filter((candidate) => candidate !== path)]
  return deduped.slice(0, maxRecentRepos)
}

export interface RepoInfo {
  path: string
  branch: string
  // False when viewing a branch other than the checked-out one.
  isCheckedOut: boolean
  hasRemote: boolean
  hasUpstream: boolean
}

export interface CommitSummary {
  hash: string
  shortHash: string
  message: string
  author: string
  date: string
  isUnpushed: boolean
}

interface RepoStore {
  repoInfo: RepoInfo | null
  commits: CommitSummary[]
  recentRepos: string[]
  // Hash of the commit currently selected in CommitList; null when nothing is
  // selected. EditPanel reads this to know which commit to load.
  selectedHash: string | null
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
  status: string
  // User-facing error message (see friendlyError), or null.
  error: string | null
  // Raw error text behind error, shown as a tooltip; null when error is
  // already the raw text or there is no error.
  errorDetail: string | null
  setRepo: (info: RepoInfo, commits: CommitSummary[]) => void
  removeRecentRepo: (path: string) => void
  selectCommit: (hash: string | null) => void
  requestEditFocus: () => void
  consumeEditFocus: () => void
  setCanUndo: (canUndo: boolean) => void
  runGitOperation: <T>(label: string, operation: () => Promise<T>) => Promise<T | undefined>
  undoLastOperation: () => Promise<void>
  reloadRepository: () => Promise<void>
  setStatus: (message: string) => void
  // Accepts raw error text; stores a friendly message plus the raw detail.
  setError: (error: string | null) => void
  clearRepo: () => void
}

export const useRepoStore = create<RepoStore>((set, get) => ({
  repoInfo: null,
  commits: [],
  recentRepos: loadRecentRepos(),
  selectedHash: null,
  canUndo: false,
  activity: null,
  pendingEditFocus: false,
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
        selectedHash: null,
        canUndo: false,
        error: null,
        errorDetail: null,
        status: `Opened: ${info.path}`,
      }
    }),

  removeRecentRepo: (path) =>
    set((state) => {
      const recentRepos = state.recentRepos.filter((candidate) => candidate !== path)
      persistRecentRepos(recentRepos)
      return { recentRepos }
    }),

  selectCommit: (hash) => set({ selectedHash: hash }),

  requestEditFocus: () => set({ pendingEditFocus: true }),

  consumeEditFocus: () => set({ pendingEditFocus: false }),

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
        const { selectedHash, canUndo } = get()
        const branchChanged = info.branch !== previous.branch

        set({
          repoInfo: info,
          commits,
          selectedHash: selectedHash && commits.some((commit) => commit.hash === selectedHash) ? selectedHash : null,
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

  clearRepo: () => set({ repoInfo: null, commits: [], selectedHash: null, canUndo: false, status: '', error: null, errorDetail: null }),
}))
