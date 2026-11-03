import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../Tabs'

describe('Tabs - Simple mode', () => {
  const tabs = [
    { id: 'tab1', label: 'Tab 1' },
    { id: 'tab2', label: 'Tab 2' },
    { id: 'tab3', label: 'Tab 3' },
  ]

  it('renders all tab buttons', () => {
    render(<Tabs tabs={tabs} activeTab="tab1" onChangeTab={vi.fn()} />)
    expect(screen.getByText('Tab 1')).toBeInTheDocument()
    expect(screen.getByText('Tab 2')).toBeInTheDocument()
    expect(screen.getByText('Tab 3')).toBeInTheDocument()
  })

  it('applies active styling to the active tab', () => {
    render(<Tabs tabs={tabs} activeTab="tab1" onChangeTab={vi.fn()} />)
    const activeButton = screen.getByText('Tab 1')
    expect(activeButton.className).toContain('shadow-sm')
  })

  it('calls onChangeTab when a tab is clicked', async () => {
    const user = userEvent.setup()
    const onChangeTab = vi.fn()
    render(<Tabs tabs={tabs} activeTab="tab1" onChangeTab={onChangeTab} />)

    await user.click(screen.getByText('Tab 2'))
    expect(onChangeTab).toHaveBeenCalledWith('tab2')
  })

  it('calls onChangeTab with correct tab id', async () => {
    const user = userEvent.setup()
    const onChangeTab = vi.fn()
    render(<Tabs tabs={tabs} activeTab="tab1" onChangeTab={onChangeTab} />)

    await user.click(screen.getByText('Tab 3'))
    expect(onChangeTab).toHaveBeenCalledWith('tab3')
  })
})

describe('Tabs - Complex mode with children', () => {
  it('renders children tabs', () => {
    render(
      <Tabs defaultValue="first">
        <TabsList>
          <TabsTrigger value="first">First</TabsTrigger>
          <TabsTrigger value="second">Second</TabsTrigger>
        </TabsList>
        <TabsContent value="first">Content 1</TabsContent>
        <TabsContent value="second">Content 2</TabsContent>
      </Tabs>
    )

    expect(screen.getByText('First')).toBeInTheDocument()
    expect(screen.getByText('Second')).toBeInTheDocument()
    expect(screen.getByText('Content 1')).toBeInTheDocument()
  })

  it('shows content for the active tab only', () => {
    render(
      <Tabs defaultValue="first">
        <TabsList>
          <TabsTrigger value="first">First</TabsTrigger>
          <TabsTrigger value="second">Second</TabsTrigger>
        </TabsList>
        <TabsContent value="first">Content 1</TabsContent>
        <TabsContent value="second">Content 2</TabsContent>
      </Tabs>
    )

    expect(screen.getByText('Content 1')).toBeInTheDocument()
    expect(screen.queryByText('Content 2')).not.toBeInTheDocument()
  })

  it('switches content when clicking a different trigger', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="first">
        <TabsList>
          <TabsTrigger value="first">First</TabsTrigger>
          <TabsTrigger value="second">Second</TabsTrigger>
        </TabsList>
        <TabsContent value="first">Content 1</TabsContent>
        <TabsContent value="second">Content 2</TabsContent>
      </Tabs>
    )

    await user.click(screen.getByText('Second'))

    expect(screen.queryByText('Content 1')).not.toBeInTheDocument()
    expect(screen.getByText('Content 2')).toBeInTheDocument()
  })

  it('calls onValueChange when tab changes', async () => {
    const user = userEvent.setup()
    const onValueChange = vi.fn()
    render(
      <Tabs defaultValue="first" onValueChange={onValueChange}>
        <TabsList>
          <TabsTrigger value="first">First</TabsTrigger>
          <TabsTrigger value="second">Second</TabsTrigger>
        </TabsList>
        <TabsContent value="first">Content 1</TabsContent>
        <TabsContent value="second">Content 2</TabsContent>
      </Tabs>
    )

    await user.click(screen.getByText('Second'))
    expect(onValueChange).toHaveBeenCalledWith('second')
  })

  it('supports controlled mode', () => {
    render(
      <Tabs value="second">
        <TabsList>
          <TabsTrigger value="first">First</TabsTrigger>
          <TabsTrigger value="second">Second</TabsTrigger>
        </TabsList>
        <TabsContent value="first">Content 1</TabsContent>
        <TabsContent value="second">Content 2</TabsContent>
      </Tabs>
    )

    expect(screen.queryByText('Content 1')).not.toBeInTheDocument()
    expect(screen.getByText('Content 2')).toBeInTheDocument()
  })
})

describe('TabsList', () => {
  it('renders children', () => {
    render(
      <TabsList>
        <button>Tab A</button>
      </TabsList>
    )
    expect(screen.getByText('Tab A')).toBeInTheDocument()
  })

  it('applies custom className', () => {
    const { container } = render(
      <TabsList className="custom-list">
        <button>Tab A</button>
      </TabsList>
    )
    expect(container.firstChild).toHaveClass('custom-list')
  })
})

// Responsive guard (audit F7): the tab strip must scroll instead of overflowing the
// viewport, and triggers must not shrink to illegible widths. Class-contract guard;
// real layout is covered by tests/responsive-audit.
describe('Tabs - responsive overflow contract', () => {
  it('simple mode strip scrolls horizontally and is capped to its container', () => {
    const { container } = render(
      <Tabs tabs={[{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }]} activeTab="a" onChangeTab={vi.fn()} />
    )
    const strip = container.firstElementChild as HTMLElement
    expect(strip.className).toContain('overflow-x-auto')
    expect(strip.className).toContain('max-w-full')
  })

  it('TabsList scrolls horizontally and triggers do not shrink', () => {
    render(
      <Tabs defaultValue="a">
        <TabsList>
          <TabsTrigger value="a">A</TabsTrigger>
        </TabsList>
      </Tabs>
    )
    const trigger = screen.getByText('A')
    expect(trigger.className).toContain('flex-shrink-0')
    expect((trigger.parentElement as HTMLElement).className).toContain('overflow-x-auto')
  })
})

// Touch-target interplay (follow-up to F7): triggers grow to 44px on coarse pointers (index.css), so a FIXED h-10
// strip makes them overflow it and overlap/occlude each other. The strip must size to its content and the simple-mode
// triggers must not shrink (the complex-mode ones already do not).
describe('Tabs - strip grows with touch-sized triggers', () => {
  it('simple mode: triggers are flex-shrink-0 and the strip has no fixed height', () => {
    const { container } = render(
      <Tabs tabs={[{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }]} activeTab="a" onChangeTab={vi.fn()} />
    )
    const strip = container.firstElementChild as HTMLElement
    expect(strip.className).not.toMatch(/(^|\s)h-10(\s|$)/)
    expect(strip.className).toMatch(/min-h-\[2.5rem\]/)
    for (const b of Array.from(strip.querySelectorAll('button'))) expect(b.className).toContain('flex-shrink-0')
  })

  it('TabsList has no fixed height either', () => {
    const { container } = render(
      <Tabs defaultValue="a"><TabsList><TabsTrigger value="a">A</TabsTrigger></TabsList></Tabs>
    )
    expect((container.firstElementChild as HTMLElement).className).not.toMatch(/(^|\s)h-10(\s|$)/)
  })
})
