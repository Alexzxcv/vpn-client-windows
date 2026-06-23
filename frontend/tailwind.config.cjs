/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: ['./src/**/*.{ts,tsx,html}'],
  theme: {
    extend: {
      colors: {
        // Surfaces (cold blue-black) — DESIGN_SYSTEM §1
        void: 'var(--void)',
        slate: 'var(--slate)',
        graphite: 'var(--graphite)',
        hairline: 'var(--hairline)',
        edge: 'var(--edge)',
        // Text
        frost: 'var(--frost)',
        ash: 'var(--ash)',
        mute: 'var(--mute)',
        // Brand accent
        ion: 'var(--ion)',
        'ion-dim': 'var(--ion-dim)',
        // Status
        ok: 'var(--ok)',
        warn: 'var(--warn)',
        alert: 'var(--alert)',
        // dataviz secondary
        violet: 'var(--viz-violet)',
      },
      fontFamily: {
        display: ['Archivo', 'system-ui', 'sans-serif'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'Consolas', 'monospace'],
      },
      fontSize: {
        // type scale (rem) — DESIGN_SYSTEM §2
        '2xs': '0.6875rem',
        xs: '0.75rem',
        sm: '0.8125rem',
        base: '0.875rem',
        md: '1rem',
        lg: '1.25rem',
        xl: '1.625rem',
        '2xl': '2.25rem',
        '3xl': '3.25rem',
      },
      borderRadius: {
        sm: 'var(--r-sm)',
        md: 'var(--r-md)',
      },
      spacing: {
        // tight scale 2/4/8/12/16/24/32/48 already covered by tailwind defaults
      },
      letterSpacing: {
        eyebrow: '0.12em',
      },
      boxShadow: {
        'ion-glow': '0 0 24px rgba(61, 169, 252, 0.28)',
      },
    },
  },
  plugins: [],
};
