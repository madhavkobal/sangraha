/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        bg: '#0f172a',
        surface: '#1e293b',
        border: '#334155',
        muted: '#94a3b8',
        accent: '#38bdf8',
        danger: '#f87171',
        success: '#4ade80',
        warn: '#fbbf24',
      },
    },
  },
  plugins: [],
}
