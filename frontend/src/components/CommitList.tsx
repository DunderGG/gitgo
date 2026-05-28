import { useRepoStore, CommitSummary } from '../store/repoStore'

function CommitRow({ commit }: { commit: CommitSummary }) {
  const date = new Date(commit.date)
  const formattedDate = date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })

  return (
    <div
      className={`flex items-center gap-3 px-4 py-2.5 border-b border-gray-800 hover:bg-gray-800/60 transition-colors ${
        commit.isUnpushed ? '' : 'opacity-60'
      }`}
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

export default function CommitList() {
  const commits = useRepoStore((s) => s.commits)

  if (commits.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-gray-500 text-sm">
        No commits found.
      </div>
    )
  }

  const unpushedCount = commits.filter((c) => c.isUnpushed).length

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

      {/* Scrollable commit rows */}
      <div className="flex-1 overflow-y-auto">
        {commits.map((commit) => (
          <CommitRow key={commit.hash} commit={commit} />
        ))}
      </div>
    </div>
  )
}
