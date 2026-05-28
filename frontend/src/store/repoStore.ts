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
  status: string
  error: string | null
  setRepo: (info: RepoInfo, commits: CommitSummary[]) => void
  setStatus: (message: string) => void
  setError: (error: string | null) => void
  clearRepo: () => void
}

export const useRepoStore = create<RepoStore>((set) => ({
  repoInfo: null,
  commits: [],
  status: '',
  error: null,

  setRepo: (info, commits) =>
    set({ repoInfo: info, commits, error: null, status: `Opened: ${info.path}` }),

  setStatus: (message) => set({ status: message }),

  setError: (error) => set({ error }),

  clearRepo: () => set({ repoInfo: null, commits: [], status: '', error: null }),
}))
