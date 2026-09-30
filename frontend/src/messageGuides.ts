// Commit message guides: a subject ruler in the message field and hints below
// it, for the usual layout of a subject line, a blank line, then a wrapped
// body. The Go side keeps the columns in the settings file
// (Settings.SubjectGuide and Settings.BodyGuide).

export interface MessageGuides {
  // Column of the subject line ruler and length check; 0 turns both off.
  subject: number
  // Longest body line before a hint; 0 turns the check off.
  body: number
}

// Same as the Go defaults, used until the settings have loaded.
export const DEFAULT_MESSAGE_GUIDES: MessageGuides = { subject: 50, body: 72 }

// Largest column SetMessageGuides accepts (maxGuideColumn in Go).
export const MAX_GUIDE_COLUMN = 200

// Length in characters rather than UTF-16 code units, so an emoji counts once.
export function lineLength(line: string): number {
  return [...line].length
}

// Hints about a message that does not follow the guides, or none. A 0 column
// turns off the matching length check.
export function messageGuideHints(message: string, guides: MessageGuides): string[] {
  const lines = message.split('\n')
  const hints: string[] = []

  const subjectLength = lineLength(lines[0])
  if (guides.subject > 0 && subjectLength > guides.subject) {
    hints.push(`The subject is ${subjectLength} characters; ${guides.subject} or fewer reads best in logs.`)
  }
  if (lines.length > 1 && lines[1].trim() !== '') {
    hints.push('Leave the second line blank, so git and other tools can tell the subject from the body.')
  }
  if (guides.body > 0) {
    const longLines = lines
      .map((line, index) => ({ number: index + 1, length: lineLength(line) }))
      .filter((line) => line.number > 1 && line.length > guides.body)
      .map((line) => line.number)
    if (longLines.length === 1) {
      hints.push(`Line ${longLines[0]} is longer than ${guides.body} characters.`)
    } else if (longLines.length > 1) {
      hints.push(`Lines ${longLines.join(', ')} are longer than ${guides.body} characters.`)
    }
  }
  return hints
}
