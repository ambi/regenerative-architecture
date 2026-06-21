import './styles.css'

import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AuthenticationAPIError, loadPageData, tenantURL } from './api'
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
    const rawMessage =
      error instanceof AuthenticationAPIError ? error.message : '認証画面を読み込めませんでした。'
    const expiredLogin =
      rawMessage.includes('認可トランザクション') ||
      rawMessage.toLowerCase().includes('transaction')
    const title = expiredLogin ? 'ログイン要求が終了しています' : '認証を続行できません'
    const message = expiredLogin
      ? '前のログイン要求は完了または期限切れになっています。利用したい画面をもう一度開いてログインしてください。'
      : rawMessage || '認証画面を読み込めませんでした。もう一度お試しください。'
    createRoot(root!).render(
      <main className="flex min-h-screen items-center justify-center bg-slate-100 p-6">
        <div className="w-full max-w-md rounded-2xl border bg-white p-8 text-center shadow-lg">
          <h1 className="text-xl font-semibold text-slate-950">{title}</h1>
          <p className="mt-3 text-sm leading-6 text-slate-600">{message}</p>
          {expiredLogin ? (
            <div className="mt-6 grid gap-2">
              <a
                href={tenantURL('/account')}
                className="inline-flex h-10 items-center justify-center rounded-lg bg-slate-950 px-4 text-sm font-semibold text-white hover:bg-slate-800"
              >
                マイページを開く
              </a>
              <a
                href={tenantURL('/admin')}
                className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-800 hover:bg-slate-50"
              >
                管理コンソールを開く
              </a>
            </div>
          ) : null}
        </div>
      </main>,
    )
  }
}

void start()
