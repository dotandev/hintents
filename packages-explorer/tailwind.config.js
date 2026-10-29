// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

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
        },
        accent: {
          DEFAULT: 'var(--accent)',
          strong: 'var(--accent-strong)',
          dim: 'var(--accent-dim)',
        },
      },
      fontFamily: {
        sans: ['var(--font-sans)', 'Inter Tight', 'sans-serif'],
        mono: ['var(--font-mono)', 'JetBrains Mono', 'monospace'],
      },
    },
  },
  plugins: [],
};
