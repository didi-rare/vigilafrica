// Accessible, themed single-select dropdown (WAI-ARIA "select-only combobox"
// pattern). Replaces native <select> on the dashboard filters so the open list
// is fully styleable / on-brand in every browser — the OS-native <select> popup
// cannot be themed. feat-filter-dropdown.
//
// Focus stays on the role="combobox" trigger throughout; the highlighted option
// is tracked with aria-activedescendant (no roving tabindex). Options are not
// in the tab order. Closes on Escape (focus returns to trigger), Tab, and
// outside pointer-down.
import { useCallback, useEffect, useId, useRef, useState } from 'react'
import { ChevronDown } from 'lucide-react'

import './Select.css'

export type SelectOption = { value: string; label: string }

interface SelectProps {
  /** Stable id for the combobox element (used to wire the listbox + a11y ids). */
  id: string
  /** Currently-selected option value (matched against options[].value). */
  value: string
  /** Called with the chosen option's value when the selection changes. */
  onChange: (value: string) => void
  options: SelectOption[]
  disabled?: boolean
  /** Field label, announced before the value (e.g. "Country" → "Country, All Countries"). */
  label: string
}

const TYPEAHEAD_RESET_MS = 500

export function Select({ id, value, onChange, options, disabled = false, label }: SelectProps) {
  const [open, setOpen] = useState(false)
  const selectedIndex = Math.max(0, options.findIndex((o) => o.value === value))
  const [activeIndex, setActiveIndex] = useState(selectedIndex)

  const rootRef = useRef<HTMLDivElement>(null)
  const comboRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const typeahead = useRef<{ buffer: string; timer: number | undefined }>({ buffer: '', timer: undefined })

  // Generated, collision-proof id prefix for the label / value / option nodes.
  const uid = useId()
  const labelId = `${uid}-label`
  const valueId = `${uid}-value`
  const listboxId = `${uid}-listbox`
  const optionId = (i: number) => `${uid}-opt-${i}`

  const selected = options[selectedIndex]

  const close = useCallback((refocus = true) => {
    setOpen(false)
    if (refocus) comboRef.current?.focus()
  }, [])

  const openList = useCallback((index: number) => {
    if (disabled) return
    setActiveIndex(index)
    setOpen(true)
  }, [disabled])

  const choose = useCallback((index: number) => {
    const opt = options[index]
    if (opt) onChange(opt.value)
    close()
  }, [options, onChange, close])

  // Keep the highlighted option scrolled into view during keyboard navigation
  // (the state list can overflow the panel's max-height).
  useEffect(() => {
    if (!open) return
    const activeEl = listRef.current?.querySelector<HTMLElement>(`#${CSS.escape(optionId(activeIndex))}`)
    // scrollIntoView is absent in jsdom and unsupported on very old engines —
    // optional-call so the keyboard-scroll niceness degrades silently.
    activeEl?.scrollIntoView?.({ block: 'nearest' })
    // optionId is derived from uid (stable); only activeIndex/open drive this.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, activeIndex])

  // Close on an outside pointer-down (without stealing focus back to the trigger).
  useEffect(() => {
    if (!open) return
    function onPointerDown(e: PointerEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) close(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open, close])

  function runTypeahead(char: string) {
    const t = typeahead.current
    window.clearTimeout(t.timer)
    t.buffer += char.toLowerCase()
    t.timer = window.setTimeout(() => { t.buffer = '' }, TYPEAHEAD_RESET_MS)
    const match = options.findIndex((o) => o.label.toLowerCase().startsWith(t.buffer))
    if (match >= 0) {
      setActiveIndex(match)
      if (!open) openList(match)
    }
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (disabled) return
    const { key } = e
    if (!open) {
      if (key === 'ArrowDown' || key === 'Enter' || key === ' ') { e.preventDefault(); openList(selectedIndex) }
      else if (key === 'ArrowUp') { e.preventDefault(); openList(selectedIndex) }
      else if (key === 'Home') { e.preventDefault(); openList(0) }
      else if (key === 'End') { e.preventDefault(); openList(options.length - 1) }
      else if (key.length === 1 && /\S/.test(key)) { e.preventDefault(); runTypeahead(key) }
      return
    }
    switch (key) {
      case 'ArrowDown': e.preventDefault(); setActiveIndex((i) => Math.min(i + 1, options.length - 1)); break
      case 'ArrowUp': e.preventDefault(); setActiveIndex((i) => Math.max(i - 1, 0)); break
      case 'Home': e.preventDefault(); setActiveIndex(0); break
      case 'End': e.preventDefault(); setActiveIndex(options.length - 1); break
      case 'Enter':
      case ' ': e.preventDefault(); choose(activeIndex); break
      case 'Escape': e.preventDefault(); close(); break
      case 'Tab': close(false); break
      default:
        if (key.length === 1 && /\S/.test(key)) { e.preventDefault(); runTypeahead(key) }
    }
  }

  return (
    <div ref={rootRef} className="select">
      <span id={labelId} className="select__label">{label}</span>
      <div
        ref={comboRef}
        id={id}
        role="combobox"
        tabIndex={disabled ? -1 : 0}
        // Only reference the listbox while it's mounted (open) — a dangling
        // aria-controls id would be an aria-valid-attr-value violation.
        aria-controls={open ? listboxId : undefined}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-labelledby={`${labelId} ${valueId}`}
        aria-activedescendant={open ? optionId(activeIndex) : undefined}
        aria-disabled={disabled || undefined}
        className="select__trigger"
        onClick={() => (open ? close() : openList(selectedIndex))}
        onKeyDown={onKeyDown}
      >
        <span id={valueId} className="select__value">{selected?.label ?? ''}</span>
        <ChevronDown size={16} className="select__chevron" aria-hidden="true" />
      </div>

      {open && (
        <ul
          ref={listRef}
          id={listboxId}
          role="listbox"
          aria-labelledby={labelId}
          className="select__list"
        >
          {options.map((opt, i) => (
            <li
              key={opt.value}
              id={optionId(i)}
              role="option"
              aria-selected={opt.value === value}
              className={
                'select__option' +
                (i === activeIndex ? ' is-active' : '') +
                (opt.value === value ? ' is-selected' : '')
              }
              // Keep focus on the combobox so blur/activedescendant stay correct.
              onPointerDown={(e) => e.preventDefault()}
              onClick={() => choose(i)}
              onMouseEnter={() => setActiveIndex(i)}
            >
              {opt.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
