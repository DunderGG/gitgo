// Commit message trailers ("Key: value" lines at the end of a message). The
// Go side finds, adds and removes them (git/trailers.go); this file only names
// the keys the panels offer and formats the people in them.

import type { app } from '../wailsjs/go/models'

export const CO_AUTHORED_BY = 'Co-authored-by'
export const SIGNED_OFF_BY = 'Signed-off-by'

export interface Person {
  name: string
  email: string
}

// "Name <email>", the value of a Co-authored-by or Signed-off-by trailer.
export function formatPerson(person: Person): string {
  return `${person.name} <${person.email}>`
}

// Reads "Name <email>" as typed in the co-author picker, or null when the
// text is not in that form. Mirrors ParseIdentity in git/trailers.go, but
// needs both a name and an email.
export function parsePerson(text: string): Person | null {
  const match = /^\s*([^<>]*?)\s*<([^<>\s]+)>\s*$/.exec(text)
  if (!match || !match[1].trim()) {
    return null
  }
  return { name: match[1].trim(), email: match[2] }
}

// "Key: value", as the line appears in the message.
export function formatTrailer(trailer: app.Trailer): string {
  return `${trailer.key}: ${trailer.value}`
}

// The trailers a change removes from a commit and the ones it adds, from its
// PreviewTrailers entry.
export function trailerDiff(preview: app.TrailerPreview): { removed: string[]; added: string[] } {
  const before = preview.before.map(formatTrailer)
  const after = preview.after.map(formatTrailer)
  return {
    removed: before.filter((line) => !after.includes(line)),
    added: after.filter((line) => !before.includes(line)),
  }
}
