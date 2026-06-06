/**
 * Layer 3 — Application Logic
 *
 * 署名鍵のローテーション (ADR-009)。
 *
 * KeyStore ポートの rotate() を呼び、SigningKeyRotated 監査イベントを発行する。
 * 「鍵を回す」という操作は永続化方式 (memory / postgres / KMS) に依存しないため、
 * ここ (アプリケーション論理) に置く。実際の鍵生成・保管はアダプタが担う。
 *
 * 呼び出し元:
 *   - infra/scripts/rotate-signing-key.ts (運用 CLI / 定期ジョブ)
 *   - 緊急回転 (鍵漏洩時) の管理オペレーション
 */

import type { KeyStore } from '../ports/key-store'
import type { DomainEvent } from '../../spec-bindings/schemas'

export interface RotateSigningKeyResult {
  newKid: string
  previousKid?: string
}

export async function rotateSigningKeyUseCase(
  deps: { keyStore: KeyStore },
  emit: (e: DomainEvent) => void,
  now: Date = new Date(),
): Promise<RotateSigningKeyResult> {
  // 回転前の active 鍵 = previousKid。無い場合 (初回) は undefined。
  let previousKid: string | undefined
  try {
    previousKid = (await deps.keyStore.getActiveKey()).kid
  } catch {
    previousKid = undefined
  }

  const next = await deps.keyStore.rotate()

  emit({
    type: 'SigningKeyRotated',
    occurredAt: now.toISOString(),
    newKid: next.kid,
    ...(previousKid ? { previousKid } : {}),
  })

  return { newKid: next.kid, previousKid }
}
