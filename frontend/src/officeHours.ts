// Office hours: the user's working hours, kept in the settings file
// (Settings.OfficeHours on the Go side). Spreading commits with "Only office
// hours" keeps them within these hours.

export interface OfficeHours {
  // Times of day as HH:MM, start before end.
  start: string
  end: string
  // Working days, 0 for Sunday to 6 for Saturday.
  days: number[]
}

// Same as the Go defaults, used until the settings have loaded.
export const DEFAULT_OFFICE_HOURS: OfficeHours = { start: '09:00', end: '17:00', days: [1, 2, 3, 4, 5] }

// Weekdays in the order they are shown, Monday first, with their index in
// OfficeHours.days.
export const WEEKDAYS = [
  { day: 1, short: 'Mon' },
  { day: 2, short: 'Tue' },
  { day: 3, short: 'Wed' },
  { day: 4, short: 'Thu' },
  { day: 5, short: 'Fri' },
  { day: 6, short: 'Sat' },
  { day: 0, short: 'Sun' },
]

const TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/

// Why hours cannot be saved, or null when they are valid. The Go side checks
// the same rules (parseOfficeHours).
export function officeHoursError(hours: OfficeHours): string | null {
  if (!TIME_PATTERN.test(hours.start) || !TIME_PATTERN.test(hours.end)) {
    return 'Enter a start and end time.'
  }
  if (hours.start >= hours.end) {
    return 'The office hours must start before they end, on the same day.'
  }
  if (hours.days.length === 0) {
    return 'Pick at least one working day.'
  }
  return null
}

// Describes hours as e.g. "Mon–Fri, 09:00–17:00" or "Mon, Wed, Sat–Sun,
// 08:00–12:00": runs of three or more days are shown as a range.
export function formatOfficeHours(hours: OfficeHours): string {
  const runs: string[][] = []
  for (const { day, short } of WEEKDAYS) {
    if (!hours.days.includes(day)) {
      runs.push([])
    } else if (runs.length === 0) {
      runs.push([short])
    } else {
      runs[runs.length - 1].push(short)
    }
  }
  const days = runs
    .filter((run) => run.length > 0)
    .map((run) => (run.length >= 3 ? `${run[0]}–${run[run.length - 1]}` : run.join(', ')))
    .join(', ')
  return `${days}, ${hours.start}–${hours.end}`
}
