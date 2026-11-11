/**
 * P7 (docs/nexus/remaining-work.md): Vitest coverage for the
 * useCoverQuality hook. Every behaviour — debug gating, header
 * parsing, module-level cache, abort-on-unmount — gets a dedicated
 * case so regressions never silently ship.
 */

import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
// jest-dom matchers are registered globally in src/test-setup.ts,
// so no per-file import is required.

import { useCoverQuality, resetCoverQualityCache } from '@/hooks/useCoverQuality';
import { CoverQualityBadge } from '@/components/debug/CoverQualityBadge';

function TestConsumer({ id, debug }: { id: number | string | null; debug: boolean }) {
  const signal = useCoverQuality(id, debug);
  return (
    <div data-testid="signal">
      {signal ? `${signal.quality}|${signal.source}` : 'null'}
    </div>
  );
}

describe('useCoverQuality', () => {
  beforeEach(() => {
    resetCoverQualityCache();
    const fetchMock = vi.fn().mockResolvedValue({
      headers: {
        get: (name: string) => {
          if (name === 'X-Cover-Quality') return 'pass';
          if (name === 'X-Cover-Source') return 'tmdb';
          return null;
        },
      },
    });
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('returns null when coverId is null', () => {
    render(<TestConsumer id={null} debug={true} />);
    expect(screen.getByTestId('signal')).toHaveTextContent('null');
    expect(fetch).not.toHaveBeenCalled();
  });

  it('does not fire a request when debug=false', () => {
    render(<TestConsumer id={42} debug={false} />);
    expect(fetch).not.toHaveBeenCalled();
  });

  it('emits a single HEAD request with credentials and parses headers', async () => {
    render(<TestConsumer id={42} debug={true} />);
    await waitFor(() => {
      expect(screen.getByTestId('signal')).toHaveTextContent('pass|tmdb');
    });
    expect(fetch).toHaveBeenCalledTimes(1);
    const [url, init] = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe('/api/v1/cover/42');
    expect(init.method).toBe('HEAD');
    expect(init.credentials).toBe('include');
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  it('caches the signal so repeated mounts do not re-fetch', async () => {
    const first = render(<TestConsumer id={99} debug={true} />);
    await waitFor(() => expect(screen.getByTestId('signal')).toHaveTextContent('pass|tmdb'));
    first.unmount();

    render(<TestConsumer id={99} debug={true} />);
    expect(screen.getByTestId('signal')).toHaveTextContent('pass|tmdb');
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('falls back to "unknown" quality + empty source when headers are absent', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        headers: { get: () => null },
      }),
    );
    render(<TestConsumer id={55} debug={true} />);
    await waitFor(() => expect(screen.getByTestId('signal')).toHaveTextContent('unknown|'));
  });

  it('reports "unknown" on fetch rejection (non-abort)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')));
    render(<TestConsumer id={77} debug={true} />);
    await waitFor(() => expect(screen.getByTestId('signal')).toHaveTextContent('unknown|'));
  });

  it('silently ignores AbortError: no fallback state is set while mounted', async () => {
    let rejecter!: (err: Error) => void;
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        new Promise((_resolve, reject) => {
          rejecter = reject;
        }),
      ),
    );
    render(<TestConsumer id={88} debug={true} />);
    expect(screen.getByTestId('signal')).toHaveTextContent('null');
    const abortErr = new Error('aborted');
    (abortErr as { name?: string }).name = 'AbortError';
    await act(async () => {
      rejecter(abortErr);
    });
    // An AbortError must NOT be turned into the 'unknown|' fallback signal.
    expect(screen.getByTestId('signal')).toHaveTextContent(/^null$/);
  });

  it('unmount aborts the in-flight request', async () => {
    vi.stubGlobal('fetch', vi.fn(() => new Promise(() => undefined)));
    const { unmount } = render(<TestConsumer id={89} debug={true} />);
    const [, init] = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(init.signal.aborted).toBe(false);
    unmount();
    expect(init.signal.aborted).toBe(true);
  });

  it('encodes special characters in coverId before requesting', async () => {
    render(<TestConsumer id="abc/xyz?q=1" debug={true} />);
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    const [url] = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe('/api/v1/cover/abc%2Fxyz%3Fq%3D1');
  });
});

describe('CoverQualityBadge', () => {
  beforeEach(() => {
    resetCoverQualityCache();
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        headers: {
          get: (name: string) => {
            if (name === 'X-Cover-Quality') return 'fail_lowres';
            if (name === 'X-Cover-Source') return 'fanart';
            return null;
          },
        },
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders nothing in non-debug mode even when coverId is set', () => {
    const { container } = render(<CoverQualityBadge coverId={1} debug={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the verdict + source pill once the hook resolves', async () => {
    render(<CoverQualityBadge coverId={42} debug={true} />);
    const badge = await screen.findByTestId('cover-quality-badge');
    expect(badge).toHaveTextContent('fail_lowres');
    expect(badge).toHaveTextContent('fanart');
  });

  it('maps every known verdict to a distinct colour class', async () => {
    const verdicts: Record<string, string> = {
      pass: 'bg-emerald-500',
      fail_lowres: 'bg-amber-500',
      fail_blurry: 'bg-amber-600',
      fail_corrupt: 'bg-rose-600',
      placeholder_fallback: 'bg-slate-500',
      unknown: 'bg-slate-400',
    };
    for (const [verdict, expectedClass] of Object.entries(verdicts)) {
      resetCoverQualityCache();
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({
          headers: {
            get: (name: string) => (name === 'X-Cover-Quality' ? verdict : ''),
          },
        }),
      );
      const { unmount } = render(<CoverQualityBadge coverId={verdict} debug={true} />);
      const badge = await screen.findByTestId('cover-quality-badge');
      expect(badge.className).toContain(expectedClass);
      unmount();
    }
  });

  it('omits the source chip when backend returns an empty X-Cover-Source', async () => {
    resetCoverQualityCache();
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        headers: {
          get: (name: string) => (name === 'X-Cover-Quality' ? 'pass' : ''),
        },
      }),
    );
    render(<CoverQualityBadge coverId={101} debug={true} />);
    const badge = await screen.findByTestId('cover-quality-badge');
    expect(badge).toHaveTextContent('pass');
    expect(badge).not.toHaveTextContent('·');
  });
});
