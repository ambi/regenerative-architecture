import {
  IconDeviceDesktopCheck,
  IconInfoCircle,
  IconKeyboard,
  IconShieldCheck,
  IconX,
} from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, continueBrowserFlow, submitDevice } from '../../api'
import { AuthShell } from '../../components/AuthShell'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'

export function DevicePage({ csrfToken, userCode }: { csrfToken: string; userCode: string }) {
  const normalizedCode = userCode.replace(/-/g, '').toUpperCase()
  const [code, setCode] = useState(normalizedCode)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const isComplete = code.length === 8

  async function handleDevice(action: 'allow' | 'deny') {
    setSubmitting(true)
    setError('')
    try {
      continueBrowserFlow(await submitDevice(csrfToken, code, action))
    } catch (cause) {
      if (cause instanceof AuthenticationAPIError && cause.code === 'authentication_required') {
        window.location.assign('/status?state=authentication-required')
        return
      }
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'デバイス要求を処理できませんでした。',
      )
      setSubmitting(false)
    }
  }

  return (
    <AuthShell
      asideTitle="新しいデバイスを、安全な確認手順で接続。"
      asideText="表示されたコードと接続先を確認し、自分が開始した操作だけを承認してください。"
    >
      <div className="flex flex-col gap-7">
        <header className="flex flex-col gap-2.5">
          <div className="mb-1 flex size-12 items-center justify-center rounded-xl border border-blue-100 bg-blue-50 text-blue-700">
            <IconDeviceDesktopCheck size={25} aria-hidden="true" />
          </div>
          <p className="eyebrow">デバイス認可</p>
          <h2 className="page-title">デバイスを接続</h2>
          <p className="page-description">
            接続するデバイスに表示されている8文字のコードを入力してください。
          </p>
        </header>

        <form onSubmit={(event) => event.preventDefault()}>
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="user-code">デバイスコード</Label>
                <span className="text-xs tabular-nums text-slate-500">{code.length} / 8</span>
              </div>
              <div className="relative">
                <IconKeyboard
                  className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-slate-400"
                  size={19}
                  aria-hidden="true"
                />
                <Input
                  id="user-code"
                  value={code}
                  onChange={(event) =>
                    setCode(
                      event.currentTarget.value
                        .replace(/[^a-z0-9]/gi, '')
                        .slice(0, 8)
                        .toUpperCase(),
                    )
                  }
                  inputMode="text"
                  autoComplete="one-time-code"
                  spellCheck={false}
                  aria-describedby="user-code-hint"
                  className="h-16 px-12 text-center font-mono text-xl font-bold tracking-[0.32em] uppercase sm:text-2xl"
                  disabled={submitting}
                />
              </div>
              <p id="user-code-hint" className="text-xs leading-5 text-slate-500">
                ハイフンは入力不要です。例:{' '}
                <span className="font-mono font-semibold">ABCD-EFGH</span>
              </p>
            </div>

            <div className="flex gap-3 rounded-xl border border-blue-100 bg-blue-50/60 p-3.5 text-xs leading-5 text-blue-950">
              <IconInfoCircle
                className="mt-0.5 shrink-0 text-blue-700"
                size={17}
                aria-hidden="true"
              />
              <p>コードが一致していても、自分で開始していない接続要求は承認しないでください。</p>
            </div>

            {error ? (
              <p role="alert" className="text-sm font-medium text-red-700">
                {error}
              </p>
            ) : null}

            <div className="flex flex-col gap-2.5">
              <Button
                type="button"
                size="lg"
                disabled={!isComplete || submitting}
                onClick={() => handleDevice('allow')}
              >
                <IconShieldCheck size={18} aria-hidden="true" />
                {submitting ? '処理しています…' : 'このデバイスを承認'}
              </Button>
              <Button
                type="button"
                size="lg"
                variant="ghost"
                disabled={!isComplete || submitting}
                onClick={() => handleDevice('deny')}
              >
                <IconX size={17} aria-hidden="true" />
                接続を拒否
              </Button>
            </div>
          </div>
        </form>
      </div>
    </AuthShell>
  )
}
