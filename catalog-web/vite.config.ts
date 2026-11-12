/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import fs from 'fs'

function getApiPort(): number {
  try {
    const port = fs.readFileSync('../catalog-api/.service-port', 'utf8').trim();
    return parseInt(port, 10);
  } catch {
    return 8080;
  }
}

function getApiHost(): string {
  return process.env.VITE_API_HOST || 'localhost';
}

// CAT-PREEXIST-7: the @vasic-digital/* file: packages point at
// "dist/index.js", but dist/ is a build output that is not tracked in the
// submodules (only ui_components_react ships one), so a fresh checkout cannot
// resolve them under vitest. Fall back to the package source ONLY where dist
// is absent; a built dist keeps being used unchanged.
// CAVEAT (PW-F3/PW-F6): the check is on file presence only. A dist/ that is out
// of date with src/ wins over src/ silently, so tests then run against old
// code. That covers a tracked dist/ (as in ui_components_react) as well as one
// left over from an earlier build; dist/ is gitignored in the other 8
// submodules, so a local build leaves an ignored dist/ that `git status` does
// not show. Rebuild or delete dist/ to test src/.
function submoduleSourceFallbacks(): Record<string, string> {
  const pkgs: Record<string, string> = {
    '@vasic-digital/auth-context': 'auth_context_react',
    '@vasic-digital/catalogizer-api-client': 'catalogizer_api_client_ts',
    '@vasic-digital/collection-manager': 'collection_manager_react',
    '@vasic-digital/dashboard-analytics': 'dashboard_analytics_react',
    '@vasic-digital/media-browser': 'media_browser_react',
    '@vasic-digital/media-player': 'media_player_react',
    '@vasic-digital/media-types': 'media_types_ts',
    '@vasic-digital/ui-components': 'ui_components_react',
    '@vasic-digital/websocket-client': 'websocket_client_ts',
  };
  const out: Record<string, string> = {};
  for (const [name, dir] of Object.entries(pkgs)) {
    const root = path.resolve(__dirname, '../submodules', dir);
    const src = path.join(root, 'src/index.ts');
    if (!fs.existsSync(path.join(root, 'dist/index.js')) && fs.existsSync(src)) {
      out[name] = src;
    }
  }
  return out;
}

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    alias: submoduleSourceFallbacks(),
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    css: true,
    exclude: [
      '**/node_modules/**',
      '**/dist/**',
      '**/e2e/**',  // Exclude Playwright E2E tests from Vitest
      '**/*.spec.ts',  // Exclude Playwright spec files
    ],
    deps: {
      optimizer: {
        web: {
          include: ['@vitejs/plugin-react', '@vasic-digital/ui-components'],
        },
      },
    },
  },
  resolve: {
    // Role: RESOLUTION, not proven deduplication. Submodule sources (aliased
    // above to ../submodules/*/src) import react/axios/etc. from directories
    // that have no node_modules, so these entries make Vite resolve those bare
    // imports from the app's node_modules. PW-F1 (CAT-PREEXIST-7): removing
    // 'react'/'react-dom' from this list was tried and a hook-using submodule
    // component (ui-components Tabs) still rendered, so the entries are not
    // test-guarded and their single-instance effect is UNCONFIRMED here.
    dedupe: ['react', 'react-dom', '@tanstack/react-query', 'axios'],
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@/components': path.resolve(__dirname, './src/components'),
      '@/hooks': path.resolve(__dirname, './src/hooks'),
      '@/lib': path.resolve(__dirname, './src/lib'),
      '@/types': path.resolve(__dirname, './src/types'),
      '@/services': path.resolve(__dirname, './src/services'),
      '@/store': path.resolve(__dirname, './src/store'),
      '@/pages': path.resolve(__dirname, './src/pages'),
      '@/assets': path.resolve(__dirname, './src/assets'),
    },
  },
  server: {
    port: 3000,
    host: true,
    proxy: {
      '/api': {
        target: 'http://' + getApiHost() + ':' + getApiPort(),
        changeOrigin: true,
        secure: false,
      },
      '/ws': {
        target: 'ws://' + getApiHost() + ':' + getApiPort(),
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom'],
          router: ['react-router-dom'],
          ui: ['framer-motion', 'lucide-react', '@headlessui/react'],
          charts: ['recharts'],
          utils: ['axios', 'date-fns', 'clsx', 'tailwind-merge'],
        },
      },
    },
  },
  optimizeDeps: {
    include: ['react', 'react-dom', 'react-router-dom'],
  },
})