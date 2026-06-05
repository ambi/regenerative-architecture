/**
 * Layer 4 — Adapter Layer（認可アダプター）
 *
 * AuthZEN スタイルの認可アダプター（ローカル実装）。
 *
 * ユースケース層は本ファイルの authorize() を呼ぶだけ。
 * authorize() の中身を「ローカル評価 vs リモート AuthZEN サービス」に差し替えても
 * ユースケース層のコードは変わらない。
 *
 * 差し替えパターン:
 *   ローカル（現在）:
 *     → src/spec-bindings/policy/client-authorization.ts の evaluate() を直接呼ぶ
 *   リモート AuthZEN サービス（本番）:
 *     → POST https://authz.internal/access/v1/evaluation に差し替える
 *   OPA サーバー:
 *     → POST http://opa:8181/v1/data/oauth/allow に差し替える
 *   Cedar:
 *     → AVA (AWS Verified Permissions) などへ差し替える
 *
 * ADR-010 参照。
 */

import { evaluate } from '../../src/spec-bindings/policy/client-authorization'
import type {
  AuthZENRequest,
  AuthZENResponse,
} from '../../src/spec-bindings/policy/client-authorization'

export type { AuthZENRequest, AuthZENResponse }

export async function authorize(req: AuthZENRequest): Promise<AuthZENResponse> {
  return evaluate(req)
}
