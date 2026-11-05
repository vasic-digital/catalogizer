import { render, waitFor } from '@testing-library/react'
import React from 'react'
import axe from 'axe-core'
import { MediaDistributionChart } from '../MediaDistributionChart'

// Real recharts; only ResponsiveContainer is given a fixed size because jsdom has no layout.
vi.mock('recharts', async () => {
  const actual = await vi.importActual<typeof import('recharts')>('recharts')
  return {
    ...actual,
    ResponsiveContainer: ({ children }: any) =>
      React.cloneElement(children, { width: 400, height: 300 }),
  }
})

describe('MediaDistributionChart a11y (CAT-UI-D5D6)', () => {
  it('renders real pie sectors and each has an accessible name (svg-img-alt)', async () => {
    const { container } = render(<MediaDistributionChart data={{ video: 5, audio: 3, image: 2 }} />)
    await waitFor(() => expect(container.querySelectorAll('path.recharts-sector').length).toBe(3), { timeout: 4000 })
    const sectors = container.querySelectorAll('path.recharts-sector')
    const r = await axe.run(container, { runOnly: { type: 'rule', values: ['svg-img-alt'] } })
    expect(r.violations.map(v => v.id)).toEqual([])
    expect(sectors[0].getAttribute('aria-label')).toMatch(/Video/)
  })
})
