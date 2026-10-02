import { useEffect } from 'react'
import { OpenFolder, OpenTerminal } from '../wailsjs/go/app/App'
import { WindowSetTitle } from '../wailsjs/runtime/runtime'
import BackupsMenu from './components/BackupsMenu'
import BranchSelector from './components/BranchSelector'
import RepoSelector from './components/RepoSelector'
import RepoSwitcher from './components/RepoSwitcher'
import RunMenu from './components/RunMenu'
import StatusBar from './components/StatusBar'
import CommitList from './components/CommitList'
import EditPanel from './components/EditPanel'
import BulkEditPanel from './components/BulkEditPanel'
import HelpDialog from './components/HelpDialog'
import SettingsDialog from './components/SettingsDialog'
import Spinner from './components/Spinner'
import WindowControls from './components/WindowControls'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { reloadActivityLabel, useRepoStore } from './store/repoStore'
import { toggleMaximiseOnDoubleClick, useFramelessWindow } from './titlebar'
import type { ThemePreference } from './theme'

// windowTitle names the open repository and branch, e.g. "GitGo — myrepo (main)",
// so the window is easy to find in the taskbar / window switcher.
function windowTitle(path: string | undefined, branch: string | undefined): string {
  if (!path || !branch) {
    return 'GitGo'
  }
  const repoName = path.split(/[\\/]/).filter(Boolean).pop() ?? path
  return `GitGo — ${repoName} (${branch})`
}

// The theme button cycles System → Light → Dark. U+FE0E asks for the text
// form of ☀ rather than the colour emoji.
const nextTheme: Record<ThemePreference, ThemePreference> = { system: 'light', light: 'dark', dark: 'system' }
const themeLabels: Record<ThemePreference, string> = { system: 'System', light: 'Light', dark: 'Dark' }
const themeIcons: Record<ThemePreference, string> = { system: '◐', light: '☀︎', dark: '☾' }

function App() {
  const repoInfo = useRepoStore((s) => s.repoInfo)
  const activity = useRepoStore((s) => s.activity)
  const isMultiSelect = useRepoStore((s) => s.selectedHashes.length > 1)
  const reloadRepository = useRepoStore((s) => s.reloadRepository)
  const closeRepository = useRepoStore((s) => s.closeRepository)
  const setHelpOpen = useRepoStore((s) => s.setHelpOpen)
  const setSettingsOpen = useRepoStore((s) => s.setSettingsOpen)
  const theme = useRepoStore((s) => s.theme)
  const setTheme = useRepoStore((s) => s.setTheme)
  const setStatus = useRepoStore((s) => s.setStatus)
  const setError = useRepoStore((s) => s.setError)
  const frameless = useFramelessWindow()
  useKeyboardShortcuts()

  async function openTerminal() {
    try {
      await OpenTerminal()
      setError(null)
      setStatus('Opened a terminal in the repository folder')
    } catch (error) {
      setError(String(error))
    }
  }

  async function openFolder() {
    try {
      await OpenFolder()
      setError(null)
      setStatus('Opened the repository folder')
    } catch (error) {
      setError(String(error))
    }
  }

  const title = windowTitle(repoInfo?.path, repoInfo?.branch)
  useEffect(() => {
    document.title = title
    try {
      WindowSetTitle(title)
    } catch {
      // The Wails runtime is missing when the frontend runs in a plain browser
      // (npm run dev); document.title is enough there.
    }
  }, [title])

  return (
    <div className="flex flex-col h-screen bg-gray-900 text-gray-100">
      <header
        className="titlebar flex items-stretch bg-gray-800 border-b border-gray-700 shrink-0"
        onDoubleClick={frameless ? toggleMaximiseOnDoubleClick : undefined}
      >
        <div className="titlebar-drag flex min-w-0 flex-1 items-center px-4 py-3">
          <h1 className="titlebar-drag select-none text-lg font-semibold text-gray-50 tracking-tight">GitGo</h1>
          {repoInfo && (
            <>
              <RepoSwitcher />
              <button
                type="button"
                onClick={closeRepository}
                disabled={activity !== null}
                title="Close the repository and go back to the start screen"
                aria-label="Close repository"
                className="ml-2 shrink-0 rounded-md px-1.5 text-sm text-gray-300 transition hover:bg-gray-700 hover:text-gray-100 disabled:cursor-not-allowed disabled:opacity-60"
              >
                ×
              </button>
            </>
          )}
          <div className="ml-auto flex items-center gap-3 pl-4">
            {repoInfo && (
              <>
                <BranchSelector />
                <button
                  type="button"
                  onClick={reloadRepository}
                  disabled={activity !== null}
                  title="Reload the repository from disk (F5)"
                  aria-label="Reload repository"
                  className="rounded-md border border-gray-700 px-2 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {activity === reloadActivityLabel ? <Spinner className="h-4 w-4" /> : '↻'}
                </button>
                <button
                  type="button"
                  onClick={openTerminal}
                  title="Open a terminal in the repository folder"
                  aria-label="Open terminal"
                  className="rounded-md border border-gray-700 px-2 py-1 font-mono text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
                >
                  &gt;_
                </button>
                <button
                  type="button"
                  onClick={openFolder}
                  title="Show the repository folder in the file manager"
                  aria-label="Open folder"
                  className="rounded-md border border-gray-700 px-2 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
                >
                  {/* An outline folder; the folder emoji would be in colour. */}
                  <svg viewBox="0 0 16 16" className="h-5 w-4" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
                    <path d="M1.5 3.5h4.5l1.5 1.5h7v7.5h-13z" strokeLinejoin="round" />
                  </svg>
                </button>
                <BackupsMenu />
                <RunMenu />
              </>
            )}
            <button
              type="button"
              onClick={() => setTheme(nextTheme[theme])}
              title={`Theme: ${themeLabels[theme]}. Click for ${themeLabels[nextTheme[theme]]}`}
              aria-label={`Theme: ${themeLabels[theme]}`}
              className="rounded-md border border-gray-700 px-2 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
            >
              {themeIcons[theme]}
            </button>
            <button
              type="button"
              onClick={() => setSettingsOpen(true)}
              title="Settings (Ctrl+,)"
              aria-label="Settings"
              className="rounded-md border border-gray-700 px-2 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
            >
              ⚙︎
            </button>
            <button
              type="button"
              onClick={() => setHelpOpen(true)}
              title="How to use GitGo (F1)"
              aria-label="Help"
              className="rounded-md border border-gray-700 px-2.5 py-1 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
            >
              ?
            </button>
          </div>
        </div>
        {frameless && <WindowControls />}
      </header>

      <main className="flex-1 overflow-hidden">
        {!repoInfo ? (
          <RepoSelector />
        ) : (
          <div className="grid h-full grid-cols-1 lg:grid-cols-[minmax(0,1fr)_auto] overflow-hidden">
            <CommitList />
            {isMultiSelect ? <BulkEditPanel /> : <EditPanel />}
          </div>
        )}
      </main>

      <StatusBar />
      <HelpDialog />
      <SettingsDialog />
    </div>
  )
}

export default App
