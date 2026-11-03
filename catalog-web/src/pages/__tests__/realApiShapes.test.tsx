/**
 * Regression for the four pages that crashed into the ErrorBoundary against a
 * real backend (responsive audit 2026-11-03):
 *   /analytics  "y.slice is not a function"
 *   /collections "t is not iterable"
 *   /admin      "Cannot read properties of undefined (reading 'toLocaleString')"
 *   /settings   "i.map is not a function"
 * Root cause: the pages' api modules assumed bare arrays / flat report objects
 * while catal-api answers with envelopes ({items}, {scans}, {roots}, {data},
 * base64 report envelope). These tests run the REAL api modules and pages over
 * responses captured from a running catalog-api (transport-level fake only).
 */
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { installRealApi } from '@/test/realApiTransport'
import collectionsNonEmpty from '@/test/fixtures/realApi/collections_nonempty.json'

vi.mock('framer-motion', async () => {
  const passthrough = (tag: string) => {
    const C = ({ children, initial: _i, animate: _a, transition: _t, whileHover: _wh, whileTap: _wt, exit: _e, layout: _l, variants: _v, ...props }: any) =>
      React.createElement(tag, props, children)
    C.displayName = `motion.${tag}`
    return C
  }
  return {
    motion: new Proxy({}, { get: (_t, tag: string) => passthrough(tag) }),
    AnimatePresence: ({ children }: any) => <>{children}</>,
  }
})

// Real ErrorBoundary semantics: if a page throws during render the test fails.
function renderPage(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

let restore: () => void
afterEach(() => restore?.())

describe('pages survive real catalog-api response shapes', () => {
  it('Analytics renders recent media from the {items,total} envelope', async () => {
    restore = installRealApi()
    const { Analytics } = await import('../Analytics')
    renderPage(<Analytics />)
    await waitFor(() => expect(screen.getByText('Analytics Dashboard')).toBeInTheDocument())
    // recent media rows come from media/recent -> {items:[{title:'Test Movie'}...]}
    await waitFor(() => expect(screen.getByText('Test Movie')).toBeInTheDocument())
    expect(screen.getByText('Test Song')).toBeInTheDocument()
  })

  it('Collections renders the empty {items:[],total:0} list without crashing', async () => {
    restore = installRealApi()
    const { Collections } = await import('../Collections')
    renderPage(<Collections />)
    await waitFor(() => expect(screen.getByText(/Organize your media/i)).toBeInTheDocument())
    // the list query must have settled into the real empty state (not a crash)
    await waitFor(() => expect(screen.getByText('No collections found')).toBeInTheDocument())
  })

  it('Collections renders a backend collection (id:int, total_items, no smart_rules)', async () => {
    restore = installRealApi({ '/collections': collectionsNonEmpty })
    const { Collections } = await import('../Collections')
    renderPage(<Collections />)
    await waitFor(() => expect(screen.getByText('Weekend')).toBeInTheDocument())
  })

  it('Admin renders with the base64 report envelopes for usage/performance', async () => {
    restore = installRealApi()
    const { Admin } = await import('../Admin')
    renderPage(<Admin />)
    await waitFor(() => expect(screen.getByText(/Performance/i)).toBeInTheDocument())
  })

  it('Settings renders scans from {scans:[]} and storage roots from {roots:[...]}', async () => {
    restore = installRealApi()
    const { Settings } = await import('../Settings')
    renderPage(<Settings />)
    await waitFor(() => expect(screen.getByText('No scans recorded yet.')).toBeInTheDocument())
    // storage root names must come from {roots:[{name:'qa-local-media'}]}
    await waitFor(() => expect(screen.getByText(/qa-local-media/)).toBeInTheDocument())
  })
})
