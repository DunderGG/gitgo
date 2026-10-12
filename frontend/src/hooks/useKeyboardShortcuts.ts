import { useEffect } from 'react'
import { useRepoStore } from '../store/repoStore'
import { DEFAULT_ZOOM, zoomIn, zoomOut } from '../zoom'

// isTextEditingTarget reports whether the key event comes from a field where
// the browser's own shortcuts (e.g. Ctrl+Z for text undo) must keep working.
function isTextEditingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable
}

// focusCommitRow moves keyboard focus to the CommitList row for hash, if it
// is rendered.
export function focusCommitRow(hash: string) {
  document.querySelector<HTMLElement>(`[data-commit-hash="${hash}"]`)?.focus()
}

// focusEditPanel moves keyboard focus to the first enabled control of the
// edit panel (EditPanel or BulkEditPanel).
export function focusEditPanel() {
  document
    .querySelector<HTMLElement>('[data-edit-panel] :is(button, input, select, textarea):not(:disabled)')
    ?.focus()
}

// focusCommitSearch moves keyboard focus to the search field above
// CommitList and selects its text, so typing replaces the last search.
export function focusCommitSearch() {
  const field = document.querySelector<HTMLInputElement>('[data-commit-search]')
  field?.focus()
  field?.select()
}

// useKeyboardShortcuts registers the app-wide shortcuts:
//   - F5 / Ctrl+R / Cmd+R: reload the repository from disk (instead of the page)
//   - Ctrl+Z / Cmd+Z: undo the last rewrite (not while typing in a field)
//   - Ctrl+A / Cmd+A: select every unpushed commit (not while typing in a field)
//   - Ctrl+F / Cmd+F: search the commit list
//   - Escape: close the edit panel by clearing the selection
//   - F1: open the help
//   - Ctrl+, / Cmd+,: open the settings
//   - Ctrl+= / Ctrl+- / Ctrl+0 (Cmd on macOS): zoom in, out, or back to 100%
//
// Row-level shortcuts (Enter, arrow keys) live in CommitList, and
// ConfirmDialog, HelpDialog and SettingsDialog handle Escape themselves while
// they are open.
export function useKeyboardShortcuts() {
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const {
        repoInfo,
        selectedHash,
        selectCommit,
        selectAllUnpushed,
        undoLastOperation,
        reloadRepository,
        setHelpOpen,
        setSettingsOpen,
        zoom,
        setZoom,
      } = useRepoStore.getState()

      if (event.key === 'F1') {
        event.preventDefault()
        // Not on top of another dialog, where Escape would close both.
        if (!document.querySelector('[aria-modal="true"]')) {
          setHelpOpen(true)
        }
        return
      }

      if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key === ',') {
        event.preventDefault()
        if (!document.querySelector('[aria-modal="true"]')) {
          setSettingsOpen(true)
        }
        return
      }

      // Zoom works everywhere, in fields and dialogs too, as in a browser.
      // Ctrl+= is Ctrl+ on keyboards where + needs Shift, and the numpad
      // keys give the same key values.
      if ((event.ctrlKey || event.metaKey) && !event.altKey && ['+', '=', '-', '0'].includes(event.key)) {
        event.preventDefault()
        if (event.key === '0') {
          setZoom(DEFAULT_ZOOM)
        } else {
          setZoom(event.key === '-' ? zoomOut(zoom) : zoomIn(zoom))
        }
        return
      }

      // F5 / Ctrl+R would otherwise reload the whole webview and lose the UI
      // state, so they are always intercepted and reload the repository instead.
      const isReloadShortcut =
        event.key === 'F5' || ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'r')

      if (isReloadShortcut) {
        event.preventDefault()
        if (repoInfo) {
          reloadRepository()
        }
        return
      }

      const isUndoShortcut =
        (event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'z'

      if (isUndoShortcut && !isTextEditingTarget(event.target)) {
        event.preventDefault()
        undoLastOperation()
        return
      }

      const isSelectAllShortcut =
        (event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'a'

      // Not in a field, where Ctrl+A selects its text, nor behind a dialog.
      if (
        isSelectAllShortcut &&
        repoInfo &&
        !isTextEditingTarget(event.target) &&
        !document.querySelector('[aria-modal="true"]')
      ) {
        event.preventDefault()
        const primary = selectAllUnpushed()
        if (primary) {
          focusCommitRow(primary)
        }
        return
      }

      const isSearchShortcut =
        (event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'f'

      // Works from text fields too, but not behind a dialog.
      if (isSearchShortcut) {
        event.preventDefault()
        if (repoInfo && !document.querySelector('[aria-modal="true"]')) {
          focusCommitSearch()
        }
        return
      }

      if (event.key === 'Escape' && selectedHash) {
        event.preventDefault()
        selectCommit(null)
        // Return focus to the list so arrow keys and Enter keep working.
        focusCommitRow(selectedHash)
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])
}
