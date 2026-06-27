import { createBrowserHistory, createRouter } from '@tanstack/react-router'
import { AuthenticationAPIError, tenantBasePath, tenantURL } from './api/core'
import { routeTree } from './routeTree.gen'
import { markErrorPage } from './routes/-page'

export function preloadPageChunks() {
  // File-based routes with autoCodeSplitting let TanStack Router/Vite own route chunk loading.
}

function ErrorScreen({ error }: { error: unknown }) {
  markErrorPage()
  const rawMessage =
    error instanceof AuthenticationAPIError ? error.message : '認証画面を読み込めませんでした。'
  const expiredLogin =
    rawMessage.includes('認可トランザクション') || rawMessage.toLowerCase().includes('transaction')
  const title = expiredLogin ? 'ログイン要求が終了しています' : '認証を続行できません'
  const message = expiredLogin
    ? '前のログイン要求は完了または期限切れになっています。利用したい画面をもう一度開いてログインしてください。'
    : rawMessage || '認証画面を読み込めませんでした。もう一度お試しください。'
  return (
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
    </main>
  )
}

export function createAppRouter() {
  return createRouter({
    routeTree,
    history: createBrowserHistory(),
    basepath: tenantBasePath() || '/',
    defaultPreload: 'intent',
    // pendingComponent を設定しないことで、loader 解決中は前ページを表示したままにする。
    defaultErrorComponent: ({ error }) => <ErrorScreen error={error} />,
  })
}
