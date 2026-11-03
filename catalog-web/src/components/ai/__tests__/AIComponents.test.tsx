import { render, screen } from '@testing-library/react'
import { AICollectionSuggestions, AINaturalSearch, AIContentCategorizer } from '../AIComponents'

describe('AIComponents (CAT-001: no canned AI output)', () => {
  it('suggestions panel is unavailable and invokes nothing', () => {
    const accept = vi.fn()
    render(<AICollectionSuggestions onSuggestionAccept={accept} />)
    expect(screen.getByTestId('ai-suggestions-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(accept).not.toHaveBeenCalled()
  })

  it('natural search offers no input that pretends to work', () => {
    const search = vi.fn()
    render(<AINaturalSearch onSearch={search} />)
    expect(screen.getByTestId('ai-nl-search-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(search).not.toHaveBeenCalled()
  })

  it('categorizer shows no category or confidence', () => {
    render(<AIContentCategorizer item={{ title: 'x' }} onCategorizationComplete={vi.fn()} />)
    expect(screen.getByTestId('ai-categorizer-unavailable')).toHaveTextContent('Unavailable')
    expect(screen.queryByText(/confidence/i)).not.toBeInTheDocument()
  })
})
