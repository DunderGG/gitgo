import type { KeyboardEvent } from 'react'
import { focusCommitRow } from '../hooks/useKeyboardShortcuts'
import { useRepoStore, CommitSummary, RepoInfo } from '../store/repoStore'

interface CommitRowProps {
  commit: CommitSummary
  isSelected: boolean
  onSelect: () => void
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void
}

function CommitRow({ commit, isSelected, onSelect, onKeyDown }: CommitRowProps) {
  const date = new Date(commit.date)
  const formattedDate = date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })

  return (
    <div
      // Rows are always clickable so the user can inspect any commit; the
      // EditPanel will disable editing for pushed commits.
      role="button"
      tabIndex={0}
      data-commit-hash={commit.hash}
      onClick={onSelect}
      onKeyDown={onKeyDown}
      className={`flex items-center gap-3 px-4 py-2.5 border-b border-gray-800 cursor-pointer transition-colors ${
        isSelected
          ? 'bg-indigo-950/60 border-l-2 border-l-indigo-500'
          : 'hover:bg-gray-800/60'
      } ${commit.isUnpushed ? '' : 'opacity-60'}`}
    >
      {/* Pushed / unpushed indicator dot */}
      <span
        className={`flex-shrink-0 w-2 h-2 rounded-full ${
          commit.isUnpushed ? 'bg-indigo-400' : 'bg-gray-600'
        }`}
        title={commit.isUnpushed ? 'Unpushed — editable' : 'Pushed — read-only'}
      />

      {/* Short hash */}
      <span className="flex-shrink-0 w-16 font-mono text-xs text-gray-500 select-all">
        {commit.shortHash}
      </span>

      {/* Commit message */}
      <span className="flex-1 text-sm text-gray-200 truncate" title={commit.message}>
        {commit.message}
      </span>

      {/* Author */}
      <span className="flex-shrink-0 hidden sm:block text-xs text-gray-400 truncate max-w-32" title={commit.author}>
        {commit.author}
      </span>

      {/* Date */}
      <span className="flex-shrink-0 text-xs text-gray-500 whitespace-nowrap">
        {formattedDate}
      </span>
    </div>
  )
}

type NoticeTone = 'info' | 'warning' | 'muted'

const noticeToneClassName: Record<NoticeTone, string> = {
  info: 'border-sky-900/60 bg-sky-950/30 text-sky-300',
  warning: 'border-yellow-900/60 bg-yellow-950/30 text-yellow-300',
  muted: 'border-gray-800 bg-gray-900 text-gray-400',
}

// listNotice explains situations where the pushed / unpushed split may be
// surprising: with no remote or upstream every commit counts as unpushed, and
// with everything pushed there is nothing to edit.
function listNotice(repoInfo: RepoInfo, unpushedCount: number): { tone: NoticeTone; text: string } | null {
  if (!repoInfo.hasRemote) {
    return {
      tone: 'info',
      text: 'This repository has no remote, so every commit is treated as unpushed and can be edited.',
    }
  }
  if (!repoInfo.hasUpstream) {
    return {
      tone: 'warning',
      text: `Branch ${repoInfo.branch} has no upstream, so every commit is treated as unpushed. If you pushed these commits under another branch name, avoid editing them.`,
    }
  }
  if (unpushedCount === 0) {
    return {
      tone: 'muted',
      text: `Every commit on ${repoInfo.branch} has been pushed, so there is nothing to edit. New local commits will show up here as editable.`,
    }
  }
  return null
}

export default function CommitList() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const commits = useRepoStore((s) => s.commits)
  const selectedHash = useRepoStore((s) => s.selectedHash)
  const selectCommit = useRepoStore((s) => s.selectCommit)
  const requestEditFocus = useRepoStore((s) => s.requestEditFocus)

  // Enter opens the commit in EditPanel (focusing its first field when the
  // commit is editable); the arrow keys move the selection between rows.
  function handleRowKeyDown(event: KeyboardEvent<HTMLDivElement>, index: number) {
    if (event.key === 'Enter') {
      event.preventDefault()
      selectCommit(commits[index].hash)
      requestEditFocus()
      return
    }

    const offset = event.key === 'ArrowDown' ? 1 : event.key === 'ArrowUp' ? -1 : 0
    const next = commits[index + offset]
    if (offset !== 0 && next) {
      event.preventDefault()
      selectCommit(next.hash)
      focusCommitRow(next.hash)
    }
  }

  if (commits.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-gray-500 text-sm">
        This branch has no commits yet.
      </div>
    )
  }

  const unpushedCount = commits.filter((c) => c.isUnpushed).length
  const notice = repoInfo ? listNotice(repoInfo, unpushedCount) : null

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Column header */}
      <div className="flex items-center gap-3 px-4 py-2 bg-gray-800/80 border-b border-gray-700 text-xs text-gray-500 uppercase tracking-wide shrink-0">
        <span className="w-2" />
        <span className="w-16">Hash</span>
        <span className="flex-1">Message</span>
        <span className="hidden sm:block max-w-32">Author</span>
        <span>Date</span>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 px-4 py-1.5 bg-gray-900 border-b border-gray-800 text-xs text-gray-500 shrink-0">
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-indigo-400 inline-block" />
          {unpushedCount} unpushed
        </span>
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-gray-600 inline-block" />
          {commits.length - unpushedCount} pushed
        </span>
      </div>

      {notice && (
        <div className={`px-4 py-2 border-b text-xs shrink-0 ${noticeToneClassName[notice.tone]}`}>
          {notice.text}
        </div>
      )}

      {/* Scrollable commit rows */}
      <div className="flex-1 overflow-y-auto">
        {commits.map((commit, index) => (
          <CommitRow
            key={commit.hash}
            commit={commit}
            isSelected={commit.hash === selectedHash}
            onSelect={() => selectCommit(commit.hash)}
            onKeyDown={(event) => handleRowKeyDown(event, index)}
          />
        ))}
      </div>
    </div>
  )
}
