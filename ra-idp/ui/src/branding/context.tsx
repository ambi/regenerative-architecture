import { createContext, useContext, useMemo, type ReactNode } from 'react'
import { readMeta } from '@/lib/page-context'

export interface Branding {
  name: string
  /** ロゴ画像 URL。null なら AuthLayout 組み込みの shield アイコンを使う。 */
  logoUrl: string | null
}

const BrandingContext = createContext<Branding | null>(null)

/**
 * Phase 2: meta タグから brand を読む単純な provider。
 * Phase 5 でテナント分離が入る際は、サーバ側 `brandFor(tenantId)` の結果が
 * meta に流れてくるだけなので、本ファイルの実装は据え置きで済む想定。
 */
export function BrandingProvider({ children }: { children: ReactNode }) {
  const value = useMemo<Branding>(
    () => ({
      name: readMeta('ra-idp:brand-name') ?? 'RA IdP',
      logoUrl: readMeta('ra-idp:brand-logo'),
    }),
    [],
  )
  return <BrandingContext.Provider value={value}>{children}</BrandingContext.Provider>
}

export function useBranding(): Branding {
  const ctx = useContext(BrandingContext)
  if (!ctx) throw new Error('useBranding must be used within BrandingProvider')
  return ctx
}
