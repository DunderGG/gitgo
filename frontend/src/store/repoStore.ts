import { create } from 'zustand'

const recentReposStorageKey = 'gitgo.recentRepos'
const maxRecentRepos = 10

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
  status: string
  error: string | null
  setRepo: (info: RepoInfo, commits: CommitSummary[]) => void
  removeRecentRepo: (path: string) => void
  selectCommit: (hash: string | null) => void
  setStatus: (message: string) => void
  setError: (error: string | null) => void
  clearRepo: () => void
}

export const useRepoStore = create<RepoStore>((set) => ({
  repoInfo: null,
  commits: [],
  recentRepos: loadRecentRepos(),
  selectedHash: null,
  status: '',
  error: null,

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
        error: null,
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

  setStatus: (message) => set({ status: message }),

  setError: (error) => set({ error }),

  clearRepo: () => set({ repoInfo: null, commits: [], selectedHash: null, status: '', error: null }),
}))
