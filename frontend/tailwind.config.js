/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Manrope Variable', 'Noto Sans SC', 'Microsoft YaHei', 'sans-serif'],
        mono: ['JetBrains Mono Variable', 'Cascadia Code', 'Consolas', 'monospace'],
      },
      colors: {
        ink: '#101828',
        slate: {
          // WCAG AA contrast for compact supporting copy on the warm paper
          // surfaces used throughout the application.
          400: '#667085',
        },
        forge: {
          50: '#fff7ed',
          100: '#ffedd5',
          200: '#fed7aa',
          300: '#fdba74',
          400: '#fb923c',
          500: '#f97316',
          600: '#ea580c',
          700: '#c2410c',
          800: '#9a3412',
          900: '#7c2d12',
        },
        navy: {
          50: '#f2f6fa',
          100: '#e3ebf3',
          500: '#31526f',
          700: '#1f3a52',
          800: '#172d41',
          900: '#14283d',
        },
      },
      boxShadow: {
        lift: '0 1px 2px rgba(16,24,40,0.04), 0 20px 48px -24px rgba(16,24,40,0.28)',
      },
      borderRadius: {
        '4xl': '2rem',
      },
    },
  },
  plugins: [],
}
