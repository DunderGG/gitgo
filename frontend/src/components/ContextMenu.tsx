import { useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'

export interface MenuItem {
  label: string
  // Dimmed text at the right: a shortcut, or the value an item copies.
  hint?: string
  // Shown but does nothing; title says why.
  disabled?: boolean
  title?: string
  onSelect?: () => void
  // Items of a submenu that opens to the side.
  submenu?: MenuEntry[]
}

export type MenuEntry = MenuItem | 'separator'

// Menus keep this far from the window edges.
const EDGE = 4

// Where a menu opens: at x when it fits to the right, otherwise ending at
// flipX (the click point for the context menu, the parent menu's left edge
// for a submenu).
interface Placement {
  x: number
  y: number
  flipX: number
}

function menuItems(menu: HTMLElement): HTMLElement[] {
  return Array.from(menu.querySelectorAll<HTMLElement>(':scope > [role="menuitem"]'))
}

interface MenuListProps {
  label: string
  entries: MenuEntry[]
  placement: Placement
  // Focus the first item once placed: for the context menu, and for a
  // submenu opened from the keyboard or by a click.
  focusFirst: boolean
  onCloseAll: () => void
  // For a submenu: close it and focus its item in the parent menu.
  onBack?: () => void
}

function MenuList({ label, entries, placement, focusFirst, onCloseAll, onBack }: MenuListProps) {
  const menuRef = useRef<HTMLDivElement>(null)
  const [submenu, setSubmenu] = useState<{ index: number; placement: Placement; focusFirst: boolean } | null>(null)
  const items = entries.filter((entry): entry is MenuItem => entry !== 'separator')

  // Placed before the browser paints, so the menu never shows at the wrong
  // spot: kept inside the window, flipped to the left when it does not fit.
  useLayoutEffect(() => {
    const menu = menuRef.current
    if (!menu) {
      return
    }
    const { width, height } = menu.getBoundingClientRect()
    const fitsRight = placement.x + width <= window.innerWidth - EDGE
    menu.style.left = `${Math.max(EDGE, fitsRight ? placement.x : placement.flipX - width)}px`
    menu.style.top = `${Math.max(EDGE, Math.min(placement.y, window.innerHeight - EDGE - height))}px`
  }, [placement])

  useLayoutEffect(() => {
    if (focusFirst && menuRef.current) {
      menuItems(menuRef.current)[0]?.focus()
    }
  }, [focusFirst])

  function openSubmenu(index: number, item: HTMLElement, focusInto: boolean) {
    const menu = menuRef.current
    if (!menu) {
      return
    }
    const menuRect = menu.getBoundingClientRect()
    // Line the submenu's first item up with its item (py-1 is 4px).
    const y = item.getBoundingClientRect().top - 5
    setSubmenu({ index, focusFirst: focusInto, placement: { x: menuRect.right - 2, y, flipX: menuRect.left + 2 } })
  }

  function closeSubmenu() {
    if (submenu && menuRef.current) {
      menuItems(menuRef.current)[submenu.index]?.focus()
    }
    setSubmenu(null)
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    // The menu takes every key while open, as a native menu does, so the
    // app-wide shortcuts (Escape, Ctrl+A, Ctrl+Z) cannot act behind it.
    event.stopPropagation()
    const menu = menuRef.current
    if (!menu) {
      return
    }
    const elements = menuItems(menu)
    const index = elements.indexOf(document.activeElement as HTMLElement)
    const last = elements.length - 1

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        elements[index < 0 || index === last ? 0 : index + 1]?.focus()
        break
      case 'ArrowUp':
        event.preventDefault()
        elements[index <= 0 ? last : index - 1]?.focus()
        break
      case 'Home':
        event.preventDefault()
        elements[0]?.focus()
        break
      case 'End':
        event.preventDefault()
        elements[last]?.focus()
        break
      case 'ArrowRight':
        if (index >= 0 && items[index].submenu && !items[index].disabled) {
          event.preventDefault()
          openSubmenu(index, elements[index], true)
        }
        break
      case 'ArrowLeft':
        if (onBack) {
          event.preventDefault()
          onBack()
        }
        break
      case 'Escape':
        event.preventDefault()
        if (onBack) {
          onBack()
        } else {
          onCloseAll()
        }
        break
      case 'Tab':
        event.preventDefault()
        onCloseAll()
        break
    }
  }

  let itemIndex = -1
  const openItem = submenu ? items[submenu.index] : null

  return (
    <>
      <div
        ref={menuRef}
        role="menu"
        aria-label={label}
        onKeyDown={handleKeyDown}
        className="fixed left-0 top-0 z-50 min-w-44 max-w-sm rounded-md border border-gray-700 bg-gray-900 py-1 text-sm shadow-2xl"
      >
        {entries.map((entry, entryIndex) => {
          if (entry === 'separator') {
            return <div key={`separator-${entryIndex}`} role="separator" className="my-1 border-t border-gray-700" />
          }
          itemIndex += 1
          const index = itemIndex
          const hasSubmenu = entry.submenu !== undefined && !entry.disabled
          return (
            <button
              key={entry.label}
              type="button"
              role="menuitem"
              tabIndex={-1}
              aria-disabled={entry.disabled || undefined}
              aria-haspopup={hasSubmenu ? 'menu' : undefined}
              aria-expanded={hasSubmenu ? submenu?.index === index : undefined}
              title={entry.title}
              // The pointer moves the focus, so hover and the arrow keys
              // highlight the same item.
              onPointerEnter={(event) => {
                event.currentTarget.focus()
                if (!hasSubmenu) {
                  setSubmenu(null)
                } else if (submenu?.index !== index) {
                  openSubmenu(index, event.currentTarget, false)
                }
              }}
              onClick={(event) => {
                if (entry.disabled) {
                  return
                }
                if (hasSubmenu) {
                  openSubmenu(index, event.currentTarget, true)
                  return
                }
                entry.onSelect?.()
                onCloseAll()
              }}
              className="group flex w-full items-center gap-6 px-3 py-1 text-left text-gray-200 outline-none focus:bg-indigo-600 focus:text-white aria-disabled:cursor-default aria-disabled:text-gray-500 aria-disabled:focus:bg-gray-800 aria-disabled:focus:text-gray-500 [&[aria-expanded=true]:not(:focus)]:bg-gray-800"
            >
              <span className="flex-1 whitespace-nowrap">{entry.label}</span>
              {entry.hint && (
                <span className="min-w-0 truncate text-xs text-gray-500 group-focus:text-indigo-100 group-aria-disabled:!text-gray-500">
                  {entry.hint}
                </span>
              )}
              {entry.submenu && (
                <span aria-hidden="true" className="-mr-1 text-gray-400 group-focus:text-white">
                  ›
                </span>
              )}
            </button>
          )
        })}
      </div>
      {submenu && openItem?.submenu && (
        <MenuList
          key={submenu.index}
          label={openItem.label}
          entries={openItem.submenu}
          placement={submenu.placement}
          focusFirst={submenu.focusFirst}
          onCloseAll={onCloseAll}
          onBack={closeSubmenu}
        />
      )}
    </>
  )
}

