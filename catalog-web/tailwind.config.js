/** @type {import('tailwindcss').Config} */
//
// Catalogizer Blue token wiring (§11.4.162 OpenDesign mandate).
// The shadcn HSL CSS vars (src/index.css :root/.dark) are the runtime theme
// source; the static colour ramps below are re-anchored so `primary-600`
// (#1565C0) and `accent-600` (#6B5778) agree with the brand vars — removing the
// pre-existing drift where the static `primary` scale was an unrelated blue.
// fontFamily / spacing / radius are sourced from src/styles/tokens.ts so config
// and CSS share one source of truth.
import { typography as designTypography, space as designSpace, radius as designRadius } from './src/styles/tokens'

export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        // Catalogizer Blue — anchored on brand primary #1565C0 at the 600 step.
        primary: {
          50: '#edf4fd',
          100: '#d1e4fa',
          200: '#a3c9f5',
          300: '#6ca9ef',
          400: '#3086e8',
          500: '#1872d8',
          600: '#1565c0',
          700: '#11529c',
          800: '#0e417c',
          900: '#0b3361',
          950: '#071f3c',
        },
        secondary: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#0f172a',
          950: '#020617',
        },
        // Brand accent — anchored on #6B5778 at the 600 step (replaces stray fuchsia).
        accent: {
          50: '#f5f3f6',
          100: '#e6e1ea',
          200: '#d3cad8',
          300: '#baacc3',
          400: '#9883a5',
          500: '#7f678e',
          600: '#6b5778',
          700: '#574762',
          800: '#45384d',
          900: '#352b3b',
          950: '#221c26',
        },
        success: {
          50: '#f0fdf4',
          100: '#dcfce7',
          200: '#bbf7d0',
          300: '#86efac',
          400: '#4ade80',
          500: '#22c55e',
          600: '#16a34a',
          700: '#15803d',
          800: '#166534',
          900: '#14532d',
          950: '#052e16',
        },
        warning: {
          50: '#fffbeb',
          100: '#fef3c7',
          200: '#fde68a',
          300: '#fcd34d',
          400: '#fbbf24',
          500: '#f59e0b',
          600: '#d97706',
          700: '#b45309',
          800: '#92400e',
          900: '#78350f',
          950: '#451a03',
        },
        error: {
          50: '#fef2f2',
          100: '#fee2e2',
          200: '#fecaca',
          300: '#fca5a5',
          400: '#f87171',
          500: '#ef4444',
          600: '#dc2626',
          700: '#b91c1c',
          800: '#991b1b',
          900: '#7f1d1d',
          950: '#450a0a',
        },
      },
      fontFamily: {
        sans: designTypography.fontFamily.sans.split(',').map((f) => f.trim()),
        mono: designTypography.fontFamily.mono.split(',').map((f) => f.trim()),
      },
      animation: {
        'fade-in': 'fadeIn 0.5s ease-in-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'scale-up': 'scaleUp 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'bounce-slow': 'bounce 2s infinite',
        'spin-slow': 'spin 3s linear infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { transform: 'translateY(20px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        slideDown: {
          '0%': { transform: 'translateY(-20px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        scaleUp: {
          '0%': { transform: 'scale(0.95)', opacity: '0' },
          '100%': { transform: 'scale(1)', opacity: '1' },
        },
      },
      boxShadow: {
        'glass': '0 8px 32px 0 rgba(31, 38, 135, 0.37)',
        'glass-dark': '0 8px 32px 0 rgba(0, 0, 0, 0.37)',
        'neumorphism': '20px 20px 60px #bebebe, -20px -20px 60px #ffffff',
        'neumorphism-dark': '20px 20px 60px #0a0a0a, -20px -20px 60px #1a1a1a',
      },
      backdropBlur: {
        xs: '2px',
      },
      spacing: {
        '18': designSpace['18'],
        '88': designSpace['88'],
        '128': designSpace['128'],
      },
      borderRadius: {
        '4xl': designRadius['4xl'],
      },
      // @tailwindcss/aspect-ratio replaces theme.aspectRatio; re-add the core keys it drops (see src/__tests__/tailwindAspect.test.ts).
      aspectRatio: { auto: 'auto', square: '1 / 1', video: '16 / 9' },
      maxWidth: {
        '8xl': '88rem',
        '9xl': '96rem',
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
    require('@tailwindcss/aspect-ratio'),
  ],
}