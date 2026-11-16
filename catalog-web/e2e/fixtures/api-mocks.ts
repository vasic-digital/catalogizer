import { Page } from '@playwright/test';

// Sample media data used across tests
export const mockMedia = [
  {
    id: 1,
    title: 'The Matrix',
    media_type: 'movie',
    year: 1999,
    rating: 8.7,
    description: 'A computer hacker learns about the true nature of reality.',
    poster_url: '/placeholder-poster.jpg',
    file_count: 2,
    total_size: 4500000000,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 2,
    title: 'Breaking Bad',
    media_type: 'tv_show',
    year: 2008,
    rating: 9.5,
    description: 'A chemistry teacher turned drug lord.',
    poster_url: '/placeholder-poster2.jpg',
    file_count: 62,
    total_size: 45000000000,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 3,
    title: 'Dark Side of the Moon',
    media_type: 'music_album',
    year: 1973,
    rating: 9.3,
    description: 'Pink Floyd classic album.',
    poster_url: '/placeholder-cover.jpg',
    file_count: 10,
    total_size: 800000000,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
];

/**
 * Mock dashboard-related API endpoints.
 */
export async function mockDashboardEndpoints(page: Page) {
  // Dashboard stats
  await page.route('**/api/v1/stats**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        total_files: 1250,
        total_size: 536870912000,
        total_entities: 340,
        total_collections: 5,
        total_favorites: 12,
        storage_roots: 3,
        recent_scans: [],
        media_by_type: {
          movie: 150,
          tv_show: 80,
          music_album: 60,
          song: 200,
          game: 30,
          software: 20,
        },
      }),
    });
  });

  // Health endpoint
  await page.route('**/api/v1/health', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        status: 'healthy',
        uptime: 86400,
        version: '1.0.0',
        database: 'ok',
      }),
    });
  });

  // Config endpoint
  await page.route('**/api/v1/config**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        version: '1.0.0',
        features: {
          ai_enabled: true,
          collections_enabled: true,
          favorites_enabled: true,
          conversion_enabled: true,
        },
      }),
    });
  });

  // Storage roots
  await page.route('**/api/v1/storage-roots**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        roots: [
          { id: 1, path: '//nas.local/media', protocol: 'smb', status: 'connected' },
          { id: 2, path: '/data/local', protocol: 'local', status: 'connected' },
        ],
      }),
    });
  });

  // Scan status
  await page.route('**/api/v1/scan/status**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        scanning: false,
        last_scan: new Date().toISOString(),
        files_scanned: 1250,
      }),
    });
  });

  // Recent activity
  await page.route('**/api/v1/activity**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ activities: [], total: 0 }),
    });
  });
}

/**
 * Mock media/entity browsing API endpoints.
 */
