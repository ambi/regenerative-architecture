/**
 * Layer 4 — Adapter Layer (HTTP: SPA shell renderer)
 *
 * 認証系画面 (login / consent / device / error) は ui/ ディレクトリの
 * React + Vite SPA に集約する。バックエンドはサーバ側状態 (request_id・CSRF
 * トークン・consent 文脈など) を `<meta name="ra-idp:*">` で SPA に渡し、
 * Vite build 後の dist/index.html を最終的なシェルとして配信する。
 *
 * SPA が読み込まれない環境 (テスト、no-JS、UI 未ビルド) でもフローを
 * 継続できるよう、本ファイルは隠しフォーム入力 (`<input name="csrf">` 等)
 * を同じシェルに出力する。spec/scl.yaml のシナリオテストはこの hidden
 * input を回帰チェックで使う。
 */

import { readFileSync, existsSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { brandFor, negotiateLocale, type SupportedLocale } from './branding'

const UI_DIST_DIR = (() => {
  // adapters/http から見て ../../ui/dist
  const here = dirname(fileURLToPath(import.meta.url))
  return join(here, '..', '..', 'ui', 'dist')
})()

interface ParsedShell {
  /** dist/index.html を読み込んで `<head>` の直前に挿入できる asset タグ集。 */
  assetTags: string
}

let cachedShell: ParsedShell | null = null

function loadParsedShell(): ParsedShell {
  if (cachedShell) return cachedShell
  const indexPath = join(UI_DIST_DIR, 'index.html')
  if (!existsSync(indexPath)) {
    cachedShell = { assetTags: '' }
    return cachedShell
  }
  const html = readFileSync(indexPath, 'utf8')
  // dist/index.html から <script type="module" src="..."> と <link rel="stylesheet"> を抜く
  const tags: string[] = []
  for (const m of html.matchAll(/<link[^>]+rel=["']stylesheet["'][^>]*>/g)) tags.push(m[0])
  for (const m of html.matchAll(/<script[^>]+type=["']module["'][^>]*src=[^>]+><\/script>/g))
    tags.push(m[0])
  cachedShell = { assetTags: tags.join('\n    ') }
  return cachedShell
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

export interface ShellInput {
  /** ページ名 (title 表示と SPA ルーティング判定の hint)。 */
  page:
    | 'login'
    | 'totp'
    | 'consent'
    | 'device'
    | 'change-password'
    | 'forgot-password'
    | 'reset-password'
    | 'admin-users'
    | 'error'
  title: string
  /** SPA / 隠しフォームに伝える初期 props。 */
  meta: Record<string, string>
  /**
   * SPA が読み込まれない環境 (テスト / no-JS) で当該ページのフローを
   * 続行するための hidden form。method / action と name=value 群を渡す。
   * `buttons` を渡すと `<noscript>` フォーム内に複数の送信ボタンを並べる
   * (consent 画面の allow / deny など)。
   */
  fallbackForm?: {
    action: string
    fields: Record<string, string>
    buttons?: Array<{ name: string; value: string; label: string }>
  }
  /** ロケール選択用 (`Accept-Language` を渡せばここで解決する)。 */
  acceptLanguage?: string
  /** Phase 5 でテナント解決に切り替えるため `brandFor()` の引数を露出。 */
  tenantId?: string | null
}

/**
 * 認証画面用の HTML shell を組み立てる。
 *
 * Vite の dist/index.html が存在すれば asset link / script タグを挿入し、
 * 無ければ HTML だけ (no-JS フォールバック相当) を返す。
 */
export function renderShell(input: ShellInput): string {
  const { assetTags } = loadParsedShell()
  const brand = brandFor(input.tenantId)
  const locale: SupportedLocale = negotiateLocale(input.acceptLanguage, brand.defaultLocale)
  const combinedMeta: Record<string, string> = {
    ...input.meta,
    locale,
    'brand-name': brand.name,
    ...(brand.logoUrl ? { 'brand-logo': brand.logoUrl } : {}),
  }
  const metaTags = Object.entries(combinedMeta)
    .map(([k, v]) => `<meta name="ra-idp:${escapeHtml(k)}" content="${escapeHtml(v)}">`)
    .join('\n    ')
  // brand primary 色は CSS 変数の "H S% L%" 形式を `<style>` で上書きする。
  // テナント未設定時 (デフォルト) は注入しない (元のトークンを使う)。
  const brandStyle = brand.primaryHsl
    ? `<style>:root { --primary: ${escapeHtml(brand.primaryHsl)}; --ring: ${escapeHtml(brand.primaryHsl)}; }</style>`
    : ''

  const fallback = input.fallbackForm
    ? (() => {
        const fields = Object.entries(input.fallbackForm.fields)
          .map(
            ([k, v]) =>
              `          <input type="hidden" name="${escapeHtml(k)}" value="${escapeHtml(v)}">`,
          )
          .join('\n')
        const buttons = (input.fallbackForm.buttons ?? [])
          .map(
            (b) =>
              `          <button type="submit" name="${escapeHtml(b.name)}" value="${escapeHtml(b.value)}">${escapeHtml(b.label)}</button>`,
          )
          .join('\n')
        return `
      <noscript>
        <form method="POST" action="${escapeHtml(input.fallbackForm.action)}">
${fields}
${buttons}
          <p>このフォームの送信には JavaScript を有効化するか、フィールドを補って手動送信してください。</p>
        </form>
      </noscript>`
      })()
    : ''

  // テスト・後方互換: `<input name="csrf" value="...">` を SPA shell 内に直接配置することで、
  // SPA がマウントされていない bun test 環境でもフォームを回収できる。
  const hiddenInputs = input.fallbackForm
    ? Object.entries(input.fallbackForm.fields)
        .map(([k, v]) => `<input type="hidden" name="${escapeHtml(k)}" value="${escapeHtml(v)}">`)
        .join('\n    ')
    : ''

  return `<!doctype html>
<html lang="${escapeHtml(locale)}" class="h-full">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${escapeHtml(input.title)} - ${escapeHtml(brand.name)}</title>
    <meta name="ra-idp:page" content="${escapeHtml(input.page)}">
    ${metaTags}
    ${assetTags}
    ${brandStyle}
  </head>
  <body class="h-full bg-background text-foreground">
    <a href="#main-content" class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-card focus:px-3 focus:py-2 focus:text-sm focus:ring-2 focus:ring-ring">${
      locale === 'ja' ? 'メインコンテンツへ移動' : 'Skip to main content'
    }</a>
    <div id="root"></div>
    ${hiddenInputs}
    ${fallback}
  </body>
</html>`
}
