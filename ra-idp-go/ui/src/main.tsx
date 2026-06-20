import './styles.css'

import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AuthenticationAPIError, loadPageData } from './api'
import { createAppRouter } from './router'

const root = document.getElementById('root')
if (!root) {
  throw new Error('RA Identity root element is missing')
}

// markPage は描画したページ種別を <meta name="ra-idp:page"> で DOM に表明する。
// SPA dispatcher の分岐 (login / consent / device など) を E2E から機械的に
// 検証できるようにするための不変条件マーカー (wi-22)。
function markPage(kind: string) {
  let meta = document.head.querySelector<HTMLMetaElement>('meta[name="ra-idp:page"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.name = 'ra-idp:page'
    document.head.appendChild(meta)
  }
  meta.content = kind
}

async function start() {
  try {
    const pageData = await loadPageData()
    markPage(pageData.kind)
    const router = createAppRouter(pageData)
    createRoot(root!).render(
      <StrictMode>
        <RouterProvider router={router} />
      </StrictMode>,
    )
  } catch (error) {
    markPage('error')
    const message =
      error instanceof AuthenticationAPIError
        ? error.message
        : '認証画面を読み込めませんでした。もう一度お試しください。'
    createRoot(root!).render(
      <main className="flex min-h-screen items-center justify-center bg-slate-100 p-6">
        <div className="w-full max-w-md rounded-2xl border bg-white p-8 text-center shadow-lg">
          <h1 className="text-xl font-semibold text-slate-950">認証を続行できません</h1>
          <p className="mt-3 text-sm leading-6 text-slate-600">{message}</p>
        </div>
      </main>,
    )
  }
}

void start()
