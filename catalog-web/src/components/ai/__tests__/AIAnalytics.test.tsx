import { render, screen } from '@testing-library/react'
import {
  AIUserBehaviorAnalytics,
  AIPredictions,
  AISmartOrganization,
} from '../AIAnalytics'

describe('AIAnalytics (CAT-001: no canned analysis)', () => {
  it('behavior analytics says it is unavailable and shows no patterns', () => {
    const onAction = vi.fn()
    render(<AIUserBehaviorAnalytics userId="u1" onActionImplement={onAction} />)
    expect(screen.getByText('User Behavior Analytics')).toBeInTheDocument()
    expect(screen.getByTestId('ai-behavior-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByText('Evening Entertainment Browsing')).not.toBeInTheDocument()
    expect(document.querySelector('.animate-spin')).toBeNull()
    expect(onAction).not.toHaveBeenCalled()
  })

  it('predictions says it is unavailable and shows no forecasts', () => {
    render(<AIPredictions onPredictionAction={vi.fn()} />)
    expect(screen.getByTestId('ai-predictions-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByText(/confidence/i)).not.toBeInTheDocument()
  })

  it('smart organization says it is unavailable and offers no apply action', () => {
    render(<AISmartOrganization collections={[]} onSuggestionApply={vi.fn()} />)
    expect(screen.getByTestId('ai-organization-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
