import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'

export default defineConfig({
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
      codeSplittingOptions: {
        defaultBehavior: [['loader', 'component']],
      },
    }),
    react(),
    tailwindcss(),
  ],
  base: '/',
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '^/realms/[^/]+/(api|authorize|token|revoke|introspect|userinfo|register|par|device_authorization|end_session|\\.well-known|jwks)(/|\\?|$)':
        'http://localhost:8081',
      '/api': 'http://localhost:8081',
      '/authorize': 'http://localhost:8081',
      '/token': 'http://localhost:8081',
      '/revoke': 'http://localhost:8081',
      '/introspect': 'http://localhost:8081',
      '/userinfo': 'http://localhost:8081',
      '/register': 'http://localhost:8081',
      '/par': 'http://localhost:8081',
      '/device_authorization': 'http://localhost:8081',
      '/end_session': 'http://localhost:8081',
      '/.well-known': 'http://localhost:8081',
      '/jwks': 'http://localhost:8081',
      '/health': 'http://localhost:8081',
    },
  },
  build: {
    cssCodeSplit: false,
  },
})