interface ContextMenuProps {
  // Where the menu opens, in window coordinates.
  x: number
  y: number
  label: string
  entries: MenuEntry[]
  // Must keep its identity between renders (useCallback).
  onClose: () => void
}

// ContextMenu is a right-click menu at (x, y), with keyboard support as in a
// native menu: ↑ ↓ Home End move, → opens a submenu and ← closes it, Enter
// runs an item, Escape and Tab close. A click elsewhere, scrolling, resizing
// or leaving the window closes it too. Focus goes back to where it was (the
// right-clicked row) unless the click that closed it focused something else.
export default function ContextMenu({ x, y, label, entries, onClose }: ContextMenuProps) {
  const rootRef = useRef<HTMLDivElement>(null)
  const placement = useMemo(() => ({ x, y, flipX: x }), [x, y])
  // Read on the first render: the menu's own layout effects focus its first
  // item before this component's effects run.
  const [previousFocus] = useState(() => (document.activeElement instanceof HTMLElement ? document.activeElement : null))

  useLayoutEffect(() => {
    const root = rootRef.current
    function handleOutside(event: Event) {
      if (!root?.contains(event.target as Node)) {
        onClose()
      }
    }
    document.addEventListener('pointerdown', handleOutside, true)
    window.addEventListener('scroll', handleOutside, true)
    window.addEventListener('resize', onClose)
    window.addEventListener('blur', onClose)
    return () => {
      document.removeEventListener('pointerdown', handleOutside, true)
      window.removeEventListener('scroll', handleOutside, true)
      window.removeEventListener('resize', onClose)
      window.removeEventListener('blur', onClose)
      const active = document.activeElement
      if (!active || active === document.body || root?.contains(active)) {
        previousFocus?.focus()
      }
    }
  }, [onClose, previousFocus])

  return createPortal(
    <div ref={rootRef} onContextMenu={(event) => event.preventDefault()}>
      <MenuList label={label} entries={entries} placement={placement} focusFirst onCloseAll={onClose} />
    </div>,
    document.body,
  )
}
