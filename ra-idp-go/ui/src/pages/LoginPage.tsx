import { IconAlertCircle, IconArrowRight, IconAt, IconLock } from '@tabler/icons-react'
import { AuthShell } from '../components/AuthShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import type { LoginPage as LoginPageData } from '../types'

export function LoginPage({ requestId, error }: LoginPageData) {
  return (
    <AuthShell>
      <div className="flex flex-col gap-8">
        <div className="flex flex-col gap-2">
          <p className="eyebrow">Welcome back</p>
          <h2 className="text-2xl font-bold tracking-tight">アカウントにログイン</h2>
          <p className="text-slate-500">続行するには認証情報を入力してください。</p>
        </div>

        {error ? (
          <Alert className="flex gap-3">
            <IconAlertCircle className="mt-0.5 shrink-0" size={18} />
            <div>
              <p className="font-semibold">ログインできません</p>
              <p className="mt-1 text-sm">{error}</p>
            </div>
          </Alert>
        ) : null}

        <form method="POST" action="/login">
          <input type="hidden" name="request_id" value={requestId} />
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="username">ユーザー名</Label>
              <div className="relative">
                <IconAt
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="username"
                  name="username"
                  placeholder="your.name"
                  className="pl-10"
                  autoComplete="username"
                  required
                  autoFocus
                />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">パスワード</Label>
              <div className="relative">
                <IconLock
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
                  size={18}
                />
                <Input
                  id="password"
                  type="password"
                  name="password"
                  placeholder="パスワードを入力"
                  className="pl-10"
                  autoComplete="current-password"
                  required
                />
              </div>
            </div>
            <Button type="submit" size="lg" className="mt-1 w-full">
              ログイン
              <IconArrowRight size={18} />
            </Button>
          </div>
        </form>
      </div>
    </AuthShell>
  )
}
