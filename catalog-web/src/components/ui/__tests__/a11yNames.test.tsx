import { render, screen } from '@testing-library/react'
import axe from 'axe-core'
import { Select } from '../Select'

describe('Select accessible name (CAT-UI-D5D6)', () => {
  const options = [
    { value: 'all', label: 'All Types' },
    { value: 'music', label: 'Music' },
  ]

  it('has an accessible name even when the call site gave none', () => {
    render(<Select value="all" options={options} />)
    expect(screen.getByRole('combobox', { name: /\S/ })).toBeInTheDocument()
  })

  it('names children-style selects too', () => {
    render(
      <Select value="title">
        <option value="created_at">Date Added</option>
        <option value="title">Title</option>
      </Select>
    )
    expect(screen.getByRole('combobox', { name: /\S/ })).toBeInTheDocument()
  })

  it('explicit aria-label wins over the fallback', () => {
    render(<Select aria-label="Filter by type" value="all" options={options} />)
    expect(screen.getByRole('combobox', { name: 'Filter by type' })).toBeInTheDocument()
  })

  it('does not override an aria-labelledby association', () => {
    render(
      <div>
        <span id="lbl">Sort order</span>
        <Select aria-labelledby="lbl" value="all" options={options} />
      </div>
    )
    expect(screen.getByRole('combobox', { name: 'Sort order' })).toBeInTheDocument()
  })

  it('passes the axe select-name rule', async () => {
    const { container } = render(<Select value="all" options={options} />)
    const r = await axe.run(container, { runOnly: { type: 'rule', values: ['select-name'] } })
    expect(r.violations.map(v => v.id)).toEqual([])
  })
})
