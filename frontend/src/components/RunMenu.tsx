import { useCallback, useEffect, useState } from 'react'
import { ExportPatches, GetGitStatus, RunHistory } from '../../wailsjs/go/app/App'
import type { app } from '../../wailsjs/go/models'
import { useRepoStore } from '../store/repoStore'
import RunOutputDialog from './RunOutputDialog'

// History formats RunHistory accepts (app/run.go), with their menu labels.
const HISTORY_FORMATS = [
  { format: 'oneline', label: 'One line per commit' },
  { format: 'full', label: 'Full metadata' },
  { format: 'csv', label: 'Authors and dates (CSV)' },
] as const

type HistoryFormat = (typeof HISTORY_FORMATS)[number]['format']

const EXPORT_PATCHES = 'patches'

// Option values: "history:<format>:all", "history:<format>:selected" or
// EXPORT_PATCHES.
function historyValue(format: HistoryFormat, scope: 'all' | 'selected'): string {
  return `history:${format}:${scope}`
}

// What the output dialog shows for a finished command.
interface Shown {
  title: string
  result: app.RunResult
}

// RunMenu is the header dropdown of ready-made git commands, so common tasks
// need no terminal. GitGo builds every command itself and only runs commands
// that read the repository. The commands need the native git program, so the
// menu is disabled with an explanation when it is not installed.
export default function RunMenu() {
  const selectedHashes = useRepoStore((s) => s.selectedHashes)
  const activity = useRepoStore((s) => s.activity)
  const runGitOperation = useRepoStore((s) => s.runGitOperation)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const [gitStatus, setGitStatus] = useState<app.GitStatus | null>(null)
  const [shown, setShown] = useState<Shown | null>(null)
  const close = useCallback(() => setShown(null), [])

  // Checked each time a repository is opened, so installing git meanwhile
  // only needs the repository reopened.
  useEffect(() => {
    GetGitStatus()
      .then(setGitStatus)
      .catch((error) => setGitStatus({ available: false, version: '', problem: String(error) }))
  }, [])

  const selectedCount = selectedHashes.length
  const commitsLabel = `${selectedCount} selected ${selectedCount === 1 ? 'commit' : 'commits'}`

  async function runHistory(format: HistoryFormat, hashes: string[]) {
    const label = HISTORY_FORMATS.find((entry) => entry.format === format)?.label ?? format
    const scope = hashes.length > 0 ? commitsLabel : 'all commits in the list'
    const result = await runGitOperation('Running git log…', () => RunHistory(format, hashes))
    if (result) {
      setShown({ title: `History of ${scope}: ${label.toLowerCase()}`, result })
      setStatus(`Ran git log over ${scope}`)
    }
  }

  async function exportPatches(hashes: string[]) {
    const result = await runGitOperation('Exporting patches…', () => ExportPatches(hashes))
    // An empty command means the folder dialog was cancelled.
    if (result?.command) {
      setShown({ title: `Patches for ${commitsLabel}`, result })
      setStatus(`Exported ${commitsLabel} as patches`)
    }
  }

  async function handleChange(value: string) {
    // Run from what is selected now, not when the menu was rendered.
    const hashes = useRepoStore.getState().selectedHashes
    try {
      setError(null)
      if (value === EXPORT_PATCHES) {
        await exportPatches(hashes)
        return
      }
      const [, format, scope] = value.split(':') as [string, HistoryFormat, string]
      await runHistory(format, scope === 'selected' ? hashes : [])
    } catch (error) {
      setError(String(error))
    }
  }

  const isAvailable = gitStatus?.available === true
  const title = !gitStatus
    ? 'Looking for git…'
    : isAvailable
      ? `Run a ready-made git command (git ${gitStatus.version})`
      : gitStatus.problem
  const optionClass = 'bg-gray-900 text-gray-100'

  return (
    <>
      {/* The title is on a wrapper, since a disabled control may not show one. */}
      <span title={title} className="flex">
        <select
          value=""
          onChange={(event) => handleChange(event.target.value)}
          disabled={!isAvailable || activity !== null}
          aria-label="Run a git command"
          className="rounded-md border border-gray-700 bg-transparent px-2 py-1 text-sm text-gray-300 outline-none transition hover:border-gray-600 hover:bg-gray-700 focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
        >
          <option value="" disabled hidden>
            Run
          </option>
          <optgroup label="History of all commits in the list" className={optionClass}>
            {HISTORY_FORMATS.map(({ format, label }) => (
              <option key={format} value={historyValue(format, 'all')} className={optionClass}>
                {label}
              </option>
            ))}
          </optgroup>
          {selectedCount > 0 && (
            <optgroup label={`History of the ${commitsLabel}`} className={optionClass}>
              {HISTORY_FORMATS.map(({ format, label }) => (
                <option key={format} value={historyValue(format, 'selected')} className={optionClass}>
                  {label}
                </option>
              ))}
            </optgroup>
          )}
          <optgroup label="Export" className={optionClass}>
            <option value={EXPORT_PATCHES} disabled={selectedCount === 0} className={optionClass}>
              {selectedCount > 0 ? `${commitsLabel} as patches…` : 'Selected commits as patches… (select commits first)'}
            </option>
          </optgroup>
        </select>
      </span>
      {shown && <RunOutputDialog title={shown.title} result={shown.result} onClose={close} />}
    </>
  )
}
