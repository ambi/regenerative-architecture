import path from 'node:path'
import { TanStackRouterVite } from '@tanstack/router-vite-plugin'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// 開発時はバックエンド (main.ts、デフォルト :3000) に API をプロキシする。
// 本番 build 成果物は backend が `/assets/*` と HTML shell として配信する。
export default defineConfig({
  plugins: [
    TanStackRouterVite({
      routesDirectory: 'src/routes',
      generatedRouteTree: 'src/routeTree.gen.ts',
    }),
    react(),
  ],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      // SPA が描画する画面 (login / consent / device / end_session) は
      // バックエンドが shell + meta を返す。Vite の dev 環境でもバックエンドを
      // 経由させることで CSRF Cookie と request_id を統一的に受け取れる。
      '/login': 'http://localhost:3000',
      '/totp': 'http://localhost:3000',
      '/consent': 'http://localhost:3000',
      '/device': 'http://localhost:3000',
      '/forgot_password': 'http://localhost:3000',
      '/reset_password': 'http://localhost:3000',
      '/account': 'http://localhost:3000',
      '/authorize': 'http://localhost:3000',
      '/end_session': 'http://localhost:3000',
      '/api': 'http://localhost:3000',
      '/.well-known': 'http://localhost:3000',
      '/jwks': 'http://localhost:3000',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    manifest: true,
  },
})
