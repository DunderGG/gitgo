import { useEffect, useRef, type ReactNode } from 'react'
import Kbd from './Kbd'
import { useRepoStore } from '../store/repoStore'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h3 className="text-xs font-medium uppercase tracking-wide text-gray-400">{title}</h3>
      <div className="space-y-2 text-sm text-gray-300">{children}</div>
    </section>
  )
}

function Steps({ children }: { children: ReactNode }) {
  return <ol className="list-decimal space-y-1.5 pl-5 marker:text-gray-500">{children}</ol>
}

function Bullets({ children }: { children: ReactNode }) {
  return <ul className="list-disc space-y-1.5 pl-5 marker:text-gray-600">{children}</ul>
}

// A button or field name as it appears in the UI.
function Ui({ children }: { children: ReactNode }) {
  return <span className="font-medium text-gray-100">{children}</span>
}

const SHORTCUTS: { keys: ReactNode; action: string }[] = [
  { keys: <><Kbd>↑</Kbd> <Kbd>↓</Kbd></>, action: 'Move the selection between commits' },
  { keys: <><Kbd>Shift</Kbd>+<Kbd>↑</Kbd> <Kbd>Shift</Kbd>+<Kbd>↓</Kbd></>, action: 'Extend the selection over several unpushed commits' },
  { keys: <><Kbd>Ctrl</Kbd>+click</>, action: 'Add or remove an unpushed commit from the selection' },
  { keys: <><Kbd>Shift</Kbd>+click</>, action: 'Select every unpushed commit between the last selected one and the clicked one' },
  { keys: <Kbd>Enter</Kbd>, action: 'Open the selected commit and jump to its first field' },
  { keys: <Kbd>Escape</Kbd>, action: 'Close the edit panel, confirmation dialog or this help' },
  { keys: <><Kbd>Ctrl</Kbd>+<Kbd>Z</Kbd></>, action: 'Undo the last rewrite (outside text fields)' },
  { keys: <><Kbd>F5</Kbd> / <Kbd>Ctrl</Kbd>+<Kbd>R</Kbd></>, action: 'Reload the repository from disk' },
  { keys: <Kbd>F1</Kbd>, action: 'Open or close this help' },
]

