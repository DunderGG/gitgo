// Date helpers shared by the edit panels. Commit dates are handled as a
// wall-clock value plus a UTC offset, so they stay in the commit's own time
// zone instead of being converted to this computer's.

export interface WallClockDate {
  // Wall-clock time in the commit's own time zone, in datetime-local input
  // format with seconds: YYYY-MM-DDTHH:mm:ss
  dateLocal: string
  // Time zone offset of the date: +HH:MM or -HH:MM
  offset: string
}

export const WALL_CLOCK_PATTERN = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/

// Quick shifts offered under the date field and in the bulk date panel.
export const DATE_SHIFTS = [
  { label: '−1d', minutes: -24 * 60, title: 'One day earlier' },
  { label: '−1h', minutes: -60, title: 'One hour earlier' },
  { label: '+1h', minutes: 60, title: 'One hour later' },
  { label: '+1d', minutes: 24 * 60, title: 'One day later' },
]

// Splits an RFC 3339 date from the backend into its wall-clock part and
// offset, keeping the commit's own time zone instead of converting to local.
export function splitRfc3339(rfc3339: string): WallClockDate {
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/.exec(rfc3339)
  if (!match) {
    return { dateLocal: '', offset: '+00:00' }
  }
  return { dateLocal: match[1], offset: match[2] === 'Z' ? '+00:00' : match[2] }
}

const pad2 = (value: number) => String(value).padStart(2, '0')

export function formatOffset(totalMinutes: number): string {
  const sign = totalMinutes < 0 ? '-' : '+'
  const absolute = Math.abs(totalMinutes)
  return `${sign}${pad2(Math.floor(absolute / 60))}:${pad2(absolute % 60)}`
}

// Formats a Date's UTC fields as a wall-clock value (YYYY-MM-DDTHH:mm:ss).
function formatWallClockUtc(date: Date): string {
  return (
    `${date.getUTCFullYear()}-${pad2(date.getUTCMonth() + 1)}-${pad2(date.getUTCDate())}` +
    `T${pad2(date.getUTCHours())}:${pad2(date.getUTCMinutes())}:${pad2(date.getUTCSeconds())}`
  )
}

// Shifts a wall-clock value by a number of minutes. The offset is fixed, so
// the arithmetic is done in UTC where there are no daylight saving jumps.
export function shiftWallClock(dateLocal: string, minutes: number): string | null {
  const date = new Date(`${dateLocal}Z`)
  if (!WALL_CLOCK_PATTERN.test(dateLocal) || Number.isNaN(date.getTime())) {
    return null
  }
  return formatWallClockUtc(new Date(date.getTime() + minutes * 60_000))
}

// The current time in this computer's time zone, as `git commit` would record it.
export function nowWallClock(): WallClockDate {
  const now = new Date()
  const offsetMins = -now.getTimezoneOffset()
  return {
    dateLocal: formatWallClockUtc(new Date(now.getTime() + offsetMins * 60_000)),
    offset: formatOffset(offsetMins),
  }
}

// Formats a shift in minutes as e.g. "+1d 3h", "−45m" or "0".
export function formatShift(minutes: number): string {
  if (minutes === 0) {
    return '0'
  }
  const absolute = Math.abs(minutes)
  const parts = [
    [Math.floor(absolute / (24 * 60)), 'd'],
    [Math.floor((absolute % (24 * 60)) / 60), 'h'],
    [absolute % 60, 'm'],
  ]
    .filter(([value]) => value !== 0)
    .map(([value, unit]) => `${value}${unit}`)
  return `${minutes < 0 ? '−' : '+'}${parts.join(' ')}`
}

export function toPreviewDateText({ dateLocal, offset }: WallClockDate): string {
  if (!dateLocal) {
    return ''
  }

  // Parse the wall-clock time as UTC and format it in UTC so the browser's own
  // time zone does not shift it; the commit's offset is shown separately.
  const date = new Date(`${dateLocal}Z`)
  if (Number.isNaN(date.getTime())) {
    return `${dateLocal} (UTC${offset})`
  }

  const wallClock = date.toLocaleString(undefined, {
    timeZone: 'UTC',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
  })
  return `${wallClock} (UTC${offset})`
}
