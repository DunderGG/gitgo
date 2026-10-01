import { useEffect, useRef, useState } from 'react'
import { SaveRunOutput } from '../../wailsjs/go/app/App'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import type { app } from '../../wailsjs/go/models'

// copyText puts text on the clipboard through Wails, or the browser when the
// frontend runs without it (npm run dev).
async function copyText(text: string): Promise<boolean> {
  try {
    return await ClipboardSetText(text)
  } catch {
    await navigator.clipboard.writeText(text)
    return true
  }
}

interface RunOutputDialogProps {
  title: string
  result: app.RunResult
  onClose: () => void
}

// RunOutputDialog shows the read-only output of a command from the Run menu,
// with the command that produced it and buttons to copy or save the output.
export default function RunOutputDialog({ title, result, onClose }: RunOutputDialogProps) {
  const closeRef = useRef<HTMLButtonElement>(null)
  // Feedback next to the buttons, such as "Copied" or the saved path.
  const [note, setNote] = useState<{ text: string; isError: boolean } | null>(null)

  // Like SettingsDialog, the dialog owns Escape while open and blocks Ctrl+Z
  // and Ctrl+A outside the output so the app-wide shortcuts cannot act behind
  // it. Focus moves into the dialog and goes back to where it was on close.
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeRef.current?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      const key = event.key.toLowerCase()
      const isBlockedShortcut = (event.ctrlKey || event.metaKey) && (key === 'z' || key === 'a')

      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      } else if (isBlockedShortcut && !(event.target instanceof HTMLTextAreaElement)) {
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

  async function copy() {
    try {
      const isCopied = await copyText(result.output)
      setNote(isCopied ? { text: 'Copied to the clipboard', isError: false } : { text: 'Could not copy', isError: true })
    } catch (error) {
      setNote({ text: `Could not copy: ${String(error)}`, isError: true })
    }
  }

  async function save() {
    try {
      const path = await SaveRunOutput(result.fileName, result.output)
      if (path) {
        setNote({ text: `Saved to ${path}`, isError: false })
      }
    } catch (error) {
      setNote({ text: String(error), isError: true })
    }
  }

  const lineCount = result.output === '' ? 0 : result.output.replace(/\n$/, '').split('\n').length

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
      onClick={(event) => event.target === event.currentTarget && onClose()}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="run-output-title"
        className="flex h-[85vh] w-full max-w-4xl flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-800 px-5 py-4">
          <div className="min-w-0">
            <h2 id="run-output-title" className="text-lg font-semibold text-gray-100">
              {title}
            </h2>
            <pre className="mt-1 whitespace-pre-wrap break-all font-mono text-xs text-gray-400">{result.command}</pre>
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            title="Close (Escape)"
            aria-label="Close output"
            className="rounded-md px-2 py-1 text-gray-400 transition hover:bg-gray-800 hover:text-gray-200"
          >
            ×
          </button>
        </div>

        {result.truncated && (
          <p className="border-b border-gray-800 bg-yellow-950/30 px-5 py-2 text-xs text-yellow-300">
            The output was cut off at 10 MB. Run the command in a terminal for all of it.
          </p>
        )}

        <textarea
          readOnly
          value={result.output}
          wrap="off"
          spellCheck={false}
          aria-label="Output"
          className="min-h-0 flex-1 resize-none bg-gray-950 px-5 py-3 font-mono text-xs leading-5 text-gray-200 outline-none"
        />

        <div className="flex items-center gap-3 border-t border-gray-800 px-5 py-3">
          <p
            className={`min-w-0 flex-1 truncate text-xs ${note?.isError ? 'text-red-300' : 'text-gray-500'}`}
            title={note?.text}
          >
            {note?.text ?? `${lineCount} ${lineCount === 1 ? 'line' : 'lines'}`}
          </p>
          <button
            type="button"
            onClick={copy}
            className="rounded-md border border-gray-700 px-3 py-1.5 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-800"
          >
            Copy
          </button>
          <button
            type="button"
            onClick={save}
            className="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-indigo-500"
          >
            Save…
          </button>
        </div>
      </div>
    </div>
  )
}
