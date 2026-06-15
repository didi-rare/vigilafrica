import { type ComponentProps } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { axe } from 'vitest-axe'

import { Select, type SelectOption } from './Select'

const OPTIONS: SelectOption[] = [
  { value: '', label: 'All Countries' },
  { value: 'Nigeria', label: 'Nigeria' },
  { value: 'Ghana', label: 'Ghana' },
]

function setup(props: Partial<ComponentProps<typeof Select>> = {}) {
  const onChange = vi.fn()
  const utils = render(
    <Select id="country" label="Country" value="" onChange={onChange} options={OPTIONS} {...props} />,
  )
  const combobox = screen.getByRole('combobox', { name: /country/i })
  return { onChange, combobox, ...utils }
}

describe('Select', () => {
  it('shows the selected option label in the trigger', () => {
    setup({ value: 'Ghana' })
    expect(screen.getByRole('combobox', { name: /country/i })).toHaveTextContent('Ghana')
  })

  it('opens on click and lists every option', async () => {
    const user = userEvent.setup()
    const { combobox } = setup()
    expect(combobox).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()

    await user.click(combobox)

    expect(combobox).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getAllByRole('option')).toHaveLength(3)
  })

  it('selects an option by click — fires onChange with its value and closes', async () => {
    const user = userEvent.setup()
    const { combobox, onChange } = setup()

    await user.click(combobox)
    await user.click(screen.getByRole('option', { name: 'Ghana' }))

    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('Ghana')
    expect(combobox).toHaveAttribute('aria-expanded', 'false')
  })

  it('is keyboard operable: ArrowDown opens, arrows move, Enter selects', async () => {
    const user = userEvent.setup()
    const { combobox, onChange } = setup()
    combobox.focus()

    await user.keyboard('{ArrowDown}')
    expect(combobox).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{ArrowDown}{ArrowDown}') // highlight moves to "Ghana"
    await user.keyboard('{Enter}')

    expect(onChange).toHaveBeenCalledWith('Ghana')
    expect(combobox).toHaveAttribute('aria-expanded', 'false')
  })

  it('jumps to an option via type-ahead', async () => {
    const user = userEvent.setup()
    const { combobox, onChange } = setup()

    await user.click(combobox)
    await user.keyboard('g') // "Ghana"
    await user.keyboard('{Enter}')

    expect(onChange).toHaveBeenCalledWith('Ghana')
  })

  it('type-ahead opens from closed, but modifier chords are left alone', async () => {
    const user = userEvent.setup()
    const { combobox } = setup()
    combobox.focus()

    // Ctrl+G must not be hijacked into type-ahead — the shortcut stays free.
    await user.keyboard('{Control>}g{/Control}')
    expect(combobox).toHaveAttribute('aria-expanded', 'false')

    // A plain "g" opens the list and highlights "Ghana".
    await user.keyboard('g')
    expect(combobox).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('option', { name: 'Ghana' })).toHaveClass('is-active')
  })

  it('closes on Escape and restores focus to the trigger', async () => {
    const user = userEvent.setup()
    const { combobox } = setup()

    await user.click(combobox)
    await user.keyboard('{Escape}')

    expect(combobox).toHaveAttribute('aria-expanded', 'false')
    expect(combobox).toHaveFocus()
  })

  it('closes when a pointer-down lands outside', async () => {
    const user = userEvent.setup()
    const { combobox } = setup()

    await user.click(combobox)
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    await user.click(document.body)
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('does not open when disabled', async () => {
    const user = userEvent.setup()
    const { combobox } = setup({ disabled: true })

    expect(combobox).toHaveAttribute('aria-disabled', 'true')
    expect(combobox).toHaveAttribute('tabindex', '-1')

    await user.click(combobox)
    expect(combobox).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('marks the current value as the selected option', async () => {
    const user = userEvent.setup()
    setup({ value: 'Nigeria' })

    await user.click(screen.getByRole('combobox', { name: /country/i }))

    expect(screen.getByRole('option', { name: 'Nigeria' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('option', { name: 'Ghana' })).toHaveAttribute('aria-selected', 'false')
  })

  it('has no axe violations closed or open', async () => {
    const user = userEvent.setup()
    const { container, combobox } = setup()

    expect((await axe(container)).violations).toHaveLength(0)

    await user.click(combobox)
    expect((await axe(container)).violations).toHaveLength(0)
  })
})
