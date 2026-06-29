// wi-22: SPA E2E スモーク 1 本。golden path (authorize -> login -> consent ->
// callback) を Bun.WebView (macOS: WKWebView / その他: Chrome via CDP) で縛り、
// (a) SPA dispatcher の画面分岐と (b) cross-origin redirect での code / iss 保持
// の 2 領域の回帰を機械検知する。外部のブラウザ自動化フレームワークや別ブラウザの取得は不要。
//
// README のローカル開発手順と同じ構成で起動する:
//   ADDR=:8081 ISSUER=http://localhost:5173 go run ./cmd/ra-idp-go
//   bun run dev   (Vite が /authorize・/api を 8081 にプロキシ)
// ISSUER を 5173 にするのはブラウザ origin と一致させて CSRF/origin 検査を
// 通すため (verifyBrowserRequest)。
import { afterAll, beforeAll, expect, test } from 'bun:test'
import {
  authorizePath,
  clickButtonByText,
  clickNavLinkByText,
  demo,
  loginFromCurrentPage,
  startE2EEnvironment,
  stopE2EEnvironment,
  uiOrigin,
  waitForLocationPath,
  waitForPage,
  waitForUrl,
} from './fixtures'

beforeAll(async () => {
  await startE2EEnvironment()
}, 180_000)

afterAll(() => {
  stopE2EEnvironment()
})

test('authorize golden path: login -> consent -> callback keeps code and iss', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await view.navigate(`${uiOrigin}${authorizePath('e2e-state')}`)

    // dispatcher 不変条件: ログイン画面が描画される。
    await waitForPage(view, 'login')
    expect(await view.evaluate('!!document.querySelector(\'input[name="username"]\')')).toBe(true)
    await loginFromCurrentPage(view)

    // dispatcher 不変条件: 同意画面へ遷移する。
    await waitForPage(view, 'consent')

    await clickButtonByText(view, '許可して続行')

    // cross-origin redirect (5173 -> 3000) で code / iss が落ちないこと (RFC 9207)。
    await waitForUrl(view, /localhost:3000\/callback/)
    const callbackUrl = new URL(view.url)
    expect(callbackUrl.searchParams.get('code')).toBeTruthy()
    expect(callbackUrl.searchParams.get('iss')).toBeTruthy()
    expect(callbackUrl.searchParams.get('state')).toBe('e2e-state')
  } finally {
    view.close()
  }
}, 60_000)

// wi-67: 管理コンソールのサイドバー遷移が client-side (フルリロード無し) であること。
// /admin は OIDC RP としてログインし (ADR-061)、ログイン後はサイドバーの Link 遷移が
// ページを再読込せず、対象 route のデータだけを取得することを検証する。
test('admin sidebar navigation is client-side (no full reload)', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    // 管理コンソールを開く → OIDC RP として /authorize 経由でログイン画面へ。
    await view.navigate(`${uiOrigin}/admin`)
    await waitForPage(view, 'login')
    await view.click('input[name="username"]')
    await view.type(demo.username)
    await view.click('input[name="password"]')
    await view.type(demo.password)
    await view.click('button[type="submit"]')

    // first-party クライアントは consent をスキップし、/callback で token 交換して
    // ダッシュボードへ戻る。
    await waitForPage(view, 'admin-dashboard')

    // フルリロード検出マーカー。client-side 遷移なら保持され、再読込なら消える。
    await view.evaluate('window.__raSpaMarker = "kept"')

    // サイドバーの「ユーザー」リンクを click (TanStack Link → client-side 遷移)。
    await clickNavLinkByText(view, '管理メニュー', 'ユーザー')

    // client-side 遷移は history.pushState のため WebView の view.url には出ない。
    // 実際の location.pathname とページ種別で遷移完了を判定する。
    await waitForLocationPath(view, '/admin/users')
    await waitForPage(view, 'admin-users')

    // 遷移後もマーカーが残っていれば、フルリロードしていない (= client-side 遷移)。
    expect(await view.evaluate('window.__raSpaMarker ?? null')).toBe('kept')
  } finally {
    view.close()
  }
}, 60_000)
