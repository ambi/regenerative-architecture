import { IconAlertCircle, IconCheck, IconLoader2 } from '@tabler/icons-react'
import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { AdminLayout } from '@/components/layout/AdminLayout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { readAdminClientsContext } from '@/lib/page-context'

interface AdminClient {
  client_id: string
  client_name?: string
  client_type: 'public' | 'confidential'
  redirect_uris: string[]
  grant_types: string[]
  response_types: string[]
  token_endpoint_auth_method: string
  scope: string
}

export function AdminClientsPage() {
  const ctx = readAdminClientsContext()
  const [clients, setClients] = useState<AdminClient[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [editing, setEditing] = useState<AdminClient | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [issuedSecret, setIssuedSecret] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await fetch(`${ctx.basePath}/api/admin/clients`, {
        credentials: 'same-origin',
        cache: 'no-store',
      })
      if (!response.ok) throw new Error(await errorMessage(response))
      const body = (await response.json()) as { clients: AdminClient[] }
      setClients(body.clients ?? [])
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'クライアント一覧を取得できませんでした。')
    } finally {
      setLoading(false)
    }
  }, [ctx.basePath])

  useEffect(() => {
    void refresh()
  }, [refresh])

  async function createClient(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setSubmitting(true)
    setError(null)
    setNotice(null)
    setIssuedSecret(null)
    try {
      const response = await fetch(`${ctx.basePath}/api/admin/clients`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'content-type': 'application/json', 'X-CSRF-Token': ctx.csrf },
        body: JSON.stringify({
          client_name: optional(form.get('client_name')),
          client_type: String(form.get('client_type') ?? 'confidential'),
          redirect_uris: lines(form.get('redirect_uris')),
          grant_types: values(form.get('grant_types')),
          response_types: ['code'],
          token_endpoint_auth_method: String(
            form.get('token_endpoint_auth_method') ?? 'client_secret_basic',
          ),
          scope: String(form.get('scope') ?? '').trim(),
        }),
      })
      if (!response.ok) throw new Error(await errorMessage(response))
      const body = (await response.json()) as {
        client: AdminClient
        client_secret?: string
      }
      setIssuedSecret(body.client_secret ?? null)
      setNotice(`${body.client.client_id} を作成しました。`)
      event.currentTarget.reset()
      setShowCreate(false)
      await refresh()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'クライアントを作成できませんでした。')
    } finally {
      setSubmitting(false)
    }
  }

  async function updateClient(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!editing) return
    const form = new FormData(event.currentTarget)
    setSubmitting(true)
    setError(null)
    setNotice(null)
    try {
      const response = await fetch(
        `${ctx.basePath}/api/admin/clients/${encodeURIComponent(editing.client_id)}`,
        {
          method: 'PATCH',
          credentials: 'same-origin',
          headers: { 'content-type': 'application/json', 'X-CSRF-Token': ctx.csrf },
          body: JSON.stringify({
            client_name: optional(form.get('client_name')),
            redirect_uris: lines(form.get('redirect_uris')),
            grant_types: values(form.get('grant_types')),
            response_types: ['code'],
            scope: String(form.get('scope') ?? '').trim(),
          }),
        },
      )
      if (!response.ok) throw new Error(await errorMessage(response))
      setNotice(`${editing.client_id} を更新しました。`)
      setEditing(null)
      await refresh()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'クライアントを更新できませんでした。')
    } finally {
      setSubmitting(false)
    }
  }

  async function deleteClient(client: AdminClient) {
    if (!window.confirm(`${client.client_name || client.client_id} を削除しますか？`)) return
    setSubmitting(true)
    setError(null)
    try {
      const response = await fetch(
        `${ctx.basePath}/api/admin/clients/${encodeURIComponent(client.client_id)}`,
        {
          method: 'DELETE',
          credentials: 'same-origin',
          headers: { 'X-CSRF-Token': ctx.csrf },
        },
      )
      if (!response.ok) throw new Error(await errorMessage(response))
      setNotice(`${client.client_id} を削除しました。`)
      await refresh()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'クライアントを削除できませんでした。')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AdminLayout
      title="クライアント管理"
      description="OAuth/OIDC クライアントの接続先、grant、scope を管理します。"
      active="clients"
      basePath={ctx.basePath}
      actorUsername={ctx.actorUsername}
    >
      <Card>
        <CardContent className="space-y-5 pt-6">
          {error ? (
            <Alert variant="destructive">
              <IconAlertCircle className="h-4 w-4" />
              <AlertTitle>失敗しました</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          {notice ? (
            <Alert>
              <IconCheck className="h-4 w-4" />
              <AlertTitle>完了しました</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          ) : null}
          {issuedSecret ? (
            <Alert>
              <IconCheck className="h-4 w-4" />
              <AlertTitle>client_secret は今回だけ表示されます</AlertTitle>
              <AlertDescription>
                <code className="break-all font-mono">{issuedSecret}</code>
              </AlertDescription>
            </Alert>
          ) : null}
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm text-muted-foreground">
              {loading ? '読み込み中…' : `${clients.length} 件のクライアント`}
            </p>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => void refresh()} disabled={loading}>
                更新
              </Button>
              <Button onClick={() => setShowCreate((value) => !value)}>
                {showCreate ? '作成を閉じる' : '新規クライアント'}
              </Button>
            </div>
          </div>
          {showCreate ? <ClientForm submitting={submitting} onSubmit={createClient} /> : null}
          {editing ? (
            <ClientForm
              key={editing.client_id}
              client={editing}
              submitting={submitting}
              onSubmit={updateClient}
              onCancel={() => setEditing(null)}
            />
          ) : null}
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="text-xs uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="py-2 pr-4">クライアント</th>
                  <th className="py-2 pr-4">種別</th>
                  <th className="py-2 pr-4">Grant</th>
                  <th className="py-2 pr-4">Scope</th>
                  <th className="py-2 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/60">
                {clients.map((client) => (
                  <tr key={client.client_id}>
                    <td className="py-3 pr-4">
                      <div className="font-medium">{client.client_name || client.client_id}</div>
                      <div className="font-mono text-xs text-muted-foreground">
                        {client.client_id}
                      </div>
                    </td>
                    <td className="py-3 pr-4">{client.client_type}</td>
                    <td className="py-3 pr-4 text-xs">{client.grant_types.join(', ')}</td>
                    <td className="py-3 pr-4 text-xs">{client.scope || '—'}</td>
                    <td className="py-3 text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          disabled={submitting}
                          onClick={() => {
                            setShowCreate(false)
                            setEditing(client)
                          }}
                        >
                          編集
                        </Button>
                        <Button
                          variant="outline"
                          disabled={submitting}
                          onClick={() => void deleteClient(client)}
                        >
                          削除
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  )
}

function ClientForm({
  client,
  submitting,
  onSubmit,
  onCancel,
}: {
  client?: AdminClient
  submitting: boolean
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onCancel?: () => void
}) {
  const editing = client !== undefined
  return (
    <form className="space-y-4 rounded-md border bg-muted/30 p-4" onSubmit={onSubmit}>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="表示名" name="client_name" value={client?.client_name} />
        {editing ? null : (
          <SelectField label="種別" name="client_type" values={['confidential', 'public']} />
        )}
        <Field
          label="Redirect URI (1行1件)"
          name="redirect_uris"
          value={client?.redirect_uris.join('\n')}
          textarea
        />
        <Field
          label="Grant (カンマ区切り)"
          name="grant_types"
          value={client?.grant_types.join(', ') ?? 'authorization_code, refresh_token'}
        />
        {editing ? null : (
          <SelectField
            label="認証方式"
            name="token_endpoint_auth_method"
            values={['client_secret_basic', 'client_secret_post', 'private_key_jwt', 'none']}
          />
        )}
        <Field label="Scope" name="scope" value={client?.scope ?? 'openid profile email'} />
      </div>
      <div className="flex gap-2">
        <Button type="submit" disabled={submitting}>
          {submitting ? <IconLoader2 className="h-4 w-4 animate-spin" /> : null}
          {editing ? '更新' : '作成'}
        </Button>
        {onCancel ? (
          <Button type="button" variant="outline" disabled={submitting} onClick={onCancel}>
            キャンセル
          </Button>
        ) : null}
      </div>
    </form>
  )
}

function Field({
  label,
  name,
  value,
  textarea = false,
}: {
  label: string
  name: string
  value?: string
  textarea?: boolean
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={name}>{label}</Label>
      {textarea ? (
        <textarea
          id={name}
          name={name}
          required
          defaultValue={value}
          className="min-h-20 w-full rounded-md border bg-background px-3 py-2 text-sm"
        />
      ) : (
        <Input id={name} name={name} defaultValue={value} />
      )}
    </div>
  )
}

function SelectField({ label, name, values }: { label: string; name: string; values: string[] }) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={name}>{label}</Label>
      <select
        id={name}
        name={name}
        className="h-10 w-full rounded-md border bg-background px-3 text-sm"
      >
        {values.map((value) => (
          <option key={value} value={value}>
            {value}
          </option>
        ))}
      </select>
    </div>
  )
}

function optional(value: FormDataEntryValue | null): string | undefined {
  const text = String(value ?? '').trim()
  return text || undefined
}

function lines(value: FormDataEntryValue | null): string[] {
  return String(value ?? '')
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function values(value: FormDataEntryValue | null): string[] {
  return String(value ?? '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

async function errorMessage(response: Response): Promise<string> {
  const body = (await response.json().catch(() => null)) as { message?: string } | null
  return body?.message ?? `管理 API が ${response.status} を返しました。`
}
