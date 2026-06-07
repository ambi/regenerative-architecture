import type { Config } from 'tailwindcss'
import animate from 'tailwindcss-animate'

/**
 * デザイントークンは globals.css の CSS 変数 (HSL) に集約。
 * shadcn/ui 規約に従い、ここでは色名のセマンティックマッピングのみ宣言する。
 *
 * カラー方針 (refined enterprise minimalism):
 *   - 落ち着いた cool slate ベース、純白を避けてわずかに灰みを残す
 *   - primary は深く彩度を抑えた sapphire blue (信頼)
 *   - accent は warm amber (人間味の差し色、status indicator として控えめに利用)
 *   - destructive は朱寄りの red で警告強度を担保
 *
 * Typography:
 *   - display: IBM Plex Serif (editorial な重み)
 *   - body:    IBM Plex Sans (humanist geometric、Inter ではない)
 *   - mono:    IBM Plex Mono (client_id / user_code / jti などの技術 ID)
 */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    container: {
      center: true,
      padding: '1rem',
      screens: { '2xl': '1280px' },
    },
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        warning: {
          DEFAULT: 'hsl(var(--warning))',
          foreground: 'hsl(var(--warning-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif'],
        serif: ['"IBM Plex Serif"', 'Georgia', 'serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      boxShadow: {
        // 単なる drop shadow ではなく、上方からの subtle 反射 + 下方の影で深みを出す
        elevated:
          '0 1px 2px rgba(15, 23, 42, 0.04), 0 12px 32px -8px rgba(15, 23, 42, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.6)',
      },
      keyframes: {
        'slide-up-fade': {
          '0%': { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
      },
      animation: {
        'slide-up-fade': 'slide-up-fade 0.5s cubic-bezier(0.2, 0.8, 0.2, 1) both',
        'fade-in': 'fade-in 0.4s ease-out both',
      },
    },
  },
  plugins: [animate],
} satisfies Config
