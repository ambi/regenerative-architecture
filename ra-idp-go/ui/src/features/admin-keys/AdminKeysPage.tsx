import { IconKey, IconRefresh, IconRotateClockwise } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, listAdminKeys, rotateAdminKey } from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import type { AdminKey } from '../../types'

const DEFAULT_TENANT_ID = 'default'

export function AdminKeysPage({
  csrfToken,
  actorUsername,
  actorRoles,
  actorTenantID,
  keys: initial,
}: {
  csrfToken: string
  actorUsername?: string
  actorRoles: string[]
  actorTenantID: string
  keys: AdminKey[]
}) {
  const [keys, setKeys] = useState(initial)
  const [selected, setSelected] = useState<AdminKey | null>(
    initial.find((k) => k.active) ?? initial[0] ?? null,
  )
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [confirm, setConfirm] = useState(false)

  const canRotate =
    actorRoles.includes('system_admin') && actorTenantID === DEFAULT_TENANT_ID

  async function refresh(preferred?: string) {
    const next = await listAdminKeys()
    setKeys(next)
    const match = next.find((k) => k.kid === preferred) ?? next.find((k) => k.active) ?? next[0]
    setSelected(match ?? null)
  }

  async function run(action: () => Promise<void>, success: string) {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(success)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '署名鍵の操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  async function handleRotate() {
    await run(async () => {
      const result = await rotateAdminKey(csrfToken)
      await refresh(result.next.kid)
    }, '新しい署名鍵に切り替えました。旧鍵は JWKS の verifying に残ります。')
    setConfirm(false)
  }

  return (
    <AdminShell
      active="keys"
      actorUsername={actorUsername}
      title="署名鍵 (Signing Keys)"
      description="ID Token / Access Token の署名に使う JWKS の鍵集合。ローテートは system_admin かつ default tenant 経路のみ。"
      actions={
        <>
          <Button
            variant="outline"
            className="size-9 px-0"
            aria-label="一覧を再読み込み"
            onClick={() => run(() => refresh(selected?.kid), '一覧を更新しました。')}
            disabled={busy}
          >
            <IconRefresh size={16} aria-hidden="true" />
          </Button>
          {canRotate ? (
            <Button onClick={() => setConfirm(true)} disabled={busy}>
              <IconRotateClockwise size={16} aria-hidden="true" />
              ローテート
            </Button>
          ) : null}
        </>
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_420px]">
        <Card className="overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">Kid</th>
                <th className="px-4 py-3">Alg</th>
                <th className="px-4 py-3">状態</th>
                <th className="px-4 py-3">生成</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((k) => (
                <tr
                  key={k.kid}
                  onClick={() => setSelected(k)}
                  className={`cursor-pointer border-t border-slate-100 hover:bg-slate-50 ${
                    selected?.kid === k.kid ? 'bg-blue-50/60' : ''
                  }`}
                >
                  <td className="px-4 py-3 font-mono text-xs">{k.kid}</td>
                  <td className="px-4 py-3">{k.alg}</td>
                  <td className="px-4 py-3">
                    {k.active ? (
                      <span className="rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700">
                        active
                      </span>
                    ) : (
                      <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">
                        verifying
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-500">{formatDate(k.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="p-5">
          <div className="flex items-center gap-2">
            <IconKey size={16} aria-hidden="true" className="text-slate-500" />
            <h2 className="text-sm font-semibold text-slate-700">公開鍵 JWK</h2>
          </div>
          {selected ? (
            <>
              <dl className="mt-4 grid grid-cols-[80px_minmax(0,1fr)] gap-y-2 text-xs">
                <dt className="text-slate-500">Kid</dt>
                <dd className="break-all font-mono">{selected.kid}</dd>
                <dt className="text-slate-500">Alg</dt>
                <dd>{selected.alg}</dd>
                <dt className="text-slate-500">状態</dt>
                <dd>{selected.active ? 'yes' : 'no'}</dd>
                <dt className="text-slate-500">生成</dt>
                <dd>{formatDate(selected.created_at)}</dd>
              </dl>
              <pre className="mt-4 max-h-[360px] overflow-auto rounded-md bg-slate-950 p-3 text-xs text-slate-50">
                {JSON.stringify(selected.public_jwk, null, 2)}
              </pre>
            </>
          ) : (
            <p className="mt-4 text-sm text-slate-500">署名鍵がありません。</p>
          )}
        </Card>
      </div>

      {confirm ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40 px-4">
          <Card className="w-full max-w-md p-6">
            <h2 className="text-base font-semibold text-slate-900">署名鍵をローテートします</h2>
            <p className="mt-3 text-sm text-slate-600">
              新しい active 鍵が生成され、旧 active 鍵は JWKS に verifying として残ります。
              JWKS キャッシュが更新されるまで一時的な検証遅延が起きる可能性があります。
            </p>
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="outline" onClick={() => setConfirm(false)} disabled={busy}>
                キャンセル
              </Button>
              <Button onClick={handleRotate} disabled={busy}>
                ローテート実行
              </Button>
            </div>
          </Card>
        </div>
      ) : null}
    </AdminShell>
  )
}

function formatDate(value: string): string {
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}
