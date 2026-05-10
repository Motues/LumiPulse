/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,ts,js}",
  ],
  theme: {
    extend: {
      colors: {
        emerald: {
          50: '#e6f7ed',
          100: '#b3e6c7',
          200: '#80d5a1',
          300: '#4dc47b',
          400: '#26b35f',
          500: '#038437',
          600: '#026a2c',
          700: '#015021',
          800: '#013616',
          900: '#001c0b',
          950: '#000e06',
        },
      },
    },
  },
  plugins: [],
}
