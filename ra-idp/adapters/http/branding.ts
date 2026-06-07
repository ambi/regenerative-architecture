/**
 * Layer 4 — Adapter Layer（Branding + Locale negotiation）
 *
 * 認証画面 (login / consent / device / error) に渡す UI 表示の差し替え点を
 * 1 ファイルに集約する。Phase 2 ではデフォルト brand のみ提供し、
 * Phase 5 でテナント解決 (request → tenant_id → brand) を差し込めるよう
 * `brandFor(tenantId?)` を slot 化しておく。
 *
 * 国際化は SPA 内に翻訳カタログを持ち、サーバは Accept-Language を見て
 * `<html lang>` と `<meta name="ra-idp:locale">` を返すだけのフラットな
 * 役割分担にする。サーバ側に翻訳テーブルは置かない。
 */

export type SupportedLocale = 'ja' | 'en'

export interface BrandingProfile {
  name: string
  /** ロゴ画像 URL。未設定なら shield アイコンの組み込みロゴが使われる。 */
  logoUrl: string | null
  /**
   * primary 色を上書き。HSL の "H S% L%" 形式 (CSS 変数値そのまま)。
   * shadcn/ui トークンの `--primary` を runtime 上書きするのに使う。
   */
  primaryHsl: string | null
  /** 既定ロケール。Accept-Language 解決前のフォールバック。 */
  defaultLocale: SupportedLocale
}

const DEFAULT_BRAND: BrandingProfile = {
  name: 'RA IdP',
  logoUrl: null,
  primaryHsl: null,
  defaultLocale: 'ja',
}

/**
 * テナントを引数に取りブランドを返す。Phase 2 では tenant 概念がまだ無いので
 * 常にデフォルトを返す。Phase 5 で resolver を差し替え可能にする。
 */
export function brandFor(_tenantId?: string | null): BrandingProfile {
  return DEFAULT_BRAND
}

/**
 * `Accept-Language` ヘッダから対応ロケールを選ぶ。q-value はサポート優先で
 * 簡易解決 (実運用では BCP 47 完全実装に差し替え可能)。
 */
export function negotiateLocale(
  acceptLanguage: string | undefined,
  fallback: SupportedLocale = 'ja',
): SupportedLocale {
  if (!acceptLanguage) return fallback
  const tokens = acceptLanguage.split(',').map((s) => s.trim().split(';')[0].toLowerCase())
  for (const tag of tokens) {
    if (tag.startsWith('ja')) return 'ja'
    if (tag.startsWith('en')) return 'en'
  }
  return fallback
}
