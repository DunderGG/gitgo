import { PlanPush } from '../../wailsjs/go/app/App'
import type { app } from '../../wailsjs/go/models'
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
  // Push up to here: called with the plan from PlanPush and the hashes it
  // was asked for, after the menu has closed.
  onPushPlan: (plan: app.PushPlan, selected: string[]) => void
}

// CommitContextMenu is the right-click menu of the CommitList rows: Edit,
// Push up to here (a git push command for the user to run), and Copy for the
// hash, subject or author, one line per commit when several are selected.
export default function CommitContextMenu({ x, y, targets, onClose, onPushPlan }: CommitContextMenuProps) {
  const hasRemote = useRepoStore((s) => s.repoInfo?.hasRemote ?? false)
  const selectCommit = useRepoStore((s) => s.selectCommit)
  const requestEditFocus = useRepoStore((s) => s.requestEditFocus)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)
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

  async function planPush() {
    const selected = targets.map((commit) => commit.hash)
    try {
      const plan = await runGitOperation('Working out the push command…', () => PlanPush(selected))
      if (plan) {
        setError(null)
        onPushPlan(plan, selected)
      }
    } catch (error) {
      setError(String(error))
    }
  }

  // Only a single commit can be a pushed one: several selected are always
  // unpushed.
  const isPushed = !first.isUnpushed
  const push: MenuEntry = {
    label: isSingle ? 'Push up to here…' : `Push ${targets.length} commits…`,
    hint: isPushed ? 'Pushed' : undefined,
    disabled: isPushed || !hasRemote,
    title: isPushed
      ? 'This commit has already been pushed'
      : !hasRemote
        ? 'This repository has no remote to push to'
        : isSingle
          ? 'Get the git command that pushes this commit and the ones below it, and no newer ones'
          : 'Get the git command that pushes the selected commits, and no newer ones',
    onSelect: () => void planPush(),
  }

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
      entries={[edit, push, 'separator', { label: 'Copy', submenu: copyItems }]}
      onClose={onClose}
    />
  )
}
