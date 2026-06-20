// golden path で共有する定数とヘルパー (wi-22)。
// デモシード (internal/bootstrap/seed.go) と redirect_uri 登録に整合させる。
import { createHash } from 'node:crypto'

export const demo = {
  clientId: 'demo-client',
  username: 'alice',
  password: 'demo-password-1234',
  redirectUri: 'http://localhost:3000/callback',
  scope: 'openid profile email offline_access',
}

// /authorize は PKCE 必須 (routes_e2e_test.go と同条件)。本スモークは
// 認可コードの token 交換まではせず callback URL の code / iss を見るだけなので、
// 固定 verifier の S256 challenge を載せれば十分。
const verifier = 'ra-idp-e2e-pkce-verifier-0123456789abcdefghij'
const codeChallenge = createHash('sha256').update(verifier).digest('base64url')

export function authorizePath(state: string): string {
  const params = new URLSearchParams({
    client_id: demo.clientId,
    redirect_uri: demo.redirectUri,
    response_type: 'code',
    scope: demo.scope,
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
  })
  return `/authorize?${params.toString()}`
}
