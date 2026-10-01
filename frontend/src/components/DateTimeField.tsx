import { formatOffset, type WallClockDate } from '../dates'

// UTC offsets in use around the world. Offsets passed in extraOffsets (such
// as a commit's original one) are added when they are not among these.
const COMMON_OFFSETS = [
  '-12:00', '-11:00', '-10:00', '-09:30', '-09:00', '-08:00', '-07:00', '-06:00',
  '-05:00', '-04:00', '-03:30', '-03:00', '-02:00', '-01:00', '+00:00', '+01:00',
  '+02:00', '+03:00', '+03:30', '+04:00', '+04:30', '+05:00', '+05:30', '+05:45',
  '+06:00', '+06:30', '+07:00', '+08:00', '+08:45', '+09:00', '+09:30', '+10:00',
  '+10:30', '+11:00', '+12:00', '+12:45', '+13:00', '+14:00',
]

// datetime-local inputs leave out ":00" seconds in their value, so add them back.
function normalizeWallClock(inputValue: string): string {
  return inputValue.length === 16 ? `${inputValue}:00` : inputValue.slice(0, 19)
}

function offsetMinutes(offset: string): number {
  const sign = offset.startsWith('-') ? -1 : 1
  const [hours, minutes] = offset.slice(1).split(':').map(Number)
  return sign * (hours * 60 + minutes)
}

// Offset of this computer's time zone at the given wall-clock time, used to
// label the matching option.
function localOffsetAt(dateLocal: string): string | null {
  const date = new Date(dateLocal)
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return formatOffset(-date.getTimezoneOffset())
}

const FIELD_CLASS =
  'rounded-md border border-gray-700 bg-gray-800 py-2 text-sm text-gray-100 outline-none transition focus:border-indigo-500 disabled:cursor-not-allowed disabled:opacity-60'

interface DateTimeFieldProps {
  value: WallClockDate
  onChange: (value: WallClockDate) => void
  disabled: boolean
  // Offsets to offer besides the common ones.
  extraOffsets?: string[]
  // Accessible name of the date input, e.g. "First date".
  label?: string
}

// A date and time to the second, with its own UTC offset, as used by the
// edit panels. Changing the offset keeps the wall-clock time.
export default function DateTimeField({ value, onChange, disabled, extraOffsets = [], label }: DateTimeFieldProps) {
  const offsetOptions = [...COMMON_OFFSETS]
  for (const offset of [...extraOffsets, value.offset]) {
    if (offset && !offsetOptions.includes(offset)) {
      offsetOptions.push(offset)
    }
  }
  offsetOptions.sort((left, right) => offsetMinutes(left) - offsetMinutes(right))
  const localOffset = localOffsetAt(value.dateLocal)

  return (
    <div className="mt-1 flex flex-wrap gap-2">
      <input
        type="datetime-local"
        step={1}
        value={value.dateLocal}
        onChange={(e) => onChange({ ...value, dateLocal: normalizeWallClock(e.target.value) })}
        disabled={disabled}
        aria-label={label}
        className={`min-w-[13rem] flex-1 px-3 ${FIELD_CLASS}`}
      />
      <select
        value={value.offset}
        onChange={(e) => onChange({ ...value, offset: e.target.value })}
        disabled={disabled}
        aria-label={label ? `${label} time zone offset` : 'Time zone offset'}
        className={`flex-1 px-2 ${FIELD_CLASS}`}
      >
        {offsetOptions.map((offset) => (
          <option key={offset} value={offset}>
            UTC{offset}
            {offset === localOffset ? ' (local)' : ''}
          </option>
        ))}
      </select>
    </div>
  )
}
