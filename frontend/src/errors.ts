// Maps raw error text from the Go backend (and a few browser errors) to
// messages a user can act on. Wails rejects a bound method's promise with the
// Go error's message, so matching is done on that text.

interface FriendlyRule {
  pattern: RegExp
  message: string | ((match: RegExpMatchArray) => string)
}

// Rules are checked in order; the first match wins. Sentinel errors from
// git/errors.go are already readable and fall through to tidy().
const rules: FriendlyRule[] = [
  {
    pattern: /not a git repository/i,
    message: 'This folder is not a Git repository. Choose the folder that contains your project’s .git directory.',
  },
  {
    pattern: /branch not found: (.+)$/im,
    message: (match) => `Branch “${match[1].trim()}” no longer exists. It may have been deleted outside GitGo.`,
  },
  {
    pattern: /no repository is open/i,
    message: 'No repository is open. Open a repository first.',
  },
  {
    pattern: /commit not found/i,
    message: 'That commit could not be found. The history may have changed outside GitGo; reopen the repository.',
  },
  {
    // Checked before the generic stash rules: both the rewrite and the pop failed.
    pattern: /also failed to restore stash/i,
    message:
      'The edit failed, and your uncommitted changes are still in the git stash. Run “git stash pop” to restore them.',
  },
  {
    pattern: /stash pop failed/i,
    message:
      'The commit was updated, but your uncommitted changes could not be restored automatically. They are still in the git stash; run “git stash pop” to restore them.',
  },
  {
    pattern: /stashing changes/i,
    message:
      'GitGo could not stash your uncommitted changes, so nothing was edited. Commit or stash them yourself and try again.',
  },
  {
    pattern: /invalid date/i,
    message: 'The date is not valid. Pick a date and time and try again.',
  },
]

// tidy turns an already-readable message into a sentence: it drops a leading
// "Error: " (added by String(new Error(...))) and capitalises the first letter.
function tidy(text: string): string {
  const trimmed = text.replace(/^Error:\s*/, '').trim()
  if (!trimmed) {
    return 'Something went wrong.'
  }
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1)
}

// errorText converts any thrown value into its message text.
export function errorText(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

// friendlyError returns a user-facing message for a raw error.
export function friendlyError(raw: string): string {
  for (const rule of rules) {
    const match = raw.match(rule.pattern)
    if (match) {
      return typeof rule.message === 'string' ? rule.message : rule.message(match)
    }
  }
  return tidy(raw)
}
