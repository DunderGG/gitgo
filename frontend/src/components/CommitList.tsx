import { useMemo, type KeyboardEvent, type MouseEvent } from 'react'
import { LOG_DEPTH, parseCommitSearch } from '../commitSearch'
import { focusCommitRow, focusCommitSearch } from '../hooks/useKeyboardShortcuts'
import { fetchStatus, useNow, type FetchStatus } from '../lastFetch'
import { useRepoStore, CommitSummary, RepoInfo } from '../store/repoStore'

interface CommitRowProps {
  commit: CommitSummary
  isSelected: boolean
  // The selected commit last clicked or moved to.
  isPrimary: boolean
  onSelect: (event: MouseEvent<HTMLDivElement>) => void
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void
}

function CommitRow({ commit, isSelected, isPrimary, onSelect, onKeyDown }: CommitRowProps) {
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
      aria-pressed={isSelected}
      onClick={onSelect}
      // Keep Shift-click from selecting the row text.
      onMouseDown={(event) => event.shiftKey && event.preventDefault()}
      onKeyDown={onKeyDown}
      className={`flex items-center gap-3 px-4 py-2.5 border-b border-gray-800 cursor-pointer transition-colors ${
        isSelected
          ? `bg-indigo-950/60 border-l-2 ${isPrimary ? 'border-l-indigo-500' : 'border-l-indigo-800'}`
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

const searchHelp = [
  'Search the commits in the list. Every word must match.',
  'A word is looked for in the message, author name and email, hash and date.',
  'author:jane  message:fix  hash:a1b2c3  date:2026-10',
  'since:2026-10-01  until:2026-10-12  "two words"',
].join('\n')

interface CommitSearchBarProps {
  query: string
  onQueryChange: (query: string) => void
  // Commits shown and loaded; shown is null without a search.
  shownCount: number | null
  totalCount: number
  problems: string[]
  // Enter or ↓ in the field: move to the list.
  onEnterList: () => void
}

function CommitSearchBar({ query, onQueryChange, shownCount, totalCount, problems, onEnterList }: CommitSearchBarProps) {
  // The list may stop before the first commit, and the search cannot see
  // past that.
  const totalText = totalCount >= LOG_DEPTH ? `the latest ${totalCount}` : `${totalCount}`

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape' && query !== '') {
      // Clears the search instead of the selection.
      event.preventDefault()
      event.stopPropagation()
      onQueryChange('')
    } else if (event.key === 'Escape') {
      event.currentTarget.blur()
    } else if (event.key === 'Enter' || event.key === 'ArrowDown') {
      event.preventDefault()
      onEnterList()
    }
  }

  return (
    <div className="shrink-0 border-b border-gray-800 bg-gray-900 px-4 py-1.5">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <input
            data-commit-search
            type="text"
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            onKeyDown={handleKeyDown}
            spellCheck={false}
            aria-label="Search commits"
            placeholder="Search message, author, hash or date (Ctrl+F)"
            title={searchHelp}
            className="w-full rounded-md border border-gray-700 bg-gray-800 py-1 pl-2 pr-7 text-sm text-gray-100 outline-none placeholder:text-gray-500 focus:border-indigo-500"
          />
          {query !== '' && (
            <button
              type="button"
              onClick={() => {
                onQueryChange('')
                focusCommitSearch()
              }}
              title="Clear the search (Escape)"
              aria-label="Clear search"
              className="absolute inset-y-0 right-1 my-auto h-5 rounded px-1 text-sm leading-none text-gray-400 transition hover:bg-gray-700 hover:text-gray-200"
            >
              ×
            </button>
          )}
        </div>
        {shownCount !== null && (
          <span className="shrink-0 text-xs text-gray-400" aria-live="polite">
            {shownCount} of {totalText}
          </span>
        )}
      </div>
      {problems.map((problem) => (
        <p key={problem} className="mt-1 text-xs text-yellow-300">
          {problem}
        </p>
      ))}
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
// with an old fetch the split may be out of date, and with everything pushed
// there is nothing to edit.
function listNotice(
  repoInfo: RepoInfo,
  fetch: FetchStatus | null,
  unpushedCount: number,
): { tone: NoticeTone; text: string } | null {
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
  if (fetch?.isStale && unpushedCount > 0) {
    return {
      tone: 'warning',
      text: `The last fetch was ${fetch.age}, so commits pushed since then, for example from another clone, may still show as unpushed. Run git fetch, then press F5 before editing.`,
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
  const selectedHashes = useRepoStore((s) => s.selectedHashes)
  const selectCommit = useRepoStore((s) => s.selectCommit)
  const toggleCommitSelection = useRepoStore((s) => s.toggleCommitSelection)
  const extendSelection = useRepoStore((s) => s.extendSelection)
  const selectAllUnpushed = useRepoStore((s) => s.selectAllUnpushed)
  const requestEditFocus = useRepoStore((s) => s.requestEditFocus)
  const staleFetchDays = useRepoStore((s) => s.staleFetchDays)
  const commitSearch = useRepoStore((s) => s.commitSearch)
  const setCommitSearch = useRepoStore((s) => s.setCommitSearch)
  const now = useNow()
  const isMultiSelect = selectedHashes.length > 1

  const search = useMemo(() => parseCommitSearch(commitSearch), [commitSearch])
  // The rows on screen; the arrow keys, Shift ranges and Select all only
  // move over these.
  const shownCommits = useMemo(
    () => (search.isActive ? commits.filter(search.matches) : commits),
    [commits, search],
  )

  // A plain click selects one commit; Ctrl/Cmd-click and Shift-click build a
  // selection of unpushed commits for BulkEditPanel.
  function handleRowClick(event: MouseEvent<HTMLDivElement>, hash: string) {
    if (event.ctrlKey || event.metaKey) {
      toggleCommitSelection(hash)
    } else if (event.shiftKey) {
      extendSelection(hash)
    } else {
      selectCommit(hash)
    }
  }

  // Enter opens the commit in EditPanel (focusing its first field when the
  // commit is editable); the arrow keys move the selection between rows, and
  // Shift+arrow extends it.
  function handleRowKeyDown(event: KeyboardEvent<HTMLDivElement>, index: number) {
    if (event.key === 'Enter') {
      event.preventDefault()
      selectCommit(shownCommits[index].hash)
      requestEditFocus()
      return
    }

    // ↑ on the first row goes back up to the search field.
    if (event.key === 'ArrowUp' && index === 0 && !event.shiftKey) {
      event.preventDefault()
      focusCommitSearch()
      return
    }

    const offset = event.key === 'ArrowDown' ? 1 : event.key === 'ArrowUp' ? -1 : 0
    const next = shownCommits[index + offset]
    if (offset !== 0 && next) {
      event.preventDefault()
      if (event.shiftKey) {
        extendSelection(next.hash)
      } else {
        selectCommit(next.hash)
      }
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
  const shownUnpushedCount = shownCommits.filter((c) => c.isUnpushed).length
  const isAllUnpushedSelected = shownCommits.every((c) => !c.isUnpushed || selectedHashes.includes(c.hash))
  const shownHashes = new Set(shownCommits.map((c) => c.hash))
  // Selected commits the search hides are still edited by BulkEditPanel.
  const hiddenSelectedCount = selectedHashes.filter((hash) => !shownHashes.has(hash)).length
  const fetch = repoInfo ? fetchStatus(repoInfo, staleFetchDays, now) : null
  const notice = repoInfo ? listNotice(repoInfo, fetch, unpushedCount) : null

  // Focus the primary row so the arrow keys and Escape work right away.
  function handleSelectAll() {
    const primary = selectAllUnpushed()
    if (primary) {
      focusCommitRow(primary)
    }
  }

  // From the search field: to the selected commit if it is shown, otherwise
  // select the first match.
  function enterList() {
    if (selectedHash && shownHashes.has(selectedHash)) {
      focusCommitRow(selectedHash)
    } else if (shownCommits.length > 0) {
      selectCommit(shownCommits[0].hash)
      focusCommitRow(shownCommits[0].hash)
    }
  }

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <CommitSearchBar
        query={commitSearch}
        onQueryChange={setCommitSearch}
        shownCount={search.isActive ? shownCommits.length : null}
        totalCount={commits.length}
        problems={search.problems}
        onEnterList={enterList}
      />

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
          {shownUnpushedCount} unpushed
        </span>
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-gray-600 inline-block" />
          {shownCommits.length - shownUnpushedCount} pushed
        </span>
        <span className="ml-auto flex items-center gap-3">
          {isMultiSelect && (
            <span
              className="text-indigo-300"
              title={hiddenSelectedCount > 0 ? 'Selected commits the search hides are still edited' : undefined}
            >
              {selectedHashes.length} selected
              {hiddenSelectedCount > 0 && `, ${hiddenSelectedCount} hidden by the search`} · Esc to clear
            </span>
          )}
          <button
            type="button"
            onClick={handleSelectAll}
            disabled={shownUnpushedCount === 0 || isAllUnpushedSelected}
            title={
              search.isActive
                ? 'Select every unpushed commit that matches the search (Ctrl+A)'
                : 'Select every unpushed commit (Ctrl+A)'
            }
            className="rounded px-1.5 py-0.5 text-indigo-300 transition hover:bg-gray-800 hover:text-indigo-200 disabled:cursor-default disabled:text-gray-600 disabled:hover:bg-transparent"
          >
            {search.isActive ? 'Select unpushed matches' : 'Select all unpushed'}
          </button>
        </span>
      </div>

      {notice && (
        <div className={`px-4 py-2 border-b text-xs shrink-0 ${noticeToneClassName[notice.tone]}`}>
          {notice.text}
        </div>
      )}

      {/* Scrollable commit rows */}
      <div className="flex-1 overflow-y-auto">
        {shownCommits.length === 0 && (
          <div className="flex flex-col items-center gap-2 px-4 py-8 text-center text-sm text-gray-500">
            <p>No commits match the search.</p>
            {commits.length >= LOG_DEPTH && (
              <p className="text-xs">Only the latest {commits.length} commits are searched.</p>
            )}
            <button
              type="button"
              onClick={() => {
                setCommitSearch('')
                focusCommitSearch()
              }}
              className="rounded px-1.5 py-0.5 text-indigo-300 transition hover:bg-gray-800 hover:text-indigo-200"
            >
              Clear search
            </button>
          </div>
        )}
        {shownCommits.map((commit, index) => (
          <CommitRow
            key={commit.hash}
            commit={commit}
            isSelected={selectedHashes.includes(commit.hash)}
            isPrimary={commit.hash === selectedHash}
            onSelect={(event) => handleRowClick(event, commit.hash)}
            onKeyDown={(event) => handleRowKeyDown(event, index)}
          />
        ))}
      </div>
    </div>
  )
}
