import { IconApps, IconTrash } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, revokeAccountConsent } from '../../api'
import { AccountShell } from '../../components/AccountShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import type { AccountConsent } from '../../types'

function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleDateString('ja-JP', { year: 'numeric', month: 'long', day: 'numeric' })
}

export function AccountApplicationsPage({
  csrfToken,
  username,
  consents: initial,
  isAdmin,
}: {
  csrfToken: string
  username: string
  consents: AccountConsent[]
  isAdmin: boolean
}) {
  const [consents, setConsents] = useState<AccountConsent[]>(initial)
  const [pending, setPending] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function handleRevoke(clientId: string) {
    setPending(clientId)
    setError('')
    setNotice('')
    try {
      await revokeAccountConsent(csrfToken, clientId)
      setConsents((current) => current.filter((consent) => consent.client_id !== clientId))
      setNotice(
        `${clientId} へのアクセスを取り消しました。次回このアプリを使うときは、改めて許可を求められます。`,
      )
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'アクセスを取り消せませんでした。',
      )
    } finally {
      setPending('')
    }
  }

  return (
    <AccountShell
      active="applications"
      username={username}
      isAdmin={isAdmin}
      title="接続済みアプリ"
      description="あなたのアカウントへのアクセスを許可したアプリケーションです。不要なものは取り消せます。"
    >
      {notice ? <Alert variant="success">{notice}</Alert> : null}
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      {consents.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 p-10 text-center">
          <IconApps size={28} className="text-slate-300" aria-hidden="true" />
          <p className="text-sm text-slate-500">アクセスを許可したアプリはありません。</p>
        </Card>
      ) : (
        <div className="grid gap-3">
          {consents.map((consent) => (
            <Card key={consent.client_id} className="flex flex-wrap items-start gap-4 p-5">
              <span className="flex size-11 shrink-0 items-center justify-center rounded-xl border border-blue-100 bg-blue-50 text-sm font-bold text-blue-700">
                {consent.client_id.slice(0, 2).toUpperCase()}
              </span>
              <div className="min-w-0 flex-1">
                <p className="font-mono text-sm font-semibold text-slate-900">
                  {consent.client_id}
                </p>
                <p className="mt-0.5 text-xs text-slate-500">
                  {formatDate(consent.granted_at)} に許可
                </p>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {consent.scopes.map((scope) => (
                    <span
                      key={scope}
                      className="rounded-md bg-slate-100 px-2 py-0.5 font-mono text-xs text-slate-600"
                    >
                      {scope}
                    </span>
                  ))}
                </div>
              </div>
              <Button
                type="button"
                variant="outline"
                className="text-red-700 hover:bg-red-50"
                disabled={pending === consent.client_id}
                onClick={() => handleRevoke(consent.client_id)}
              >
                <IconTrash size={16} aria-hidden="true" />
                {pending === consent.client_id ? '取り消し中…' : 'アクセスを取り消す'}
              </Button>
            </Card>
          ))}
        </div>
      )}
    </AccountShell>
  )
}
