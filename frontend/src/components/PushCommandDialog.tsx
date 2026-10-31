import { useEffect, useRef, useState } from 'react'
import { OpenTerminal } from '../../wailsjs/go/app/App'
import type { app } from '../../wailsjs/go/models'
import { copyText } from '../clipboard'

function commitCount(count: number): string {
  return `${count} ${count === 1 ? 'commit' : 'commits'}`
}

interface PushCommandDialogProps {
  plan: app.PushPlan
  // The commits picked in the list, by hash. Git pushes a commit together
  // with the ones below it, so the plan can push more than these.
  selected: string[]
  onClose: () => void
}

// PushCommandDialog shows the git command from PlanPush that pushes some
// unpushed commits and leaves the newer ones unpushed. GitGo never pushes:
// the user copies the command and runs it in a terminal.
export default function PushCommandDialog({ plan, selected, onClose }: PushCommandDialogProps) {
  const copyRef = useRef<HTMLButtonElement>(null)
  // Feedback next to the buttons, such as "Copied".
  const [note, setNote] = useState<{ text: string; isError: boolean } | null>(null)

  // Like RunOutputDialog, the dialog owns Escape while open and blocks Ctrl+Z
  // and Ctrl+A so the app-wide shortcuts cannot act behind it. Focus moves
  // into the dialog and goes back to where it was on close.
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    copyRef.current?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      const key = event.key.toLowerCase()
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      } else if ((event.ctrlKey || event.metaKey) && (key === 'z' || key === 'a')) {
        event.preventDefault()
        event.stopPropagation()
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      previousFocus?.focus()
    }
  }, [onClose])

  // Returns whether the command is on the clipboard.
  async function copy(): Promise<boolean> {
    try {
      if (await copyText(plan.command)) {
        setNote({ text: 'Copied. Paste it into a terminal in the repository folder.', isError: false })
        return true
      }
      setNote({ text: 'Could not copy', isError: true })
    } catch (error) {
      setNote({ text: `Could not copy: ${String(error)}`, isError: true })
    }
    return false
  }

  async function copyAndOpenTerminal() {
    if (!(await copy())) {
      return
    }
    try {
      await OpenTerminal()
      setNote({ text: 'Copied, and opened a terminal. Paste the command there.', isError: false })
    } catch (error) {
      setNote({ text: `Copied, but could not open a terminal: ${String(error)}`, isError: true })
    }
  }

  const destination = `${plan.remote}/${plan.branch}`
  const upstream = `${plan.upstreamRemote}/${plan.upstreamBranch}`
  const isSelected = new Set(selected)
  // "Push up to here" on one commit means it and the ones below it; only a
  // selection of several can leave out commits it then pushes anyway.
  const isExtra = (hash: string) => selected.length > 1 && !isSelected.has(hash)
  const extraCount = plan.commits.filter((commit) => isExtra(commit.hash)).length

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
      onClick={(event) => event.target === event.currentTarget && onClose()}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="push-command-title"
        className="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-800 px-5 py-4">
          <div className="min-w-0">
            <h2 id="push-command-title" className="text-lg font-semibold text-gray-100">
              Push {commitCount(plan.commits.length)} to {destination}
            </h2>
            <p className="mt-1 text-sm text-gray-400">GitGo never pushes by itself. Run this command in a terminal:</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            title="Close (Escape)"
            aria-label="Close"
            className="rounded-md px-2 py-1 text-gray-400 transition hover:bg-gray-800 hover:text-gray-200"
          >
            ×
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-4">
          <pre
            aria-label="Push command"
            className="select-all whitespace-pre-wrap break-all rounded-md border border-gray-700 bg-gray-950 px-3 py-2 font-mono text-xs text-gray-100"
          >
            {plan.command}
          </pre>

          {extraCount > 0 && (
            <p className="rounded-md border border-yellow-900/60 bg-yellow-950/30 px-3 py-2 text-xs text-yellow-300">
              Git pushes a commit together with every commit below it, so this also pushes{' '}
              {commitCount(extraCount)} you did not select, marked below.
            </p>
          )}
          {plan.isNewBranch && (
            <p className="rounded-md border border-sky-900/60 bg-sky-950/30 px-3 py-2 text-xs text-sky-300">
              As of your last fetch, {plan.remote} has no branch {plan.branch}, so the push creates it.
            </p>
          )}
          {plan.upstreamBranch !== '' && plan.upstreamBranch !== plan.branch && (
            <p className="rounded-md border border-sky-900/60 bg-sky-950/30 px-3 py-2 text-xs text-sky-300">
              {plan.branch} tracks {upstream}, which has another name. The command pushes to {destination}, a branch
              of its own name, so the commits never land on {plan.upstreamBranch} by surprise.
            </p>
          )}

          <div>
            <h3 className="text-xs font-medium uppercase tracking-wide text-gray-400">Pushed</h3>
            <ul className="mt-1 divide-y divide-gray-800 rounded-md border border-gray-800">
              {plan.commits.map((commit) => (
                <li key={commit.hash} className="flex items-center gap-3 px-3 py-1.5 text-sm">
                  <span className="w-16 shrink-0 font-mono text-xs text-gray-500">{commit.shortHash}</span>
                  <span className="min-w-0 flex-1 truncate text-gray-200" title={commit.message}>
                    {commit.message}
                  </span>
                  {isExtra(commit.hash) && <span className="shrink-0 text-xs text-yellow-300">not selected</span>}
                </li>
              ))}
            </ul>
          </div>

          <p className="text-xs text-gray-400">
            {plan.leftOut === 0
              ? `This pushes every unpushed commit on ${plan.branch}.`
              : plan.leftOut === 1
                ? 'The other unpushed commit stays unpushed and editable.'
                : `The other ${plan.leftOut} unpushed commits stay unpushed and editable.`}{' '}
            Once the push has run, GitGo notices it and offers to reload; the pushed commits are then read-only.
          </p>
        </div>

        <div className="flex items-center gap-3 border-t border-gray-800 px-5 py-3">
          <p
            className={`min-w-0 flex-1 truncate text-xs ${note?.isError ? 'text-red-300' : 'text-gray-500'}`}
            title={note?.text}
          >
            {note?.text}
          </p>
          <button
            type="button"
            onClick={copyAndOpenTerminal}
            className="rounded-md border border-gray-700 px-3 py-1.5 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-800"
          >
            Copy and open terminal
          </button>
          <button
            ref={copyRef}
            type="button"
            onClick={copy}
            className="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-indigo-500"
          >
            Copy command
          </button>
        </div>
      </div>
    </div>
  )
}
