// step-up 再認証 (ADR-043 / wi-43)。高 sensitivity な self-service 操作が 403
// step_up_required を返したとき、再認証 modal を出して password / TOTP を提示させ、
// 成立後に元の操作を 1 回だけ再試行する共通フック + ダイアログ。
import { type FormEvent, useState } from 'react'
import {
  AuthenticationAPIError,
  type StepUpMethod,
  completeStepUp,
  isStepUpRequired,
  startStepUp,
} from '../api'
import { Alert } from './ui/alert'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'

// ユーザーが再認証 modal を閉じた (キャンセルした) ことを表す。呼び出し側はこれを
// 「エラー表示なしの中断」として握りつぶす。
export class StepUpCancelledError extends Error {
  constructor() {
    super('step-up cancelled')
    this.name = 'StepUpCancelledError'
  }
}

interface PendingStepUp {
  methods: StepUpMethod[]
  resolve: () => void
  reject: (cause: unknown) => void
}

// useStepUpGuard は guard() でラップした操作が step_up_required を返したら再認証 modal を
// 開き、成立後に同じ操作を再試行してその結果を呼び出し元へ返す。dialog を JSX に描画する。
export function useStepUpGuard(csrfToken: string) {
  const [pending, setPending] = useState<PendingStepUp | null>(null)

  async function guard<T>(action: () => Promise<T>): Promise<T> {
    try {
      return await action()
    } catch (cause) {
      if (!isStepUpRequired(cause)) throw cause
      const methods = await startStepUp(csrfToken).catch(() => ['password'] as StepUpMethod[])
      await new Promise<void>((resolve, reject) => {
        setPending({ methods, resolve, reject })
      })
      return action()
    }
  }

  const dialog = pending ? (
    <StepUpDialog
      methods={pending.methods}
      csrfToken={csrfToken}
      onAuthenticated={() => {
        const current = pending
        setPending(null)
        current.resolve()
      }}
      onCancel={() => {
        const current = pending
        setPending(null)
        current.reject(new StepUpCancelledError())
      }}
    />
  ) : null

  return { guard, dialog }
}

const methodLabels: Record<StepUpMethod, string> = {
  password: 'パスワード',
  totp: '認証アプリ',
}

interface StepUpDialogProps {
  methods: StepUpMethod[]
  csrfToken: string
  onAuthenticated: () => void
  onCancel: () => void
}

function StepUpDialog({ methods, csrfToken, onAuthenticated, onCancel }: StepUpDialogProps) {
  const available = methods.length > 0 ? methods : (['password'] as StepUpMethod[])
  const [method, setMethod] = useState<StepUpMethod>(available[0])
  const [credential, setCredential] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      await completeStepUp(csrfToken, method, credential.trim())
      onAuthenticated()
    } catch (cause) {
      const message =
        cause instanceof AuthenticationAPIError ? cause.message : '再認証に失敗しました。'
      setError(message)
      setBusy(false)
    }
  }

  const isTotp = method === 'totp'

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="step-up-title"
      onKeyDown={(event) => {
        if (event.key === 'Escape') onCancel()
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) onCancel()
      }}
    >
      <div className="w-full max-w-sm rounded-xl bg-white p-6 shadow-xl">
        <h2 id="step-up-title" className="text-base font-semibold text-slate-900">
          本人確認のため再認証してください
        </h2>
        <p className="mt-1 text-sm text-slate-600">
          この操作はアカウントの安全に関わるため、もう一度本人確認を行います。
        </p>

        {error ? (
          <Alert variant="destructive" className="mt-4">
            {error}
          </Alert>
        ) : null}

        <form onSubmit={handleSubmit} className="mt-4 grid gap-4">
          {available.length > 1 ? (
            <div className="flex gap-2" role="tablist" aria-label="再認証の方法">
              {available.map((option) => (
                <Button
                  key={option}
                  type="button"
                  variant={option === method ? 'default' : 'outline'}
                  className="h-9 px-3 text-xs"
                  aria-pressed={option === method}
                  onClick={() => {
                    setMethod(option)
                    setCredential('')
                    setError('')
                  }}
                >
                  {methodLabels[option]}
                </Button>
              ))}
            </div>
          ) : null}

          <div className="grid gap-1.5">
            <Label htmlFor="step-up-credential">
              {isTotp ? '認証アプリの 6 桁コード' : '現在のパスワード'}
            </Label>
            <Input
              id="step-up-credential"
              autoFocus
              type={isTotp ? 'text' : 'password'}
              inputMode={isTotp ? 'numeric' : undefined}
              autoComplete={isTotp ? 'one-time-code' : 'current-password'}
              pattern={isTotp ? '[0-9]{6}' : undefined}
              maxLength={isTotp ? 6 : undefined}
              required
              placeholder={isTotp ? '123456' : '現在のパスワードを入力'}
              value={credential}
              className={isTotp ? 'font-mono tracking-[0.3em]' : undefined}
              onChange={(event) =>
                setCredential(isTotp ? event.target.value.replace(/\D/g, '') : event.target.value)
              }
            />
          </div>

          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" disabled={busy} onClick={onCancel}>
              キャンセル
            </Button>
            <Button
              type="submit"
              disabled={
                busy || credential.trim() === '' || (isTotp && credential.trim().length !== 6)
              }
            >
              {busy ? '確認中…' : '再認証して続行'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
