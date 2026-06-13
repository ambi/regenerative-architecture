/**
 * Layer 4 — Adapter Layer (client IP 抽出)
 *
 * リクエストヘッダから「信頼できる」クライアント IP を解決する (ADR-029)。
 * `X-Forwarded-For` は trustedHops > 0 のときのみ採用する。trustedHops の値は
 * IdP の手前にある信頼境界（K8s Ingress / LB / WAF など）の段数を表す。
 *
 * X-Forwarded-For は左端ほどクライアントに近く、右端ほど自分に近いプロキシが
 * 付け足したエントリ。trustedHops=N とは「右端から N 個は自前で運用している
 * プロキシ」を意味するため、real client IP は ips[len - 1 - N] となる。
 *
 * trustedHops=0 / 値が無い場合は null を返し、per-IP throttle はその request では
 * 適用されない（安全側: 偽装可能な X-Forwarded-For を黙って信頼するより、
 * per-IP 防御が部分的に効かないほうが運用事故が軽い）。
 */

export interface ExtractClientIpOptions {
  trustedHops: number
}

export function extractClientIp(headers: Headers, options: ExtractClientIpOptions): string | null {
  const { trustedHops } = options
  if (trustedHops <= 0) return null
  const xff = headers.get('x-forwarded-for')
  if (!xff) return null
  const ips = xff
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
  const idx = ips.length - 1 - trustedHops
  if (idx < 0 || idx >= ips.length) return null
  return ips[idx]
}
