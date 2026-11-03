import { render, screen } from '@testing-library/react'
import CollectionAutomation from '../CollectionAutomation'

describe('CollectionAutomation (CAT-001: no invented rules)', () => {
  it('renders heading and description', () => {
    render(<CollectionAutomation />)
    expect(screen.getByText('Automation Rules')).toBeInTheDocument()
    expect(screen.getByText('Create workflows to automatically manage your collections')).toBeInTheDocument()
  })

  it('says automation is unavailable and offers no working create button', () => {
    render(<CollectionAutomation />)
    expect(screen.getByTestId('automation-unavailable')).toBeInTheDocument()
    expect(screen.getByText('Automation unavailable')).toBeInTheDocument()
    expect(screen.getByText('Create Rule').closest('button')).toBeDisabled()
  })

  it('shows none of the former hard-coded sample rules or statistics', () => {
    render(<CollectionAutomation />)
    for (const invented of [/Auto-Tag New Movies/, /Total Runs/, /Success Rate/, /Total Rules/]) {
      expect(screen.queryByText(invented)).not.toBeInTheDocument()
    }
  })
})
