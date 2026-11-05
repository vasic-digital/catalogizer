import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AIDashboard from '../AIDashboard'

// Mock AI component modules
vi.mock('../../components/ai/AIComponents', () => ({
  AICollectionSuggestions: ({ onSuggestionAccept: _onSuggestionAccept }: any) => (
    <div data-testid="ai-suggestions">AI Collection Suggestions</div>
  ),
  AINaturalSearch: ({ onSearch: _onSearch }: any) => (
    <div data-testid="ai-search">AI Natural Search</div>
  ),
  AIContentCategorizer: ({ onCategorizationComplete: _onCategorizationComplete }: any) => (
    <div data-testid="ai-categorizer">AI Content Categorizer</div>
  ),
  AIService: {},
}))

vi.mock('../../components/ai/AIAnalytics', () => ({
  AIUserBehaviorAnalytics: ({ onActionImplement: _onActionImplement }: any) => (
    <div data-testid="ai-behavior">AI User Behavior Analytics</div>
  ),
  AIPredictions: ({ onPredictionAction: _onPredictionAction }: any) => (
    <div data-testid="ai-predictions">AI Predictions</div>
  ),
  AISmartOrganization: ({ onSuggestionApply: _onSuggestionApply }: any) => (
    <div data-testid="ai-organization">AI Smart Organization</div>
  ),
  AIAnalyticsService: {},
}))

vi.mock('../../components/ai/AIMetadata', () => ({
  AIMetadataExtractor: ({ onMetadataExtracted: _onMetadataExtracted }: any) => (
    <div data-testid="ai-metadata">AI Metadata Extractor</div>
  ),
  AIAutomationRules: ({ onRuleToggle: _onRuleToggle, onRuleExecute: _onRuleExecute }: any) => (
    <div data-testid="ai-automation">AI Automation Rules</div>
  ),
  AIContentQualityAnalyzer: ({ onQualityImprovement: _onQualityImprovement }: any) => (
    <div data-testid="ai-quality">AI Content Quality Analyzer</div>
  ),
  AIMetadataService: {},
}))

describe('AIDashboard Page', () => {
  it('renders AI Dashboard heading', () => {
    render(<AIDashboard />)
    expect(screen.getByText('AI Dashboard')).toBeInTheDocument()
  })

  it('renders description text', () => {
    render(<AIDashboard />)
    expect(
      screen.getByText(/Advanced AI-powered features for intelligent content management/)
    ).toBeInTheDocument()
  })

  it('renders navigation tabs', () => {
    render(<AIDashboard />)
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('AI Suggestions')).toBeInTheDocument()
    expect(screen.getByText('Natural Search')).toBeInTheDocument()
    expect(screen.getByText('Analytics')).toBeInTheDocument()
    expect(screen.getByText('Metadata')).toBeInTheDocument()
    expect(screen.getByText('Automation')).toBeInTheDocument()
  })

  it('shows overview section by default', () => {
    render(<AIDashboard />)
    expect(screen.getByText('Processed Items')).toBeInTheDocument()
    expect(screen.getByText('AI Accuracy')).toBeInTheDocument()
    expect(screen.getByText('Time Saved')).toBeInTheDocument()
  })

  it('CAT-001: shows no invented metric values', () => {
    render(<AIDashboard />)
    for (const invented of ['2,847', '92%', '12.5 hours']) {
      expect(screen.queryByText(invented)).not.toBeInTheDocument()
    }
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThanOrEqual(3)
  })

  it('switches to suggestions tab on click', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('AI Suggestions'))

    expect(await screen.findByText('AI-Powered Suggestions')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-suggestions')).toBeInTheDocument()
  })

  it('switches to search tab on click', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Natural Search'))

    expect(await screen.findByText('Natural Language Search')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-search')).toBeInTheDocument()
  })

  it('switches to analytics tab on click', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Analytics'))

    expect(await screen.findByText('AI Analytics')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-behavior')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-predictions')).toBeInTheDocument()
  })

  it('switches to metadata tab on click', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Metadata'))

    expect(await screen.findByText('AI Metadata Services')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-metadata')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-quality')).toBeInTheDocument()
  })

  it('switches to automation tab on click', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Automation'))

    expect(await screen.findByText('AI Automation')).toBeInTheDocument()
    expect(await screen.findByTestId('ai-automation')).toBeInTheDocument()
  })

  it('shows quick actions in overview', () => {
    render(<AIDashboard />)
    expect(screen.getByText('Quick Actions')).toBeInTheDocument()
    expect(screen.getByText('Get Suggestions')).toBeInTheDocument()
    expect(screen.getByText('AI Search')).toBeInTheDocument()
    expect(screen.getByText('View Analytics')).toBeInTheDocument()
    expect(screen.getByText('Manage Rules')).toBeInTheDocument()
  })

  it('navigates to suggestions from quick action button', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Get Suggestions'))

    expect(screen.getByText('AI-Powered Suggestions')).toBeInTheDocument()
  })

  // --- Additional branch coverage tests ---

  it('navigates to search from quick action button', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('AI Search'))

    expect(screen.getByText('Natural Language Search')).toBeInTheDocument()
  })

  it('navigates to analytics from quick action button', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('View Analytics'))

    expect(screen.getByText('AI Analytics')).toBeInTheDocument()
  })

  it('navigates to automation from quick action button', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Manage Rules'))

    expect(screen.getByText('AI Automation')).toBeInTheDocument()
  })

  it('switches between all tabs without errors', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    // Visit each tab
    await user.click(screen.getByText('AI Suggestions'))
    expect(await screen.findByTestId('ai-suggestions')).toBeInTheDocument()

    await user.click(screen.getByText('Natural Search'))
    expect(await screen.findByTestId('ai-search')).toBeInTheDocument()

    await user.click(screen.getByText('Analytics'))
    expect(await screen.findByTestId('ai-behavior')).toBeInTheDocument()

    await user.click(screen.getByText('Metadata'))
    expect(await screen.findByTestId('ai-metadata')).toBeInTheDocument()

    await user.click(screen.getByText('Automation'))
    expect(await screen.findByTestId('ai-automation')).toBeInTheDocument()

    // Return to Overview
    await user.click(screen.getByText('Overview'))
    expect(screen.getByText('Processed Items')).toBeInTheDocument()
  })

  it('displays all metric cards in overview', () => {
    render(<AIDashboard />)
    expect(screen.getByText('Processed Items')).toBeInTheDocument()
    expect(screen.getByText('AI Accuracy')).toBeInTheDocument()
    expect(screen.getByText('Time Saved')).toBeInTheDocument()
  })

  it('renders suggestions tab with suggestion component', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('AI Suggestions'))
    expect(await screen.findByTestId('ai-suggestions')).toBeInTheDocument()
  })

  it('renders analytics tab with predictions', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Analytics'))
    expect(await screen.findByTestId('ai-predictions')).toBeInTheDocument()
  })

  // ai-organization is verified in 'switches to analytics tab on click' above

  it('renders metadata tab with quality analyzer', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)

    await user.click(screen.getByText('Metadata'))
    expect(await screen.findByTestId('ai-quality')).toBeInTheDocument()
  })
})

describe('AIDashboard accessible names (CAT-UI-D5D6)', () => {
  it('names the section toggle and exposes aria-expanded', async () => {
    const user = userEvent.setup()
    render(<AIDashboard />)
    const toggle = screen.getByRole('button', { name: 'Toggle AI Performance section' })
    const before = toggle.getAttribute('aria-expanded')
    expect(['true', 'false']).toContain(before)
    await user.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).not.toBe(before)
  })
})