export async function mockMediaEndpoints(page: Page) {
  // Entity types
  await page.route('**/api/v1/entities/types', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        types: [
          { id: 1, name: 'movie', display_name: 'Movies', count: 150 },
          { id: 2, name: 'tv_show', display_name: 'TV Shows', count: 80 },
          { id: 3, name: 'music_album', display_name: 'Music Albums', count: 60 },
          { id: 4, name: 'song', display_name: 'Songs', count: 200 },
          { id: 5, name: 'game', display_name: 'Games', count: 30 },
          { id: 6, name: 'software', display_name: 'Software', count: 20 },
        ],
      }),
    });
  });

  // Entity listing/browse
  await page.route('**/api/v1/entities?**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: mockMedia,
        total: mockMedia.length,
        limit: 24,
        offset: 0,
      }),
    });
  });

  // Entity stats
  await page.route('**/api/v1/entities/stats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        total_entities: 340,
        total_files: 1250,
        by_type: { movie: 150, tv_show: 80, music_album: 60, song: 200, game: 30, software: 20 },
      }),
    });
  });

  // Entity search
  await page.route('**/api/v1/entities/search**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: mockMedia,
        total: mockMedia.length,
        limit: 24,
        offset: 0,
      }),
    });
  });

  // Single entity detail
  await page.route('**/api/v1/entities/*', async (route) => {
    const url = route.request().url();
    if (url.includes('/types') || url.includes('/search') || url.includes('/stats')) {
      return route.fallback();
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ...mockMedia[0],
        files: [
          { id: 1, name: 'The.Matrix.1999.BluRay.mkv', size: 4500000000, path: '/movies/The.Matrix.1999.BluRay.mkv' },
        ],
      }),
    });
  });

  // Media browse (legacy endpoint)
  await page.route('**/api/v1/media**', async (route) => {
    const url = route.request().url();
    if (url.includes('/stats') || url.includes('/search')) {
      return route.fallback();
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: mockMedia,
        total: mockMedia.length,
        limit: 24,
        offset: 0,
      }),
    });
  });

  // Media stats
  await page.route('**/api/v1/media/stats**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        total_items: 340,
        by_type: { movie: 150, tv_show: 80, music_album: 60, song: 200, game: 30, software: 20 },
        total_size: 536870912000,
        recent_additions: 8,
      }),
    });
  });

  // Media search
  await page.route('**/api/v1/media/search**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: mockMedia,
        total: mockMedia.length,
        limit: 24,
        offset: 0,
      }),
    });
  });

  // Catalog browse
  await page.route('**/api/v1/catalog/browse**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [],
        path: '/',
        total: 0,
      }),
    });
  });
}

/**
 * Mock collection-related API endpoints.
 */
export async function mockCollectionEndpoints(page: Page) {
  await page.route('**/api/v1/collections**', async (route) => {
    const url = route.request().url();
    const method = route.request().method();

    if (method === 'POST') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'col-new',
          name: 'New Collection',
          description: '',
          item_count: 0,
          is_smart: false,
          is_public: false,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }),
      });
      return;
    }

    // Specific collection by ID
    if (url.match(/\/collections\/[^/?]+$/)) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'col-1',
          name: 'Action Movies',
          description: 'Best action films',
          item_count: 25,
          is_smart: false,
          is_public: false,
          media_type: 'video',
          items: mockMedia.slice(0, 2),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }),
      });
      return;
    }

    // Collections list
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        collections: [
          {
            id: 'col-1',
            name: 'Action Movies',
            description: 'Best action films',
            item_count: 25,
            is_smart: false,
            is_public: false,
            media_type: 'video',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
          {
            id: 'col-2',
            name: 'Jazz Collection',
            description: 'Smooth jazz albums',
            item_count: 42,
            is_smart: true,
            is_public: true,
            media_type: 'music',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ],
        total: 2,
      }),
    });
  });
}

/**
 * Mock all API endpoints. Combines dashboard, media, collection, and favorites mocks.
 */
export async function mockAllEndpoints(page: Page) {
  await mockDashboardEndpoints(page);
  await mockMediaEndpoints(page);
  await mockCollectionEndpoints(page);

  // Favorites
  await page.route('**/api/v1/favorites**', async (route) => {
    const url = route.request().url();
    if (url.includes('/stats')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          total_count: 2,
          media_type_breakdown: { movie: 1, tv_show: 1 },
          recent_additions: [],
        }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          { id: 1, media_id: 1, title: 'The Matrix', media_type: 'movie', added_at: new Date().toISOString() },
          { id: 2, media_id: 2, title: 'Breaking Bad', media_type: 'tv_show', added_at: new Date().toISOString() },
        ],
        total: 2,
      }),
    });
  });

  // Playlists
  await page.route('**/api/v1/playlists**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ playlists: [], total: 0 }),
    });
  });

  // Challenges
  await page.route('**/api/v1/challenges**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ challenges: [], total: 0 }),
    });
  });

  // Admin settings
  await page.route('**/api/v1/admin/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ settings: {}, message: 'OK' }),
    });
  });

  // Conversion jobs
  await page.route('**/api/v1/conversion/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ jobs: [], total: 0 }),
    });
  });

  // Subtitles
  await page.route('**/api/v1/subtitles/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ subtitles: [], total: 0 }),
    });
  });
}
