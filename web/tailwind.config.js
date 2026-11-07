/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        background: '#090d16',
        surface: '#111726',
        'surface-elevated': '#182138',
        border: '#232f4c',
        'border-focus': '#3b82f6',
        accent: '#6366f1',
        chaos: '#f43f5e',
        healthy: '#10b981',
        warning: '#f59e0b',
      },
      fontFamily: {
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
      },
    },
  },
  plugins: [],
}
