import { IconCircleCheck, IconMail } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, confirmEmailChange, tenantURL } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'

export function EmailVerifyPage({ csrfToken, token }: { csrfToken: string; token: string }) {
  const [state, setState] = useState<'idle' | 'submitting' | 'done'>('idle')
  const [error, setError] = useState('')

  async function handleConfirm() {
    setState('submitting')
    setError('')
    try {
      await confirmEmailChange(csrfToken, token)
      setState('done')
    } catch (cause) {
      setError(cause instanceof AuthenticationAPIError ? cause.message : '確認に失敗しました。')
      setState('idle')
    }
  }

  return (
    <AuthShell aside={false}>
      <div className="grid gap-6">
        <header className="grid gap-2">
          <span className="flex size-11 items-center justify-center rounded-xl bg-blue-50 text-blue-700">
            <IconMail size={22} aria-hidden="true" />
          </span>
          <h1 className="text-xl font-semibold text-slate-900">メールアドレスの確認</h1>
          <p className="text-sm text-slate-600">
            このアドレスをアカウントのメールアドレスとして確定します。
          </p>
        </header>

        {error ? <Alert variant="destructive">{error}</Alert> : null}

        {state === 'done' ? (
          <Alert variant="success" className="flex items-start gap-2">
            <IconCircleCheck className="mt-0.5 shrink-0" size={18} aria-hidden="true" />
            <span>
              メールアドレスを確認しました。{' '}
              <a href={tenantURL('/account')} className="font-medium underline">
                アカウントへ戻る
              </a>
            </span>
          </Alert>
        ) : token ? (
          <Button type="button" onClick={handleConfirm} disabled={state === 'submitting'}>
            {state === 'submitting' ? '確認中…' : 'メールアドレスを確認する'}
          </Button>
        ) : (
          <Alert variant="destructive">
            確認リンクが正しくありません。メール内のリンクをもう一度開いてください。
          </Alert>
        )}
      </div>
    </AuthShell>
  )
}
