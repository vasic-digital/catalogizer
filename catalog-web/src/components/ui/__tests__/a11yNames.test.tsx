import { render, screen } from '@testing-library/react'
import axe from 'axe-core'
import { Select } from '../Select'

const options = [
  { value: 'all', label: 'All Types' },
  { value: 'music', label: 'Music' },
]

describe('Select accessible name (CAT-UI-D5D6)', () => {
  it('does not invent a name from the current value', () => {
    render(<Select value="all" options={options} />)
    expect(screen.getByRole('combobox')).toHaveAccessibleName('')
  })

  it('explicit aria-label names the control', () => {
    render(<Select aria-label="Filter by type" value="all" options={options} />)
    expect(screen.getByRole('combobox', { name: 'Filter by type' })).toBeInTheDocument()
  })

  it('an associated <label htmlFor> provides the name', () => {
    render(
      <div>
        <label htmlFor="sort-by">Sort By</label>
        <Select id="sort-by" value="all" options={options} />
      </div>
    )
    expect(screen.getByRole('combobox', { name: 'Sort By' })).toBeInTheDocument()
  })

  it('a wrapping <label> provides the name and is not overridden by the current value', () => {
    render(
      <label>
        <span>Logic:</span>
        <Select value="all" options={options} />
      </label>
    )
    const name = screen.getByRole('combobox').getAttribute('aria-label')
    expect(name).toBeNull()
    expect(screen.getByRole('combobox', { name: /Logic/ })).toBeInTheDocument()
  })

  it('aria-labelledby is honoured', () => {
    render(
      <div>
        <span id="lbl">Sort order</span>
        <Select aria-labelledby="lbl" value="all" options={options} />
      </div>
    )
    expect(screen.getByRole('combobox', { name: 'Sort order' })).toBeInTheDocument()
  })

  it('axe select-name passes for labelled and fails for an unlabelled select (oracle works)', async () => {
    const good = render(<Select aria-label="X" value="all" options={options} />)
    expect((await axe.run(good.container, { runOnly: { type: 'rule', values: ['select-name'] } })).violations).toHaveLength(0)
    good.unmount()
    const bad = render(<Select value="all" options={options} />)
    expect((await axe.run(bad.container, { runOnly: { type: 'rule', values: ['select-name'] } })).violations).toHaveLength(1)
  })
})
