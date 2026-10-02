import { useEffect, useRef, useState } from 'react'
import { ListAuthors } from '../../wailsjs/go/app/App'
import { DATE_BUTTON_CLASS } from './DateShiftButtons'
import { errorText } from '../errors'
import { formatPerson, parsePerson, type Person } from '../trailers'

interface CoAuthorPickerProps {
  label: string
  title: string
  disabled: boolean
  onPick: (person: Person) => void
  // The people to pick from. Without it the branch's authors are loaded
  // (ListAuthors) and a typed "Name <email>" can be picked too.
  people?: Person[]
  // Shown as the first entry when set, e.g. "All co-authors".
  allLabel?: string
  onPickAll?: () => void
}

const MAX_SHOWN = 50

// A small button that opens a searchable list of people, for adding (or
// removing) Co-authored-by trailers, like GitHub Desktop's co-author picker.
export default function CoAuthorPicker({
  label,
  title,
  disabled,
  onPick,
  people,
  allLabel,
  onPickAll,
}: CoAuthorPickerProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [authors, setAuthors] = useState<Person[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const boxRef = useRef<HTMLDivElement>(null)

  // The branch's authors, read each time the list opens so new commits count.
  useEffect(() => {
    if (!isOpen || people) {
      return
    }
    let isCurrent = true
    ListAuthors()
      .then((result) => isCurrent && setAuthors(result))
      .catch((error) => isCurrent && setLoadError(errorText(error)))
    return () => {
      isCurrent = false
    }
  }, [isOpen, people])

  // Close on a click outside the list.
  useEffect(() => {
    if (!isOpen) {
      return
    }
    function handlePointerDown(event: PointerEvent) {
      if (boxRef.current && !boxRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('pointerdown', handlePointerDown)
    return () => document.removeEventListener('pointerdown', handlePointerDown)
  }, [isOpen])

  function open() {
    setQuery('')
    setAuthors(null)
    setLoadError(null)
    setIsOpen(true)
  }

  function pick(person: Person) {
    setIsOpen(false)
    onPick(person)
  }

  const candidates = people ?? authors ?? []
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  const matches = candidates.filter((person) => {
    const text = formatPerson(person).toLowerCase()
    return words.every((word) => text.includes(word))
  })
  const typed = people ? null : parsePerson(query)
  const typedIsListed =
    typed !== null && candidates.some((person) => person.email.toLowerCase() === typed.email.toLowerCase())
  const itemClass =
    'block w-full truncate px-3 py-1.5 text-left text-sm text-gray-200 hover:bg-gray-800 focus:bg-gray-800 focus:outline-none'

  return (
    <div ref={boxRef} className="relative">
      <button
        type="button"
        title={title}
        disabled={disabled}
        aria-expanded={isOpen}
        onClick={() => (isOpen ? setIsOpen(false) : open())}
        className={DATE_BUTTON_CLASS}
      >
        {label}
      </button>
      {isOpen && (
        <div
          className="absolute right-0 z-20 mt-1 w-80 max-w-[80vw] rounded-md border border-gray-700 bg-gray-900 py-1 font-sans shadow-lg"
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              event.stopPropagation()
              setIsOpen(false)
            }
          }}
        >
          <input
            autoFocus
            type="text"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key !== 'Enter') {
                return
              }
              // Enter picks a typed person, else the first match.
              event.preventDefault()
              if (typed && !typedIsListed) {
                pick(typed)
              } else if (matches.length > 0) {
                pick(matches[0])
              }
            }}
            aria-label="Search people"
            placeholder={people ? 'Search' : 'Search, or type Name <email>'}
            className="mx-2 mb-1 w-[calc(100%-1rem)] rounded-md border border-gray-700 bg-gray-800 px-2 py-1 text-sm text-gray-100 outline-none focus:border-indigo-500"
          />
          <div className="max-h-64 overflow-y-auto" role="listbox" aria-label={label}>
            {allLabel && onPickAll && words.length === 0 && (
              <button
                type="button"
                className={`${itemClass} italic`}
                onClick={() => {
                  setIsOpen(false)
                  onPickAll()
                }}
              >
                {allLabel}
              </button>
            )}
            {typed && !typedIsListed && (
              <button type="button" className={itemClass} onClick={() => pick(typed)}>
                Add {formatPerson(typed)}
              </button>
            )}
            {matches.slice(0, MAX_SHOWN).map((person) => (
              <button
                key={`${person.email.toLowerCase()} ${person.name}`}
                type="button"
                className={itemClass}
                title={formatPerson(person)}
                onClick={() => pick(person)}
              >
                {person.name} <span className="text-gray-500">&lt;{person.email}&gt;</span>
              </button>
            ))}
            {!people && authors === null && !loadError && (
              <p className="px-3 py-1.5 text-xs text-gray-500">Loading authors…</p>
            )}
            {loadError && <p className="px-3 py-1.5 text-xs text-red-300">{loadError}</p>}
            {(people !== undefined || authors !== null) && matches.length === 0 && !typed && (
              <p className="px-3 py-1.5 text-xs text-gray-500">
                {people ? 'Nobody matches.' : 'Nobody matches. Type Name <email> to add someone else.'}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
