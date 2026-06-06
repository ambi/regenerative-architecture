/**
 * Layer 5 — Runtime: デモ用クライアント / ユーザーを 1 件投入する。
 * 本番想定で SKIP_DEMO_SEED が設定されていれば呼び出し側がスキップする。
 */

import { createHash } from 'crypto'

import { Argon2idPasswordHasher } from '../adapters/crypto/argon2id-password-hasher'
import type { ClientRepository } from '../src/oauth2/ports/client-repository'
import type { UserRepository } from '../src/authentication/ports/user-repository'
import {
  PasswordPolicyError,
  validatePassword,
} from '../src/authentication/usecases/password-policy'
import { ClientSchema, UserSchema } from '../src/spec-bindings/schemas'

export async function seedDemoData(
  deps: { clientRepo: ClientRepository; userRepo: UserRepository },
  passwordHasher: Argon2idPasswordHasher,
): Promise<void> {
  const demoClientSecret = process.env.DEMO_CLIENT_SECRET ?? 'demo-secret-please-rotate'
  const demoClient = ClientSchema.parse({
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

  const demoPassword = process.env.DEMO_USER_PASSWORD ?? 'alice-password'
  const policy = validatePassword(demoPassword)
  if (!policy.ok) throw new PasswordPolicyError(policy.violations)
  const demoUser = UserSchema.parse({
    sub: 'user_alice',
    preferred_username: 'alice',
    password_hash: await passwordHasher.hash(demoPassword),
    name: 'Alice Demo',
    given_name: 'Alice',
    family_name: 'Demo',
    email: 'alice@example.com',
    email_verified: true,
    mfa_enrolled: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
  await deps.userRepo.save(demoUser)
}
