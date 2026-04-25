import type { Config } from 'tailwindcss';

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Fira Sans', 'Inter', 'ui-sans-serif', 'system-ui'],
        mono: ['Fira Code', 'JetBrains Mono', 'ui-monospace', 'SFMono-Regular'],
      },
      colors: {
        ink: {
          950: '#020617',
          900: '#0f172a',
          800: '#1e293b',
          700: '#334155',
        },
        brand: {
          400: '#60a5fa',
          500: '#3b82f6',
          600: '#2563eb',
        },
        accent: {
          400: '#fbbf24',
          500: '#f59e0b',
        },
      },
      boxShadow: {
        glow: '0 0 32px rgba(59, 130, 246, 0.18)',
      },
    },
  },
  plugins: [],
} satisfies Config;
