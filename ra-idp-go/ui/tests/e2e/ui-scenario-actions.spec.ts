// wi-75: 到達性スモークから一段進め、主要なブラウザ操作が API と接続されて
// ユーザー可視の成功状態へ到達することを検証する。
import { createHmac } from 'node:crypto'
import { afterAll, beforeAll, expect, test } from 'bun:test'
import {
  authorizePath,
  clickButtonByText,
  clickElementByAriaLabel,
  clickLinkByText,
  demo,
  navigateAndLogin,
  loginFromCurrentPage,
  selectDropdownOption,
  setCheckboxValue,
  setInputValue,
  setSelectValue,
  startE2EEnvironment,
  stopE2EEnvironment,
  uiOrigin,
  waitForPage,
  waitForUrl,
  waitForEmailURL,
  waitForText,
} from './fixtures'

function totpCode(secret: string, now = Date.now()): string {
  const counter = Math.floor(now / 1000 / 30)
  const key = decodeBase32(secret.replace(/\s+/g, ''))
  const message = Buffer.alloc(8)
  message.writeBigUInt64BE(BigInt(counter))
  const digest = createHmac('sha1', key).update(message).digest()
  const offset = digest[digest.length - 1] & 0x0f
  const binary =
    ((digest[offset] & 0x7f) << 24) |
    ((digest[offset + 1] & 0xff) << 16) |
    ((digest[offset + 2] & 0xff) << 8) |
    (digest[offset + 3] & 0xff)
  return String(binary % 1_000_000).padStart(6, '0')
}

function decodeBase32(value: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  let bits = ''
  for (const char of value.toUpperCase().replace(/=+$/, '')) {
    const index = alphabet.indexOf(char)
    if (index < 0) continue
    bits += index.toString(2).padStart(5, '0')
  }
  const bytes: number[] = []
  for (let i = 0; i + 8 <= bits.length; i += 8) {
    bytes.push(Number.parseInt(bits.slice(i, i + 8), 2))
  }
  return Buffer.from(bytes)
}

beforeAll(async () => {
  await startE2EEnvironment()
}, 180_000)

afterAll(() => {
  stopE2EEnvironment()
})

test('account profile can be updated from the browser', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await navigateAndLogin(view, '/account/profile', 'account-profile')

    const suffix = String(Date.now())
    const displayName = `Alice E2E ${suffix}`
    await clickButtonByText(view, '編集')
    await setInputValue(view, '#name', displayName)
    await setInputValue(view, '#given-name', 'Alice')
    await setInputValue(view, '#family-name', `Scenario ${suffix}`)
    await clickButtonByText(view, '保存')

    await waitForText(view, 'プロフィールを更新しました。')
    await waitForText(view, displayName)
  } finally {
    view.close()
  }
}, 60_000)

test('account data export is triggered from the browser', async () => {
  const view = new Bun.WebView({ width: 1280, height: 1600 })
  try {
    await navigateAndLogin(view, '/account/data', 'account-data')
    await view.evaluate(`(() => {
      window.__raDownloadClicked = false
      const original = HTMLAnchorElement.prototype.click
      HTMLAnchorElement.prototype.click = function () {
        window.__raDownloadClicked = true
        return original.call(this)
      }
    })()`)

    await clickButtonByText(view, 'データをダウンロード')

    const deadline = Date.now() + 10_000
    while (Date.now() < deadline) {
      if ((await view.evaluate('window.__raDownloadClicked === true')) === true) {
        return
      }
      await Bun.sleep(150)
    }
    throw new Error('timeout waiting for data export download trigger')
  } finally {
    view.close()
  }
}, 60_000)

test('admin general settings can be updated from the browser', async () => {
  const view = new Bun.WebView({ width: 1280, height: 1800 })
  try {
    await navigateAndLogin(view, '/admin/settings', 'admin-settings')

    const displayName = `Default organization ${Date.now()}`
    await clickButtonByText(view, '編集')
    await setInputValue(view, '#display-name', displayName)
    await clickButtonByText(view, '保存')

    await waitForText(view, '表示名を更新しました。')
    await waitForText(view, displayName)
  } finally {
    view.close()
  }
}, 60_000)

