// The last fetch: pushed and unpushed commits are worked out from the
// remote-tracking branches, which only change when the repository is fetched.
// GitGo never fetches itself, so it shows how old that information is and
// warns when it is older than the user's setting (Settings.StaleFetchDays on
// the Go side).
import { useEffect, useState } from 'react'
import type { RepoInfo } from './store/repoStore'

// Same as the Go defaults, used until the settings have loaded.
export const DEFAULT_STALE_FETCH_DAYS = 7
export const MAX_STALE_FETCH_DAYS = 365

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

// A rough age such as "5 minutes ago" or "3 days ago".
export function formatAge(milliseconds: number): string {
  const units: [number, string][] = [
    [DAY, 'day'],
    [HOUR, 'hour'],
    [MINUTE, 'minute'],
  ]
  for (const [size, name] of units) {
    const count = Math.floor(milliseconds / size)
    if (count >= 1) {
      return `${count} ${name}${count === 1 ? '' : 's'} ago`
    }
  }
  return 'just now'
}

export interface FetchStatus {
  // Short text for the status bar, for example "Fetched 3 days ago".
  label: string
  // The full explanation, for a tooltip.
  title: string
  // How long ago the last fetch was, or null when none is recorded.
  age: string | null
  // True when the last fetch is older than staleFetchDays.
  isStale: boolean
}

const EXPLANATION =
  'Pushed and unpushed commits are worked out from the remote-tracking branches, which only change when you run git fetch or git pull. GitGo never fetches by itself.'

// What to show about the last fetch, or null for a repository without a
// remote, where there is nothing to fetch. staleFetchDays 0 never warns.
export function fetchStatus(repoInfo: RepoInfo, staleFetchDays: number, now: number): FetchStatus | null {
  if (!repoInfo.hasRemote) {
    return null
  }
  const fetchedAt = Date.parse(repoInfo.lastFetch)
  if (!repoInfo.lastFetch || Number.isNaN(fetchedAt)) {
    return {
      label: 'No fetch recorded',
      title: `No git fetch is recorded for this repository (a fresh clone has none). ${EXPLANATION}`,
      age: null,
      isStale: false,
    }
  }
  // A clock set back can put the fetch in the future; treat it as just now.
  const milliseconds = Math.max(0, now - fetchedAt)
  const age = formatAge(milliseconds)
  const isStale = staleFetchDays > 0 && milliseconds >= staleFetchDays * DAY
  const staleNote = isStale
    ? ` That is more than ${staleFetchDays} day${staleFetchDays === 1 ? '' : 's'} ago: commits pushed since then, for example from another clone, may still show as unpushed. Run git fetch, then press F5.`
    : ''
  return {
    label: `Fetched ${age}`,
    title: `Last fetch: ${new Date(fetchedAt).toLocaleString()}.${staleNote} ${EXPLANATION}`,
    age,
    isStale,
  }
}

// The current time, updated every minute so ages shown on screen keep up.
export function useNow(): number {
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), MINUTE)
    return () => window.clearInterval(timer)
  }, [])
  return now
}
