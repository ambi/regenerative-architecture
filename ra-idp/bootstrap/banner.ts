/**
 * Layer 5 — Runtime: 起動時に主要エンドポイント一覧と現在の構成を stdout に出力する。
 */

import type { RuntimeConfig } from './config'

export function printStartupBanner(config: RuntimeConfig): void {
  // eslint-disable-next-line no-console
  console.log(`\nOAuth2 / OIDC IdP — ${config.issuer}`)
  // eslint-disable-next-line no-console
  console.log(
    `persistence=${config.persistenceMode}  event_sink=${config.eventSinkMode}  observability=${config.observabilityMode}`,
  )
  // eslint-disable-next-line no-console
  console.log('\n主要エンドポイント:')
  // eslint-disable-next-line no-console
  console.log(`  GET    /.well-known/openid-configuration  Discovery (OIDC)`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /.well-known/oauth-authorization-server  Discovery (OAuth2)`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /jwks                              公開鍵 (JWKS)`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /register                          クライアント登録`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /device_authorization              デバイス認可リクエスト (RFC 8628)`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /device                            デバイス認可 verification_uri`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /authorize                         認可エンドポイント`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /login                             パスワードログイン`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /end_session                       RP-Initiated Logout`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /par                               Pushed Authorization Request`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /token                             トークンエンドポイント`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /introspect                        トークン introspection`)
  // eslint-disable-next-line no-console
  console.log(`  POST   /revoke                            トークン失効`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /userinfo                          UserInfo (OIDC)`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /health                            ヘルスチェック`)
  // eslint-disable-next-line no-console
  console.log(`  GET    /events                            イベント履歴 (memory モードのみ)`)
  // eslint-disable-next-line no-console
  console.log(`\nデモ:`)
  // eslint-disable-next-line no-console
  console.log(`  client_id     = demo-web-app`)
  // eslint-disable-next-line no-console
  console.log(`  client_secret = ${process.env.DEMO_CLIENT_SECRET ?? 'demo-secret-please-rotate'}`)
  // eslint-disable-next-line no-console
  console.log(`  user          = alice (X-User-Sub: user_alice)`)
  // eslint-disable-next-line no-console
  console.log(`\nテストドライバー: ./demo.sh`)
}
