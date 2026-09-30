import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import { errorText } from './errors'
import { loadSettings, useRepoStore } from './store/repoStore'
import './index.css'

// Surface failures that escape component code (e.g. a rejected backend call
// nobody awaited with a catch) in StatusBar instead of losing them silently.
window.addEventListener('unhandledrejection', (event) => {
  console.error('Unhandled promise rejection:', event.reason)
  useRepoStore.getState().setError(errorText(event.reason))
})

// Without saved settings the app still works, just with an empty recent list.
loadSettings()
  .catch((error) => useRepoStore.getState().setError(errorText(error)))
  .finally(() => {
    ReactDOM.createRoot(document.getElementById('root')!).render(
      <React.StrictMode>
        <ErrorBoundary>
          <App />
        </ErrorBoundary>
      </React.StrictMode>,
    )
  })
