import { Component, type ErrorInfo, type ReactNode } from 'react'
import { useRepoStore } from '../store/repoStore'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
}

// ErrorBoundary catches errors thrown while rendering the UI and shows a
// recovery screen instead of a blank window. Errors in event handlers and
// promises are not caught here; those go to StatusBar via setError (see the
// global handlers in main.tsx).
export default class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('GitGo UI error:', error, info.componentStack)
  }

  // Re-render the app with the same state. Works when the error was caused by
  // something transient.
  handleTryAgain = () => {
    this.setState({ error: null })
  }

  // Close the repository and go back to the open screen. Git history is not
  // affected; this only resets what the UI shows.
  handleCloseRepository = () => {
    useRepoStore.getState().clearRepo()
    this.setState({ error: null })
  }

  handleReload = () => {
    window.location.reload()
  }

  render() {
    const { error } = this.state
    if (!error) {
      return this.props.children
    }

    return (
      <div className="flex h-screen items-center justify-center bg-gray-900 p-6 text-gray-100">
        <div className="w-full max-w-lg rounded-xl border border-gray-700 bg-gray-800 p-6 shadow-2xl" role="alert">
          <h1 className="text-lg font-semibold text-white">Something went wrong</h1>
          <p className="mt-2 text-sm text-gray-300">
            GitGo hit an unexpected problem while showing this screen. Your repository has not been changed by this
            error.
          </p>
          <p className="mt-2 text-sm text-gray-400">
            Try again first. If the problem keeps happening, close the repository or reload GitGo.
          </p>

          <details className="mt-4 rounded-md border border-gray-700 bg-gray-900/60 px-3 py-2 text-xs text-gray-400">
            <summary className="cursor-pointer select-none text-gray-300">Technical details</summary>
            <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words font-mono">
              {error.stack ?? error.message}
            </pre>
          </details>

          <div className="mt-5 flex flex-wrap justify-end gap-3">
            <button
              type="button"
              onClick={this.handleReload}
              className="rounded-md border border-gray-700 px-4 py-2 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
            >
              Reload GitGo
            </button>
            <button
              type="button"
              onClick={this.handleCloseRepository}
              className="rounded-md border border-gray-700 px-4 py-2 text-sm text-gray-300 transition hover:border-gray-600 hover:bg-gray-700"
            >
              Close repository
            </button>
            <button
              type="button"
              onClick={this.handleTryAgain}
              className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500"
            >
              Try again
            </button>
          </div>
        </div>
      </div>
    )
  }
}
