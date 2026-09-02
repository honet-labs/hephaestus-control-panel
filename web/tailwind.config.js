/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#f0f7ff',
          100: '#D0E7E6',
          200: '#95CCDD',
          300: '#6ea6ec',
          400: '#4274D9',
          500: '#4274D9',
          600: '#293681',
          700: '#1e2863',
          800: '#161c47',
          900: '#0f1430',
        },
        blue: {
          50: '#f0f7ff',
          100: '#D0E7E6',
          200: '#95CCDD',
          300: '#6ea6ec',
          400: '#5384e6',
          500: '#4274D9',
          600: '#3461c2',
          700: '#293681',
          800: '#1e2863',
          900: '#0f1430',
        },
        navy: {
          DEFAULT: '#293681',
          dark: '#1e2863',
        },
        royal: {
          DEFAULT: '#4274D9',
          light: '#6ea6ec',
        },
        skycyan: {
          DEFAULT: '#95CCDD',
        },
        palemint: {
          DEFAULT: '#D0E7E6',
        },
        dark: {
          bg: '#090d16',
          card: '#0e1118',
          border: '#1b2234',
          muted: '#2a3652',
        }
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
        sans: ['"Google Sans"', '"Product Sans"', '"Open Sans"', 'Roboto', '-apple-system', 'BlinkMacSystemFont', 'system-ui', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
