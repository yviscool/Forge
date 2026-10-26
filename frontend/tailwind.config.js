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
        // 系统栈领衔：教师机与机房都是 Windows，与 OS 同源才「原生」；
        // Manrope / Noto Sans SC 只作回退，保证非 Windows 平台不塌。
        sans: [
          'Segoe UI Variable Text',
          'Segoe UI',
          'Manrope Variable',
          'Noto Sans SC',
          'Microsoft YaHei',
          'sans-serif',
        ],
        mono: ['JetBrains Mono Variable', 'Cascadia Code', 'Consolas', 'monospace'],
      },
      colors: {
        ink: '#131c2b',
        slate: {
          // WCAG AA contrast for compact supporting copy on the cool paper
          // surfaces used throughout the application.
          400: '#5f6b7a',
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
          100: '#e2eaf2',
          500: '#2b4a63',
          700: '#1a3549',
          800: '#132a3e',
          900: '#0f2233',
        },
      },
      boxShadow: {
        lift: '0 1px 2px rgba(19,28,43,0.05), 0 16px 40px -24px rgba(19,28,43,0.24)',
      },
      borderRadius: {
        '4xl': '2rem',
      },
    },
  },
  plugins: [],
}
