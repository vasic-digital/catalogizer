import { render, screen } from '@testing-library/react'
import { AIMetadataExtractor, AIAutomationRules, AIContentQualityAnalyzer } from '../AIMetadata'

describe('AIMetadata (CAT-001: no canned AI output)', () => {
  it('extractor is unavailable and proposes no metadata', () => {
    render(<AIMetadataExtractor content={{ title: 't' }} onMetadataExtracted={vi.fn()} />)
    expect(screen.getByTestId('ai-extractor-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByText(/confidence/i)).not.toBeInTheDocument()
  })

  it('automation rules shows no rules or run button', () => {
    render(<AIAutomationRules onRuleToggle={vi.fn()} onRuleExecute={vi.fn()} />)
    expect(screen.getByTestId('ai-automation-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('quality analyzer shows no score', () => {
    render(<AIContentQualityAnalyzer content={{ title: 't' }} onQualityImprovement={vi.fn()} />)
    expect(screen.getByTestId('ai-quality-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByText(/\d+%/)).not.toBeInTheDocument()
  })
})
