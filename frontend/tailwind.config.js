/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    extend: {
      colors: {
        // A warm, low-chroma beige system. Backgrounds recede, the terracotta
        // accent is reserved for state that matters, and every pairing below
        // clears WCAG AA on its intended surface.
        sand: {
          50: '#FDFBF7',
          100: '#FAF6EE',
          200: '#F2EADC',
          300: '#E7DBC7',
          400: '#D8C7AC',
          500: '#C3AC8B',
          600: '#A88F6C',
          700: '#877053',
          800: '#5F4E39',
          900: '#3B3025',
        },
        ink: {
          50: '#F6F4F1',
          100: '#E4DFD8',
          200: '#C4BCB0',
          300: '#9C9184',
          400: '#786D60',
          500: '#584E43',
          600: '#413931',
          700: '#2E2823',
          800: '#221D19',
          900: '#171310',
        },
        clay: {
          50: '#FBF1EC',
          100: '#F4DED2',
          200: '#E5B99F',
          300: '#D28F6B',
          400: '#B4643C',
          500: '#9A5231',
          600: '#7C4127',
          700: '#5D311D',
        },
        sage: {
          100: '#E5EBE2',
          300: '#A8BBA0',
          500: '#6B7F6B',
          700: '#495A49',
        },
        amberish: {
          100: '#F7ECCF',
          300: '#E0BF6B',
          500: '#C9A227',
          700: '#8E7119',
        },
      },
      fontFamily: {
        sans: ['"Inter var"', 'Inter', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'],
        display: ['"Bricolage Grotesque"', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      boxShadow: {
        soft: '0 1px 2px rgba(59, 48, 37, 0.04), 0 8px 24px -12px rgba(59, 48, 37, 0.18)',
        lifted: '0 2px 4px rgba(59, 48, 37, 0.06), 0 18px 40px -18px rgba(59, 48, 37, 0.28)',
        inset: 'inset 0 1px 0 rgba(255, 255, 255, 0.6)',
      },
      borderRadius: {
        xl2: '1.25rem',
      },
      transitionTimingFunction: {
        smooth: 'cubic-bezier(0.22, 1, 0.36, 1)',
      },
      keyframes: {
        'fade-up': {
          '0%': { opacity: '0', transform: 'translateY(6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'pulse-ring': {
          '0%': { transform: 'scale(0.7)', opacity: '0.7' },
          '80%, 100%': { transform: 'scale(2.2)', opacity: '0' },
        },
        'spin-slow': {
          '0%': { transform: 'rotateY(0deg)' },
          '100%': { transform: 'rotateY(360deg)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-400px 0' },
          '100%': { backgroundPosition: '400px 0' },
        },
      },
      animation: {
        'fade-up': 'fade-up 0.35s cubic-bezier(0.22, 1, 0.36, 1) both',
        'pulse-ring': 'pulse-ring 2s cubic-bezier(0.24, 0, 0.38, 1) infinite',
        'spin-slow': 'spin-slow 14s linear infinite',
        shimmer: 'shimmer 1.4s linear infinite',
      },
    },
  },
  plugins: [],
}
