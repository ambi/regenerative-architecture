import { useEffect, type ReactNode } from 'react'

// markPage は描画したページ種別を <meta name="ra-idp:page"> で DOM に表明する。
// SPA dispatcher の分岐を E2E から機械的に検証するための不変条件マーカー (wi-22)。
function markPage(kind: string) {
  let meta = document.head.querySelector<HTMLMetaElement>('meta[name="ra-idp:page"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.name = 'ra-idp:page'
    document.head.appendChild(meta)
  }
  meta.content = kind
}

export function PageMarker({ kind, children }: { kind: string; children: ReactNode }) {
  useEffect(() => {
    markPage(kind)
  }, [kind])
  return children
}

export function markErrorPage() {
  markPage('error')
}
