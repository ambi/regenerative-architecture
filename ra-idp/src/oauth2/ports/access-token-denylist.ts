/**
 * Layer 3 — Port: Access Token Denylist (JWT 即時失効)
 *
 * JWT access token は self-contained のため通常は revocation 不可だが、
 * 侵害時の即時失効が必要なケースのために jti を denylist で管理する。
 * 期限切れエントリは保持不要 (introspect 時に exp で別途弾かれるため)。
 *
 * 永続化先:
 *   - InMemory (dev / test): Map<jti, expiresAt>
 *   - Redis (prod): SET idp:at:denylist:<jti> 1 EX <ttl>
 *
 * RFC 7009 §6 にあるとおり revocation request での JWT サポートは任意。
 * ここでは "/revoke で access_token を受け取ったら denylist に積み、
 * /introspect 経路で jti をチェックする" 設計とする。
 */

export interface AccessTokenDenylist {
  /** jti を失効済みとして記録。expiresAt 経過後の振る舞いは実装依存 (Redis は EX で自動消去)。 */
  add(jti: string, expiresAt: Date): Promise<void>
  /** jti が失効済みなら true。未登録または失効しているが exp 経過は false 扱い。 */
  isRevoked(jti: string): Promise<boolean>
}
