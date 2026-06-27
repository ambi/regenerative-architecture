import { IconCheck, IconLayoutDashboard, IconRefresh, IconX } from '@tabler/icons-react'
import { AuthShell } from '../../components/AuthShell'
import { Button } from '../../components/ui/button'

export function CallbackPage({
  code,
  error,
  errorDescription,
}: {
  code?: string
  error?: string
  errorDescription?: string
}) {
  const succeeded = Boolean(code) && !error

  return (
    <AuthShell>
      <div className="flex flex-col items-center gap-7 py-4 text-center">
        <div
          className={`flex size-16 items-center justify-center rounded-2xl border ${
            succeeded
              ? 'border-emerald-100 bg-emerald-50 text-emerald-700'
              : 'border-red-100 bg-red-50 text-red-700'
          }`}
        >
          {succeeded ? (
            <IconCheck size={30} aria-hidden="true" />
          ) : (
            <IconX size={30} aria-hidden="true" />
          )}
        </div>

        <header className="flex max-w-md flex-col items-center gap-2.5">
          <p className="eyebrow">
            {succeeded ? 'Authentication complete' : 'Authentication failed'}
          </p>
          <h2 className="page-title">
            {succeeded ? 'ローカルデモ認証が完了しました' : '認証を完了できませんでした'}
          </h2>
          <p className="page-description">
            {succeeded
              ? '認可コードが発行され、ブラウザ認証フローが正常に完了しました。'
              : (errorDescription ?? error ?? '認可レスポンスが不正です。')}
          </p>
        </header>

        <div className="grid w-full gap-3">
          {succeeded && (
            <Button asChild className="w-full">
              <a href="/admin">
                <IconLayoutDashboard size={17} aria-hidden="true" />
                管理コンソールを開く
              </a>
            </Button>
          )}
          <Button asChild variant="outline" className="w-full">
            <a href="/">
              <IconRefresh size={17} aria-hidden="true" />
              もう一度試す
            </a>
          </Button>
        </div>
      </div>
    </AuthShell>
  )
}