test('admin signing key rotation action is hidden from tenant admins', async () => {
  const view = new Bun.WebView({ width: 1280, height: 1800 })
  try {
    await navigateAndLogin(view, '/admin/keys', 'admin-keys')
    await waitForText(view, '署名鍵')
    expect(
      await view.evaluate(`(() => [...document.querySelectorAll('button')]
        .some((button) => (button.textContent ?? '').includes('ローテート')))()`),
    ).toBe(false)
  } finally {
    view.close()
  }
}, 60_000)

test('account connected application consent can be revoked from the browser', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await view.navigate(`${uiOrigin}${authorizePath(`consent-revoke-${Date.now()}`)}`)
    await loginFromCurrentPage(view)
    await waitForPage(view, 'consent')
    await clickButtonByText(view, '許可して続行')
    await waitForUrl(view, /localhost:3000\/callback/)

    await view.navigate(`${uiOrigin}/account/applications`)
    await waitForPage(view, 'account-applications')
    await waitForText(view, 'demo-client')
    await clickButtonByText(view, 'アクセスを取り消す')
    await waitForText(view, 'demo-client へのアクセスを取り消しました。')
    await waitForText(view, 'アクセスを許可したアプリはありません。')
  } finally {
    view.close()
  }
}, 60_000)

test('account TOTP enrollment and removal step-up work from the browser', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2200 })
  try {
    await navigateAndLogin(view, '/account/security', 'account-security')

    await clickButtonByText(view, '認証アプリを設定')
    await waitForText(view, 'セットアップキー')
    const secret = String(
      await view.evaluate('document.querySelector("#totp-secret")?.value ?? ""'),
    )
    expect(secret).not.toBe('')
    await setInputValue(view, '#totp-code', totpCode(secret))
    await clickButtonByText(view, '登録を完了')
    await waitForText(view, '認証アプリを登録しました。')

    await setInputValue(view, '#remove-code', totpCode(secret))
    await clickButtonByText(view, '認証アプリを解除')

    const deadline = Date.now() + 10_000
    while (Date.now() < deadline) {
      if (
        await view.evaluate(`document.body.textContent?.includes('本人確認のため再認証') ?? false`)
      ) {
        await setInputValue(view, '#step-up-credential', demo.password)
        await clickButtonByText(view, '再認証して続行')
      }
      if (
        await view.evaluate(
          `document.body.textContent?.includes('認証アプリを解除しました。') ?? false`,
        )
      ) {
        return
      }
      await Bun.sleep(150)
    }
    throw new Error('timeout waiting for TOTP removal')
  } finally {
    view.close()
  }
}, 60_000)

test('account session list can revoke a different browser session', async () => {
  const first = new Bun.WebView({ width: 1280, height: 1800 })
  const second = new Bun.WebView({ width: 1280, height: 1200 })
  try {
    await navigateAndLogin(first, '/account', 'account-home')
    await navigateAndLogin(second, '/account', 'account-home')

    await first.navigate(`${uiOrigin}/account/activity`)
    await waitForPage(first, 'account-activity')
    await waitForText(first, '他のセッションを終了')
    const beforeCount = Number(
      await first.evaluate(`(() => [...document.querySelectorAll('button')]
        .filter((button) => (button.textContent ?? '').trim() === '終了').length)()`),
    )
    expect(beforeCount).toBeGreaterThan(0)
    const clicked = await first.evaluate(`(() => {
      const target = [...document.querySelectorAll('button')]
        .find((button) => (button.textContent ?? '').trim() === '終了')
      if (!target) return false
      target.click()
      return true
    })()`)
    expect(clicked).toBe(true)
    const deadline = Date.now() + 10_000
    while (Date.now() < deadline) {
      const afterCount = Number(
        await first.evaluate(`(() => [...document.querySelectorAll('button')]
          .filter((button) => (button.textContent ?? '').trim() === '終了').length)()`),
      )
      if (afterCount < beforeCount) return
      await Bun.sleep(150)
    }
    throw new Error('timeout waiting for revoked session row count to decrease')
  } finally {
    first.close()
    second.close()
  }
}, 60_000)

