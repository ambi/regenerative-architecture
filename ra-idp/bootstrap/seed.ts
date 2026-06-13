/**
 * Layer 5 — Runtime: デモ用クライアント / ユーザーを 1 件投入する。
 * 本番想定で SKIP_DEMO_SEED が設定されていれば呼び出し側がスキップする。
 */

import { createHash } from 'crypto'

import type { Argon2idPasswordHasher } from '../adapters/crypto/argon2id-password-hasher'
import type { ClientRepository } from '../src/oauth2/ports/client-repository'
import type { UserRepository } from '../src/authentication/ports/user-repository'
import type { MfaFactorRepository } from '../src/authentication/ports/mfa-factor-repository'
import {
  PasswordPolicyError,
  validatePassword,
} from '../src/authentication/usecases/password-policy'
import { buildOtpauthUri } from '../src/authentication/usecases/totp'
import { ClientSchema, UserSchema } from '../src/spec-bindings/schemas'

export async function seedDemoData(
  deps: {
    clientRepo: ClientRepository
    userRepo: UserRepository
    mfaFactorRepo: MfaFactorRepository
  },
  passwordHasher: Argon2idPasswordHasher,
): Promise<void> {
  const demoClientSecret = process.env.DEMO_CLIENT_SECRET ?? 'demo-secret-please-rotate'
  const demoClient = ClientSchema.parse({
    tenant_id: 'default',
    client_id: 'demo-web-app',
    client_secret_hash: createHash('sha256').update(demoClientSecret).digest('hex'),
    client_name: 'Demo Web Application',
    client_type: 'confidential',
    redirect_uris: ['http://localhost:8080/callback', 'https://app.example.com/callback'],
    grant_types: [
      'authorization_code',
      'refresh_token',
      'client_credentials',
      'urn:ietf:params:oauth:grant-type:device_code',
    ],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile email offline_access',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
  })
  await deps.clientRepo.save(demoClient)

  const demoPassword = process.env.DEMO_USER_PASSWORD ?? 'demo-password-1234'
  const policy = validatePassword(demoPassword, {
    username: 'alice',
    email: 'alice@example.com',
  })
  if (!policy.ok) throw new PasswordPolicyError(policy.violations)
  // DEMO_TOTP_SECRET (base32) を渡せば alice に TOTP factor を仕込み、
  // パスワード後に /totp challenge へ誘導するデモを有効化する。otpauth:// URI を
  // 起動ログに出すので、それを Authenticator アプリ (Google Authenticator 等) に
  // 入れて 6 桁コードを発行できる。未指定なら mfa_enrolled=false のまま。
  const demoTotpSecret = process.env.DEMO_TOTP_SECRET
  const demoUser = UserSchema.parse({
    sub: 'user_alice',
    tenant_id: 'default',
    preferred_username: 'alice',
    password_hash: await passwordHasher.hash(demoPassword),
    name: 'Alice Demo',
    given_name: 'Alice',
    family_name: 'Demo',
    email: 'alice@example.com',
    email_verified: true,
    mfa_enrolled: !!demoTotpSecret,
    // ADR-031: デモ環境では alice に admin ロールを付与し、/admin/users を試せるようにする。
    // 本番では明示的な bootstrap 手順で admin ユーザーを作成する。
    roles: ['admin'],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
  await deps.userRepo.save(demoUser)

  if (demoTotpSecret) {
    await deps.mfaFactorRepo.save({
      sub: 'user_alice',
      type: 'totp',
      secret: demoTotpSecret,
      label: 'Demo Authenticator',
      created_at: new Date().toISOString(),
    })
    const otpauthUri = buildOtpauthUri({
      secretBase32: demoTotpSecret,
      accountName: demoUser.preferred_username,
      issuer: 'RA IdP (demo)',
    })
    console.log(`[seed] TOTP factor enrolled for alice. otpauth URI: ${otpauthUri}`)
  }
}
