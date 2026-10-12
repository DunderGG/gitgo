// Searching the commit list. A search is words separated by spaces, and a
// commit must match every word. A plain word is looked for in the message
// (subject and body), the author's name and email, the start of the hash and,
// when it looks like a date, the date. A qualifier limits a word to one field:
//
//   author:jane      the author's name or email contains "jane"
//   message:fix      the subject or body contains "fix"
//   hash:a1b2c3      the hash starts with a1b2c3
//   date:2026-10     the date is in October 2026 (a year, month or day)
//   since:2026-10-01 on or after that day (or the start of that month or year)
//   until:2026-10-12 on or before that day (or the end of that month or year)
//
// Quotes keep spaces in a word: "fix login" or author:"Jane Doe". Matching
// ignores case. Dates are calendar days in this computer's time zone, as the
// list shows them.

import type { CommitSummary } from './store/repoStore'

// How many commits the list loads (DefaultLogDepth in git/log.go). A list of
// this length may have older commits the search cannot see.
export const LOG_DEPTH = 100

const qualifiers = ['author', 'message', 'hash', 'date', 'since', 'until'] as const
type Qualifier = (typeof qualifiers)[number]

function isQualifier(key: string): key is Qualifier {
  return (qualifiers as readonly string[]).includes(key)
}

type Matcher = (commit: CommitSummary) => boolean

export interface CommitSearch {
  // False for an empty search, which shows every commit.
  isActive: boolean
  // Words that could not be used, such as a qualifier with a date that does
  // not exist, explained for the user. They are left out of the search.
  problems: string[]
  matches: Matcher
}

// A year, month or day from a date word, as the start and end (exclusive) of
// that period in local time.
interface DateRange {
  start: number
  end: number
}

const datePattern = /^(\d{4})(?:-(\d{1,2})(?:-(\d{1,2}))?)?$/

function parseDateRange(text: string): DateRange | null {
  const match = datePattern.exec(text)
  if (!match) {
    return null
  }
  const year = Number(match[1])
  const month = match[2] === undefined ? null : Number(match[2]) - 1
  const day = match[3] === undefined ? null : Number(match[3])

  if (month === null) {
    return { start: new Date(year, 0, 1).getTime(), end: new Date(year + 1, 0, 1).getTime() }
  }
  if (month < 0 || month > 11) {
    return null
  }
  if (day === null) {
    return { start: new Date(year, month, 1).getTime(), end: new Date(year, month + 1, 1).getTime() }
  }
  const start = new Date(year, month, day)
  // Date rolls 2026-02-30 over into March; such a day does not exist.
  if (start.getMonth() !== month || start.getDate() !== day) {
    return null
  }
  return { start: start.getTime(), end: new Date(year, month, day + 1).getTime() }
}

function commitTime(commit: CommitSummary): number {
  return new Date(commit.date).getTime()
}

function textMatcher(value: string): Matcher {
  const lower = value.toLowerCase()
  // Like git, a hash is only matched from four characters on, so short words
  // do not pick up commits by their hash.
  const isHashPrefix = /^[0-9a-f]{4,}$/.test(lower)
  const range = parseDateRange(value)
  return (commit) =>
    commit.message.toLowerCase().includes(lower) ||
    commit.body.toLowerCase().includes(lower) ||
    commit.author.toLowerCase().includes(lower) ||
    commit.authorEmail.toLowerCase().includes(lower) ||
    (isHashPrefix && commit.hash.startsWith(lower)) ||
    (range !== null && commitTime(commit) >= range.start && commitTime(commit) < range.end)
}

function qualifierMatcher(key: Qualifier, value: string): Matcher | string {
  const lower = value.toLowerCase()
  switch (key) {
    case 'author':
      return (commit) => commit.author.toLowerCase().includes(lower) || commit.authorEmail.toLowerCase().includes(lower)
    case 'message':
      return (commit) => commit.message.toLowerCase().includes(lower) || commit.body.toLowerCase().includes(lower)
    case 'hash':
      return (commit) => commit.hash.startsWith(lower)
    case 'date':
    case 'since':
    case 'until': {
      const range = parseDateRange(value)
      if (!range) {
        return `${key}:${value} is not a date. Use a year, month or day, such as 2026, 2026-10 or 2026-10-12.`
      }
      if (key === 'since') {
        return (commit) => commitTime(commit) >= range.start
      }
      if (key === 'until') {
        return (commit) => commitTime(commit) < range.end
      }
      return (commit) => commitTime(commit) >= range.start && commitTime(commit) < range.end
    }
  }
}

// An optional key: before the value, then a quoted value (the closing quote
// may still be missing while typing) or a run of non-space characters.
const wordPattern = /(?:([A-Za-z]+):)?(?:"([^"]*)"?|(\S+))/g

export function parseCommitSearch(query: string): CommitSearch {
  const matchers: Matcher[] = []
  const problems: string[] = []

  for (const match of query.matchAll(wordPattern)) {
    const key = match[1]?.toLowerCase()
    const value = match[2] ?? match[3] ?? ''
    // "author:" on its own is a qualifier still being typed. With no value
    // after it, the pattern reads it as a plain word.
    const isUnfinished = key === undefined && value.endsWith(':') && isQualifier(value.slice(0, -1).toLowerCase())
    if (isUnfinished || (key !== undefined && isQualifier(key) && value === '')) {
      continue
    }
    if (key !== undefined && isQualifier(key)) {
      const matcher = qualifierMatcher(key, value)
      if (typeof matcher === 'string') {
        problems.push(matcher)
      } else {
        matchers.push(matcher)
      }
      continue
    }
    // An unknown key, as in "fix:" or a URL, is part of the text.
    const text = key !== undefined ? `${match[1]}:${value}` : value
    if (text !== '') {
      matchers.push(textMatcher(text))
    }
  }

  return {
    isActive: matchers.length > 0,
    problems,
    matches: (commit) => matchers.every((matcher) => matcher(commit)),
  }
}

// The commits the list shows for query, in list order.
export function filterCommits(commits: CommitSummary[], query: string): CommitSummary[] {
  const search = parseCommitSearch(query)
  return search.isActive ? commits.filter(search.matches) : commits
}