test('admin audit log can be filtered and export can be triggered', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await navigateAndLogin(view, '/admin/audit_events', 'admin-audit-events')
    await view.evaluate(`(() => {
      window.__raAuditExportURL = ''
      window.open = (url) => {
        window.__raAuditExportURL = String(url ?? '')
        return null
      }
    })()`)

    await setSelectValue(view, 'select', 'authentication')
    await setInputValue(view, 'input[placeholder="例: user_..."]', 'user_alice')
    await clickButtonByText(view, '絞り込み')
    await waitForText(view, 'UserAuthenticated')

    await clickButtonByText(view, 'エクスポート')
    const exportURL = await view.evaluate('window.__raAuditExportURL ?? ""')
    expect(String(exportURL)).toContain('/api/admin/audit_events/export')
    expect(String(exportURL)).toContain('category=authentication')
    expect(String(exportURL)).toContain('sub=user_alice')
  } finally {
    view.close()
  }
}, 60_000)

test('admin user attribute schema can add and delete a custom attribute', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2200 })
  try {
    await navigateAndLogin(view, '/admin/tenant/attributes', 'admin-tenant-attributes')

    const key = `e2e_attr_${Date.now()}`
    await clickButtonByText(view, '属性を追加')
    await setInputValue(view, '#attr-label', 'E2E 属性')
    await setInputValue(view, '#attr-key', key)
    await setSelectValue(view, '#attr-type', 'string')
    await setSelectValue(view, '#attr-visibility', 'self_readable')
    await setCheckboxValue(view, '#attr-editable', true)
    await clickButtonByText(view, '保存')

    await waitForText(view, '属性を追加しました。')
    await waitForText(view, key)

    await clickElementByAriaLabel(view, `${key} を削除`)
    await waitForText(view, '属性を削除しました。')
    await waitForText(view, 'カスタム属性はまだありません。')
  } finally {
    view.close()
  }
}, 60_000)

test('account email change confirms through the local SMTP sink', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2000 })
  try {
    await navigateAndLogin(view, '/account/emails', 'account-emails')

    const nextEmail = `alice.e2e.${Date.now()}@example.com`
    await clickButtonByText(view, '変更')
    await setInputValue(view, '#new-email', nextEmail)
    await clickButtonByText(view, '確認メールを送信')

    const deadline = Date.now() + 10_000
    while (Date.now() < deadline) {
      if (
        await view.evaluate(`document.body.textContent?.includes('本人確認のため再認証') ?? false`)
      ) {
        await setInputValue(view, '#step-up-credential', demo.password)
        await clickButtonByText(view, '再認証して続行')
        break
      }
      if (
        await view.evaluate(
          `document.body.textContent?.includes(${JSON.stringify(nextEmail)}) ?? false`,
        )
      ) {
        break
      }
      await Bun.sleep(150)
    }

    await waitForText(view, nextEmail)
    const verifyURL = await waitForEmailURL(nextEmail, '/account/email/verify')
    await view.navigate(verifyURL)
    await waitForPage(view, 'email-verify')
    await clickButtonByText(view, 'メールアドレスを確認する')
    await waitForText(view, 'メールアドレスを確認しました。')
    demo.email = nextEmail
  } finally {
    view.close()
  }
}, 60_000)

