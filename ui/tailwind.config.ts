import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          50:  '#f0faf6',
          100: '#d4f1e6',
          200: '#a9e3cd',
          300: '#74cfb0',
          400: '#44b896',
          500: '#2ea87f',
          600: '#218768',
          700: '#1c6c55',
          800: '#195644',
          900: '#164738',
        },
        // Keep primary as alias
        primary: {
          50:  '#f0faf6',
          100: '#d4f1e6',
          200: '#a9e3cd',
          300: '#74cfb0',
          400: '#44b896',
          500: '#2ea87f',
          600: '#218768',
          700: '#1c6c55',
          800: '#195644',
          900: '#164738',
        },
      },
      backgroundImage: {
        'gradient-login': 'linear-gradient(135deg, #f0faf6 0%, #d4f1e6 60%, #a9e3cd 100%)',
      },
      boxShadow: {
        'card': '0 1px 3px 0 rgb(0 0 0 / 0.07), 0 1px 2px -1px rgb(0 0 0 / 0.07)',
        'card-hover': '0 4px 12px 0 rgb(0 0 0 / 0.10), 0 2px 4px -1px rgb(0 0 0 / 0.06)',
        // Floating overlays (dropdowns, popovers, suggestion lists) — the tier
        // between a resting card and a modal. One token so they stop drifting to raw shadow-lg.
        'popover': '0 10px 24px -8px rgb(0 0 0 / 0.15), 0 2px 6px -2px rgb(0 0 0 / 0.08)',
        'modal': '0 20px 60px -10px rgb(0 0 0 / 0.25)',
        'topbar': '0 1px 0 0 #e2e8f0',
      },
    },
  },
  plugins: [],
}
export default config
