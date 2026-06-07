export interface LoginSession {
  id: string
  sub: string
  auth_time: number
  /** OIDC Core §2: 認証成立時に経由した RFC 8176 メソッド列 (pwd / otp / webauthn 等)。 */
  amr: string[]
  /** annotations.acr_vocabulary に従って amr から導出した URN。step-up 判定で参照。 */
  acr: string
  expires_at: string
}
