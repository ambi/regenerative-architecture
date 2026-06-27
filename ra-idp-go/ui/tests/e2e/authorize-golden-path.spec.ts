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
import { spawn, spawnSync, type Subprocess } from 'bun'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { authorizePath, demo } from './fixtures'

const here = dirname(fileURLToPath(import.meta.url))
const uiDir = resolve(here, '../..') // ra-idp-go/ui
const goDir = resolve(here, '../../..') // ra-idp-go

const uiOrigin = 'http://localhost:5173'
const apiHealth = 'http://localhost:8081/health'
const callbackPort = 3000

let goServer: Subprocess | undefined
let viteServer: Subprocess | undefined
let goBinary: string | undefined
let callback: ReturnType<typeof Bun.serve> | undefined

async function isUp(url: string): Promise<boolean> {
  try {
    return (await fetch(url)).ok
  } catch {
    return false
  }
}

async function waitForUp(url: string, timeoutMs = 120_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (await isUp(url)) {
      return
    }
    await Bun.sleep(500)
  }
  throw new Error(`timeout waiting for ${url}`)
}

beforeAll(async () => {
  // demo-client の redirect_uri (http://localhost:3000/callback) を受ける最小サーバ。
  // 認可レスポンスの URL はブラウザ側 (view.url) から読むため、ここは 200 を返して
  // 接続拒否を防ぐだけ。既に何かが listen していれば再利用する。
  if (!(await isUp(`http://localhost:${callbackPort}/health`))) {
    try {
      callback = Bun.serve({ port: callbackPort, fetch: () => new Response('received') })
    } catch {
      // ポートが既に使用中なら既存のものを使う。
    }
  }

  // go run は子プロセス (ビルド済みバイナリ) を孫として起動するため kill が届きにくい。
  // 事前にバイナリへビルドして直接起動し、afterAll で確実に停止できるようにする。
  if (!(await isUp(apiHealth))) {
    goBinary = join(tmpdir(), `ra-idp-go-e2e-${process.pid}`)
    const build = spawnSync(['go', 'build', '-o', goBinary, './cmd/ra-idp-go'], { cwd: goDir })
    if (build.exitCode !== 0) {
      throw new Error(`go build failed: ${build.stderr?.toString() ?? ''}`)
    }
    goServer = spawn([goBinary], {
      cwd: goDir,
      env: {
        ...process.env,
        ADDR: ':8081',
        ISSUER: uiOrigin,
        PERSISTENCE: 'memory',
      },
      stdout: 'ignore',
      stderr: 'ignore',
    })
  }

  if (!(await isUp(uiOrigin))) {
    viteServer = spawn(['bun', 'run', 'dev'], { cwd: uiDir, stdout: 'ignore', stderr: 'ignore' })
  }

  await waitForUp(apiHealth)
  await waitForUp(uiOrigin)
}, 180_000)

afterAll(() => {
  goServer?.kill()
  viteServer?.kill()
  callback?.stop(true)
})

// metaPage は SPA dispatcher が表明する <meta name="ra-idp:page"> の content を読む。
function metaPage(view: Bun.WebView): Promise<unknown> {
  return view.evaluate(
    'document.querySelector(\'meta[name="ra-idp:page"]\')?.getAttribute("content") ?? null',
  )
}

// waitForPage は SPA の route loader と page marker の完了まで待つ。
// window.location.assign による完全遷移後、meta が更新されるのは fetch 解決後のため。
async function waitForPage(view: Bun.WebView, kind: string, timeoutMs = 15_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      if ((await metaPage(view)) === kind) {
        return
      }
    } catch {
      // 遷移中は evaluate が失敗しうる。リトライする。
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for page kind=${kind}, last url=${view.url}`)
}

async function waitForUrl(view: Bun.WebView, pattern: RegExp, timeoutMs = 15_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (pattern.test(view.url)) {
      return
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for url ${pattern}, last=${view.url}`)
}

// waitForLocationPath は client-side 遷移 (history.pushState) 後の window.location.pathname を
// 待つ。Bun.WebView の view.url は top-level 遷移しか追わないため、pushState では更新されない。
async function waitForLocationPath(
  view: Bun.WebView,
  expected: string,
  timeoutMs = 15_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if ((await view.evaluate('window.location.pathname')) === expected) {
      return
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for location.pathname=${expected}`)
}

// clickButtonByText は表示文言からボタンを特定して click する。consent の許可ボタンは
// type=button で固有の CSS セレクタを持たないため text で引く。React の onClick は
// プログラム的 click でも発火する。
async function clickButtonByText(view: Bun.WebView, text: string): Promise<void> {
  const clicked = await view.evaluate(`(() => {
    const target = [...document.querySelectorAll('button')]
      .find((b) => (b.textContent ?? '').includes(${JSON.stringify(text)}))
    if (!target) return false
    target.click()
    return true
  })()`)
  if (clicked !== true) {
    throw new Error(`button not found: ${text}`)
  }
}

test('authorize golden path: login -> consent -> callback keeps code and iss', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await view.navigate(`${uiOrigin}${authorizePath('e2e-state')}`)

    // dispatcher 不変条件: ログイン画面が描画される。
    await waitForPage(view, 'login')
    expect(await view.evaluate('!!document.querySelector(\'input[name="username"]\')')).toBe(true)

    await view.click('input[name="username"]')
    await view.type(demo.username)
    await view.click('input[name="password"]')
    await view.type(demo.password)
    await view.click('button[type="submit"]')

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
    const clicked = await view.evaluate(`(() => {
      const link = [...document.querySelectorAll('nav[aria-label="管理メニュー"] a')]
        .find((a) => (a.textContent ?? '').trim() === 'ユーザー')
      if (!link) return false
      link.click()
      return true
    })()`)
    expect(clicked).toBe(true)

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
