import { create } from 'zustand'

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
  // Hash of the commit currently selected in CommitList; null when nothing is
  // selected. EditPanel reads this to know which commit to load.
  selectedHash: string | null
  status: string
  error: string | null
  setRepo: (info: RepoInfo, commits: CommitSummary[]) => void
  selectCommit: (hash: string | null) => void
  setStatus: (message: string) => void
  setError: (error: string | null) => void
  clearRepo: () => void
}

export const useRepoStore = create<RepoStore>((set) => ({
  repoInfo: null,
  commits: [],
  selectedHash: null,
  status: '',
  error: null,

  // Opening a new repo clears any previous selection so EditPanel doesn't
  // show stale data from the prior repository.
  setRepo: (info, commits) =>
    set({ repoInfo: info, commits, selectedHash: null, error: null, status: `Opened: ${info.path}` }),

  selectCommit: (hash) => set({ selectedHash: hash }),

  setStatus: (message) => set({ status: message }),

  setError: (error) => set({ error }),

  clearRepo: () => set({ repoInfo: null, commits: [], selectedHash: null, status: '', error: null }),
}))