test('password reset succeeds through the local SMTP sink without external mail', async () => {
  const view = new Bun.WebView({ width: 1280, height: 1800 })
  try {
    const suffix = Date.now()
    const username = `reset-e2e-${suffix}`
    const email = `reset.e2e.${suffix}@example.com`
    const initialPassword = `initial-password-${suffix}`
    const nextPassword = `reset-password-${suffix}`

    await navigateAndLogin(view, '/admin/users', 'admin-users')
    await clickButtonByText(view, 'ユーザーを追加')
    await setInputValue(view, 'input[name="preferred_username"]', username)
    await setInputValue(view, 'input[name="name"]', 'Reset E2E')
    await setInputValue(view, 'input[name="email"]', email)
    await setInputValue(view, 'input[name="password"]', initialPassword)
    await setCheckboxValue(view, 'input[name="email_verified"]', true)
    await clickButtonByText(view, '作成')
    await waitForText(view, 'ユーザーを作成しました。')

    await view.navigate(`${uiOrigin}/forgot_password`)
    await waitForPage(view, 'forgot-password')
    await setInputValue(view, 'input[name="email"]', email)
    await clickButtonByText(view, 'リセットリンクを送信')
    await waitForText(view, 'アカウントが確認できた場合')

    const resetURL = await waitForEmailURL(email, '/reset_password')
    await view.navigate(resetURL)
    await waitForPage(view, 'reset-password')
    await setInputValue(view, 'input[name="new_password"]', nextPassword)
    await clickButtonByText(view, 'パスワードを更新')
    await waitForText(view, 'パスワードを更新しました。')
  } finally {
    view.close()
  }
}, 60_000)

test('admin application lifecycle and agent credential binding work from the browser', async () => {
  const view = new Bun.WebView({ width: 1280, height: 2400 })
  try {
    const suffix = Date.now()
    const appName = `E2E OIDC App ${suffix}`
    const agentName = `e2e-agent-${suffix}`

    await navigateAndLogin(view, '/admin/applications', 'admin-applications')
    await clickButtonByText(view, 'アプリケーションを追加')
    await setInputValue(view, '#app-name', appName)
    await setInputValue(view, '#app-redirects', `https://client.example.test/callback/${suffix}`)
    await setInputValue(view, '#app-oidc-scope', 'openid profile email')
    await clickButtonByText(view, '作成')
    await waitForText(view, 'クライアントを作成しました。')

    const clientID = String(
      await view.evaluate(`(() => {
        const values = [...document.querySelectorAll('code')]
          .map((node) => node.textContent?.trim() ?? '')
          .filter(Boolean)
        return values[0] ?? ''
      })()`),
    )
    expect(clientID).not.toBe('')

    await clickButtonByText(view, '保管しました')
    await waitForUrl(view, /\/admin\/applications\/[^/]+$/)
    const appDetailURL = view.url
    await waitForText(view, appName)
    await waitForText(view, clientID)
    await clickLinkByText(view, '編集')
    await waitForUrl(view, /\/admin\/applications\/[^/]+\/edit$/)
    await selectDropdownOption(view, '選択…', demo.username)
    await clickButtonByText(view, '割り当て')
    await waitForText(view, demo.username)

    await view.navigate(`${uiOrigin}/admin/agents`)
    await waitForPage(view, 'admin-agents')
    await clickButtonByText(view, '登録')
    await setInputValue(view, '#agent-name', agentName)
    await setInputValue(view, '#agent-description', 'E2E credential binding')
    await setSelectValue(view, '#agent-kind', 'supervised')
    await setInputValue(view, '#agent-roles', 'e2e:read, e2e:write')
    await view.click('form button[type="submit"]')
    await waitForText(view, 'エージェントを登録しました。')
    await waitForText(view, agentName)

    await setInputValue(view, 'input[aria-label="バインドする client_id"]', clientID)
    await clickButtonByText(view, 'バインド')
    await waitForText(view, clientID)

    await clickButtonByText(view, '解除')
    await waitForText(view, 'バインドされた資格情報はありません。')

    await view.navigate(appDetailURL)
    await waitForText(view, appName)
    await clickButtonByText(view, '削除')
    await clickButtonByText(view, '削除を確定')
    await waitForPage(view, 'admin-applications')
  } finally {
    view.close()
  }
}, 90_000)
