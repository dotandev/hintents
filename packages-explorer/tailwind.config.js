/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        zinc: {
          900: '#18181b',
          950: '#09090b',
        }
      }
    },
  },
  plugins: [],
}
