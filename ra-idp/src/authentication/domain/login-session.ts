export interface LoginSession {
  id: string
  tenant_id: string
  sub: string
  auth_time: number
  /** OIDC Core §2: 認証成立時に経由した RFC 8176 メソッド列 (pwd / otp / webauthn 等)。 */
  amr: string[]
  /** annotations.acr_vocabulary に従って amr から導出した URN。step-up 判定で参照。 */
  acr: string
  /** 追加 factor 検証を待っている中間状態か。完了で false。/authorize 完了経路は true を未認証扱いとする。 */
  authentication_pending?: boolean
  expires_at: string
}
