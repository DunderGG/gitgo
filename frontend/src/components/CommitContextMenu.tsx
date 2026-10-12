import { copyText } from '../clipboard'
import { focusEditPanel } from '../hooks/useKeyboardShortcuts'
import { useRepoStore, type CommitSummary } from '../store/repoStore'
import ContextMenu, { type MenuEntry } from './ContextMenu'

// What the Copy submenu can copy, with the names used in its labels and the
// status message.
const COPY_FIELDS: { name: string; plural: string; value: (commit: CommitSummary) => string }[] = [
  { name: 'full hash', plural: 'full hashes', value: (commit) => commit.hash },
  { name: 'short hash', plural: 'short hashes', value: (commit) => commit.shortHash },
  { name: 'subject', plural: 'subjects', value: (commit) => commit.message },
  { name: 'author name', plural: 'author names', value: (commit) => commit.author },
  { name: 'author email', plural: 'author emails', value: (commit) => commit.authorEmail },
]

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}

interface CommitContextMenuProps {
  x: number
  y: number
  // The commits the menu acts on, in list order: the right-clicked commit,
  // or the whole selection when it was right-clicked inside it.
  targets: CommitSummary[]
  onClose: () => void
}

// CommitContextMenu is the right-click menu of the CommitList rows: Edit, and
// Copy for the hash, subject or author, one line per commit when several are
// selected.
export default function CommitContextMenu({ x, y, targets, onClose }: CommitContextMenuProps) {
  const selectCommit = useRepoStore((s) => s.selectCommit)
  const requestEditFocus = useRepoStore((s) => s.requestEditFocus)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)

  const [first] = targets
  const isSingle = targets.length === 1
  const whose = isSingle ? first.shortHash : `${targets.length} commits`

  async function copy(field: (typeof COPY_FIELDS)[number]) {
    const text = targets.map(field.value).join('\n')
    const what = isSingle ? field.name : field.plural
    try {
      if (!(await copyText(text))) {
        throw new Error('the clipboard refused the text')
      }
      setError(null)
      setStatus(`Copied the ${what} of ${whose}`)
    } catch (error) {
      setError(`Could not copy the ${what}: ${String(error)}`)
    }
  }

  // EditPanel focuses its first field once the commit has loaded; the
  // several-commits panel is already open.
  const edit: MenuEntry = isSingle
    ? {
        label: 'Edit',
        hint: first.isUnpushed ? 'Enter' : 'Pushed',
        disabled: !first.isUnpushed,
        title: first.isUnpushed ? undefined : 'Pushed commits are read-only',
        onSelect: () => {
          selectCommit(first.hash)
          requestEditFocus()
        },
      }
    : { label: `Edit ${targets.length} commits`, onSelect: focusEditPanel }

  const copyItems: MenuEntry[] = COPY_FIELDS.map((field) => ({
    label: capitalize(isSingle ? field.name : field.plural),
    hint: isSingle ? field.value(first) : undefined,
    onSelect: () => void copy(field),
  }))

  return (
    <ContextMenu
      x={x}
      y={y}
      label={isSingle ? `Commit ${first.shortHash}` : `${targets.length} selected commits`}
      entries={[edit, 'separator', { label: 'Copy', submenu: copyItems }]}
      onClose={onClose}
    />
  )
}