// HelpDialog explains how to use the app: a walkthrough of editing one or
// several commits, the keyboard shortcuts, and the safety rules.
export default function HelpDialog() {
  const isOpen = useRepoStore((s) => s.isHelpOpen)
  const setHelpOpen = useRepoStore((s) => s.setHelpOpen)
  const closeRef = useRef<HTMLButtonElement>(null)

  // Like ConfirmDialog, the dialog owns Escape (and F1) while open and blocks
  // Ctrl+Z so the app-wide shortcuts cannot act behind it. Focus moves into
  // the dialog and goes back to where it was on close.
  useEffect(() => {
    if (!isOpen) {
      return
    }
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeRef.current?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      const isUndoShortcut = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z'

      if (event.key === 'Escape' || event.key === 'F1') {
        event.preventDefault()
        event.stopPropagation()
        setHelpOpen(false)
      } else if (isUndoShortcut) {
        event.preventDefault()
        event.stopPropagation()
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
      previousFocus?.focus()
    }
  }, [isOpen, setHelpOpen])

  if (!isOpen) {
    return null
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/75 p-4"
      onClick={(event) => event.target === event.currentTarget && setHelpOpen(false)}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="help-title"
        className="flex max-h-[90vh] w-full max-w-3xl flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-800 px-5 py-4">
          <div>
            <h2 id="help-title" className="text-lg font-semibold text-gray-100">
              How to use GitGo
            </h2>
            <p className="mt-1 text-sm text-gray-400">
              Edit the message, date and author of commits you have not pushed yet.
            </p>
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={() => setHelpOpen(false)}
            title="Close (Escape)"
            aria-label="Close help"
            className="rounded-md px-2 py-1 text-gray-400 transition hover:bg-gray-800 hover:text-gray-200"
          >
            ×
          </button>
        </div>

        <div className="space-y-6 overflow-y-auto p-5">
          <Section title="1. Open a repository">
            <p>
              Click <Ui>Open Repository</Ui> and pick the folder of a local Git repository. Recently opened
              repositories are listed on the start screen for quick access.
            </p>
            <p>
              The checked-out branch is shown first. Use the <Ui>Branch</Ui> menu in the header to view and edit
              another local branch; it is not checked out, and your working tree is left alone.
            </p>
          </Section>

          <Section title="2. Pushed and unpushed commits">
            <p>Only commits you have not pushed yet can be edited. The dot at the start of each row tells them apart:</p>
            <Bullets>
              <li>
                <span className="mr-1.5 inline-block h-2 w-2 rounded-full bg-indigo-400" />
                <Ui>Unpushed</Ui>: editable.
              </li>
              <li>
                <span className="mr-1.5 inline-block h-2 w-2 rounded-full bg-gray-600" />
                <Ui>Pushed</Ui>: dimmed and read-only. You can still select it to see its details.
              </li>
            </Bullets>
            <p className="text-gray-400">
              Without a remote or an upstream branch, every commit counts as unpushed. A notice above the commit list
              tells you when this is the case.
            </p>
          </Section>

          <Section title="3. Edit a single commit">
            <Steps>
              <li>
                Click an unpushed commit, or move to it with <Kbd>↑</Kbd> <Kbd>↓</Kbd> and press <Kbd>Enter</Kbd>.
                Its details load in the <Ui>Edit Commit</Ui> panel.
              </li>
              <li>
                Change the <Ui>Message</Ui>, <Ui>Author Date</Ui>, <Ui>Author Name</Ui> or <Ui>Author Email</Ui>.
              </li>
              <li>
                For the date, type a new time (to the second) and pick a time zone, or use the quick buttons:{' '}
                <Ui>−1d</Ui> <Ui>−1h</Ui> <Ui>+1h</Ui> <Ui>+1d</Ui> and <Ui>Now</Ui>. Tick{' '}
                <Ui>Also set committer date</Ui> to give the committer date the same value.
              </li>
              <li>
                Click <Ui>Review Changes</Ui>. <Ui>Reset</Ui> puts the fields back to how they were.
              </li>
            </Steps>
          </Section>

          <Section title="4. Shift the dates of several commits">
            <Steps>
              <li>
                Select several unpushed commits: <Kbd>Ctrl</Kbd>+click to add or remove one at a time,{' '}
                <Kbd>Shift</Kbd>+click to select a range, or hold <Kbd>Shift</Kbd> and use <Kbd>↑</Kbd> <Kbd>↓</Kbd>.
                The <Ui>Shift Commit Dates</Ui> panel replaces the edit panel.
              </li>
              <li>
                Use <Ui>−1d</Ui> <Ui>−1h</Ui> <Ui>+1h</Ui> <Ui>+1d</Ui> to build up the shift. Every selected commit
                moves by the same amount and keeps its own time zone. The list below shows each new date.
              </li>
              <li>
                Tick <Ui>Also shift committer dates</Ui> if you want those moved too, then click{' '}
                <Ui>Review Changes</Ui>.
              </li>
            </Steps>
            <p className="text-gray-400">
              Only dates can be changed for several commits at once; messages and authors stay as they are.
            </p>
          </Section>

          <Section title="5. Review and apply">
            <p>
              The confirmation dialog shows the current and new values side by side, with changes highlighted. Click{' '}
              <Ui>Apply</Ui> to rewrite the history, or <Ui>Cancel</Ui> to go back. The dialog also warns you when:
            </p>
            <Bullets>
              <li>
                <Ui>Signed commits</Ui> would lose their GPG/SSH signature. A signature only matches the exact commit
                it was made for, and GitGo cannot re-sign commits yet.
              </li>
              <li>
                <Ui>Other branches</Ui> point at a rewritten commit. Tick the box to move them along with the edit.
              </li>
              <li>
                <Ui>Tags</Ui> point at a rewritten commit. Tags are never moved and keep pointing at the old commit.
              </li>
            </Bullets>
          </Section>

          <Section title="6. Undo">
            <p>
              Made a mistake? Click <Ui>Undo</Ui> in the status bar or press <Kbd>Ctrl</Kbd>+<Kbd>Z</Kbd> to put the
              branch back to how it was before the last rewrite. Only the most recent rewrite can be undone, and
              switching branches clears it.
            </p>
          </Section>

          <Section title="Keyboard shortcuts">
            <table className="w-full text-left text-sm">
              <tbody>
                {SHORTCUTS.map(({ keys, action }) => (
                  <tr key={action} className="border-t border-gray-800 first:border-t-0 align-top">
                    <td className="whitespace-nowrap py-1.5 pr-4 text-gray-400">{keys}</td>
                    <td className="py-1.5 text-gray-300">{action}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="text-gray-400">
              On macOS, use <Kbd>Cmd</Kbd> in place of <Kbd>Ctrl</Kbd>.
            </p>
          </Section>

          <Section title="Good to know">
            <Bullets>
              <li>
                Editing a commit gives it and every commit above it a new hash, because a commit's hash covers its
                parent.
              </li>
              <li>
                Uncommitted changes (staged or not) and the stash are never touched.
              </li>
              <li>
                Whether a commit is pushed is checked again right before every edit and undo, so an edit is refused if
                you pushed in the meantime.
              </li>
              <li>
                Every rewrite and undo is written to the reflog with a <span className="font-mono">gitgo:</span>{' '}
                prefix. From the command line you can always recover with{' '}
                <span className="font-mono text-gray-100">git reset --hard &lt;branch&gt;@&#123;1&#125;</span>.
              </li>
              <li>
                The committer name and email are always kept; only the author can be changed.
              </li>
              <li>
                Made changes outside GitGo, for example a new commit in a terminal? Press <Kbd>F5</Kbd> or click{' '}
                <Ui>↻</Ui> in the header to reload.
              </li>
            </Bullets>
          </Section>
        </div>
      </div>
    </div>
  )
}
