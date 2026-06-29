// wi-75: SCL に追加済みの UI シナリオを、まず画面到達性と主要導線の
// ブラウザ E2E として固定する。低レベル usecase の重複検査ではなく、
// SPA route loader、OIDC RP ログイン、サイドバー遷移、フォーム送信の接続を検証する。
import { afterAll, beforeAll, test } from 'bun:test'
import {
  clickNavLinkByText,
  demo,
  startE2EEnvironment,
  stopE2EEnvironment,
  uiOrigin,
  waitForLocationPath,
  waitForPage,
  waitForText,
  navigateAndLogin,
} from './fixtures'

beforeAll(async () => {
  await startE2EEnvironment()
}, 180_000)

afterAll(() => {
  stopE2EEnvironment()
})

test('login assistance pages render and forgot password has enumeration-safe success copy', async () => {
  const view = new Bun.WebView({ width: 1280, height: 1600 })
  try {
    await view.navigate(`${uiOrigin}/forgot_password`)
    await waitForPage(view, 'forgot-password')
    await view.click('input[name="email"]')
    await view.type(demo.email)
    await view.click('button[type="submit"]')

    await waitForText(view, 'アカウントが確認できた場合')

    await view.navigate(`${uiOrigin}/reset_password`)
    await waitForPage(view, 'reset-password')

    await waitForText(view, 'リセットリンクが不正です。')
  } finally {
    view.close()
  }
}, 60_000)

test('account portal scenarios are reachable after account-audience login', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await navigateAndLogin(view, '/account', 'account-home')

    const pages = [
      ['アプリ', '/account/apps', 'account-apps'],
      ['アカウント情報', '/account/profile', 'account-profile'],
      ['メールアドレス', '/account/emails', 'account-emails'],
      ['セキュリティ', '/account/security', 'account-security'],
      ['アクティビティ', '/account/activity', 'account-activity'],
      ['接続済みアプリ', '/account/applications', 'account-applications'],
      ['データとプライバシー', '/account/data', 'account-data'],
    ] as const

    for (const [label, path, marker] of pages) {
      await clickNavLinkByText(view, 'マイページメニュー', label)
      await waitForLocationPath(view, path)
      await waitForPage(view, marker)
    }
  } finally {
    view.close()
  }
}, 90_000)

test('admin console scenarios are reachable after admin-audience login', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2200 })
  try {
    await navigateAndLogin(view, '/admin', 'admin-dashboard')

    const pages = [
      ['アプリケーション', '/admin/applications', 'admin-applications'],
      ['エージェント', '/admin/agents', 'admin-agents'],
      ['監査ログ', '/admin/audit_events', 'admin-audit-events'],
      ['署名鍵', '/admin/keys', 'admin-keys'],
      ['ユーザー属性', '/admin/tenant/attributes', 'admin-tenant-attributes'],
      ['設定', '/admin/settings', 'admin-settings'],
    ] as const

    for (const [label, path, marker] of pages) {
      await clickNavLinkByText(view, '管理メニュー', label)
      await waitForLocationPath(view, path)
      await waitForPage(view, marker)
    }
  } finally {
    view.close()
  }
}, 90_000)
