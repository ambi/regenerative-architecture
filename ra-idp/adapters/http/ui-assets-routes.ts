/**
 * Layer 4 — Adapter Layer (HTTP: UI 静的アセット配信)
 *
 * Vite が `ui/dist/assets/` に出力する hashed bundle (JS / CSS / font / image) を
 * `/assets/*` で配信する。dist が未生成の環境では 404 を返すだけで、
 * 既存テスト (no-JS フォールバックで完結) には影響しない。
 *
 * 配置の責務分離:
 *   - ビルドは ui/ ディレクトリで完結 (Vite + tsc)
 *   - 配信は本ファイルが担当
 *   - SPA shell は spa-shell.ts が dist/index.html から asset タグを抽出
 */

import { existsSync, statSync } from 'node:fs'
import { dirname, join, normalize } from 'node:path'
import { fileURLToPath } from 'node:url'
import { Hono } from 'hono'

const UI_DIST_DIR = (() => {
  const here = dirname(fileURLToPath(import.meta.url))
  return join(here, '..', '..', 'ui', 'dist')
})()

const ASSETS_DIR = join(UI_DIST_DIR, 'assets')

const CONTENT_TYPES: Record<string, string> = {
  '.js': 'application/javascript; charset=UTF-8',
  '.mjs': 'application/javascript; charset=UTF-8',
  '.css': 'text/css; charset=UTF-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.map': 'application/json',
}

export function createUiAssetsRoutes() {
  const app = new Hono()

  app.get('/assets/*', async (c) => {
    // Hono のワイルドカードキャプチャから path を取り出す。
    const rest = c.req.path.replace(/^\/assets\//, '')
    // path traversal 対策: 正規化後に ASSETS_DIR の prefix を保ったまま。
    const requested = normalize(join(ASSETS_DIR, rest))
    if (!requested.startsWith(ASSETS_DIR + '/') && requested !== ASSETS_DIR) {
      return c.notFound()
    }
    if (!existsSync(requested) || !statSync(requested).isFile()) {
      return c.notFound()
    }
    const ext = requested.slice(requested.lastIndexOf('.'))
    const file = Bun.file(requested)
    return new Response(file, {
      headers: {
        'content-type': CONTENT_TYPES[ext] ?? 'application/octet-stream',
        'cache-control': 'public, max-age=31536000, immutable',
      },
    })
  })

  return app
}
