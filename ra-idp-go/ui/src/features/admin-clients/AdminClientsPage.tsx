import {
  IconAlertTriangle,
  IconArrowLeft,
  IconCheck,
  IconCopy,
  IconEdit,
  IconKey,
  IconPlus,
  IconRefresh,
  IconSearch,
  IconTrash,
  IconX,
} from '@tabler/icons-react'
import { type FormEvent, useMemo, useState } from 'react'
import {
  AuthenticationAPIError,
  createAdminClient,
  deleteAdminClient,
  getAdminClient,
  listAdminClients,
  tenantURL,
  updateAdminClient,
} from '../../api'
import { AdminPaneActions } from '../../components/AdminPaneActions'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { DropdownMenuItem } from '../../components/ui/dropdown-menu'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import { cn } from '../../lib/utils'
import type {
  AdminClient,
  AdminClientDetailPage as AdminClientDetailPageData,
  AdminClientsPage as AdminClientsPageData,
} from '../../types'

type ClientForm = {
  client_name: string
  client_type: 'public' | 'confidential'
  redirect_uris: string
  grant_types: string
  response_types: string
  token_endpoint_auth_method: AdminClient['token_endpoint_auth_method']
  scope: string
  jwks_uri: string
  tls_client_auth_subject_dn: string
  require_pushed_authorization_requests: boolean
  dpop_bound_access_tokens: boolean
}

const emptyForm: ClientForm = {
  client_name: '',
  client_type: 'confidential',
  redirect_uris: '',
  grant_types: 'authorization_code, refresh_token',
  response_types: 'code',
  token_endpoint_auth_method: 'client_secret_basic',
  scope: 'openid profile email',
  jwks_uri: '',
  tls_client_auth_subject_dn: '',
  require_pushed_authorization_requests: false,
  dpop_bound_access_tokens: false,
}

export function AdminClientsPage({
  csrfToken,
  actorUsername,
  clients: initialClients,
}: AdminClientsPageData) {
  const [clients, setClients] = useState(initialClients)
  const [selectedID, setSelectedID] = useState(initialClients[0]?.client_id ?? '')
  const [query, setQuery] = useState('')
  const [form, setForm] = useState<ClientForm>(emptyForm)
  const [dialog, setDialog] = useState<'create' | 'edit' | 'delete' | null>(null)
  const [issuedSecret, setIssuedSecret] = useState<{ clientID: string; secret: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const selected = clients.find((client) => client.client_id === selectedID)
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return clients.filter((client) =>
      !needle
        ? true
        : [
            client.client_id,
            client.client_name,
            client.client_type,
            client.token_endpoint_auth_method,
            client.scope,
            ...client.redirect_uris,
          ].some((value) => value?.toLowerCase().includes(needle)),
    )
  }, [clients, query])

  async function refresh(preferredID = selectedID) {
    const next = await listAdminClients()
    setClients(next)
    setSelectedID(next.find((client) => client.client_id === preferredID)?.client_id ?? next[0]?.client_id ?? '')
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
          : 'アプリケーション管理操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  function openCreate() {
    setForm(emptyForm)
    setDialog('create')
  }

  function openEdit(client: AdminClient) {
    setForm({
      client_name: client.client_name ?? '',
      client_type: client.client_type,
      redirect_uris: client.redirect_uris.join('\n'),
      grant_types: client.grant_types.join(', '),
      response_types: client.response_types.join(', '),
      token_endpoint_auth_method: client.token_endpoint_auth_method,
      scope: client.scope,
      jwks_uri: client.jwks_uri ?? '',
      tls_client_auth_subject_dn: client.tls_client_auth_subject_dn ?? '',
      require_pushed_authorization_requests: client.require_pushed_authorization_requests,
      dpop_bound_access_tokens: client.dpop_bound_access_tokens,
    })
    setDialog('edit')
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (dialog === 'create') {
      await run(async () => {
        const result = await createAdminClient(csrfToken, {
          client_name: optional(form.client_name),
          client_type: form.client_type,
          redirect_uris: lines(form.redirect_uris),
          grant_types: values(form.grant_types),
          response_types: values(form.response_types),
          token_endpoint_auth_method: form.token_endpoint_auth_method,
          scope: form.scope.trim(),
          jwks_uri: optional(form.jwks_uri),
          tls_client_auth_subject_dn: optional(form.tls_client_auth_subject_dn),
          require_pushed_authorization_requests: form.require_pushed_authorization_requests,
          dpop_bound_access_tokens: form.dpop_bound_access_tokens,
        })
        setDialog(null)
        if (result.client_secret) {
          setIssuedSecret({ clientID: result.client.client_id, secret: result.client_secret })
        }
        await refresh(result.client.client_id)
      }, 'アプリケーションを作成しました。')
      return
    }
    if (!selected) return
    await run(async () => {
      await updateAdminClient(csrfToken, selected.client_id, {
        client_name: optional(form.client_name),
        redirect_uris: lines(form.redirect_uris),
        grant_types: values(form.grant_types),
        response_types: values(form.response_types),
        scope: form.scope.trim(),
        require_pushed_authorization_requests: form.require_pushed_authorization_requests,
        dpop_bound_access_tokens: form.dpop_bound_access_tokens,
      })
      setDialog(null)
      await refresh(selected.client_id)
    }, 'アプリケーションを更新しました。')
  }

  async function handleDelete() {
    if (!selected) return
    await run(async () => {
      await deleteAdminClient(csrfToken, selected.client_id)
      setDialog(null)
      await refresh('')
    }, 'アプリケーションを削除しました。')
  }

  return (
    <>
      <AdminShell
        active="clients"
        actorUsername={actorUsername}
        title="アプリケーション"
        description="OAuth client の接続先、認証方式、許可する grant と scope を管理します。"
        actions={
          <Button onClick={openCreate}>
            <IconPlus size={17} aria-hidden="true" />
            アプリケーションを追加
          </Button>
        }
      >
            <section className="grid gap-3 sm:grid-cols-3" aria-label="アプリケーション概要">
              <Metric label="総アプリケーション" value={clients.length} />
              <Metric label="機密タイプ" value={clients.filter((client) => client.client_type === 'confidential').length} />
              <Metric label="PAR 必須" value={clients.filter((client) => client.require_pushed_authorization_requests).length} />
            </section>

            {error && <Alert>{error}</Alert>}
            {notice && (
              <div role="status" className="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
                <IconCheck size={18} aria-hidden="true" />
                {notice}
              </div>
            )}

            <Card className="overflow-hidden shadow-[0_1px_2px_rgb(15_23_42/4%)]">
              <div className="flex flex-col gap-3 border-b border-slate-200 p-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="relative w-full max-w-xl">
                  <IconSearch size={18} className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                  <Input value={query} onChange={(event) => setQuery(event.target.value)} className="h-10 pl-10" placeholder="名前、client ID、redirect URI、scope で検索" aria-label="アプリケーションを検索" />
                </div>
                <Button
                  variant="outline"
                  className="size-9 px-0"
                  disabled={busy}
                  aria-label="一覧を再読み込み"
                  onClick={() => void run(() => refresh(), '一覧を更新しました。')}
                >
                  <IconRefresh size={16} aria-hidden="true" />
                </Button>
              </div>
              <div className="grid min-h-[520px] xl:grid-cols-[minmax(0,1.55fr)_420px]">
                <div className="min-w-0 overflow-x-auto">
                  <table className="w-full min-w-[760px] text-left text-sm">
                    <thead className="border-b border-slate-200 bg-slate-50/80 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-slate-500">
                      <tr>
                        <th className="px-5 py-3.5">アプリケーション</th>
                        <th className="px-5 py-3.5">種別</th>
                        <th className="px-5 py-3.5">認証方式</th>
                        <th className="px-5 py-3.5">グラント</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {filtered.map((client) => (
                        <tr key={client.client_id} onClick={() => setSelectedID(client.client_id)} className={cn('cursor-pointer bg-white hover:bg-slate-50', selectedID === client.client_id && 'bg-blue-50/60 hover:bg-blue-50/80')}>
                          <td className="px-5 py-4">
                            <p className="font-semibold text-slate-900">{client.client_name || client.client_id}</p>
                            <p className="mt-1 font-mono text-xs text-slate-500">{client.client_id}</p>
                          </td>
                          <td className="px-5 py-4"><Badge>{client.client_type}</Badge></td>
                          <td className="px-5 py-4 text-xs text-slate-600">{client.token_endpoint_auth_method}</td>
                          <td className="px-5 py-4 text-xs text-slate-600">{client.grant_types.join(', ')}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {filtered.length === 0 && (
                    <div className="flex min-h-64 flex-col items-center justify-center px-6 text-center">
                      <IconKey size={28} className="text-slate-300" aria-hidden="true" />
                      <p className="mt-3 font-semibold text-slate-800">アプリケーションが見つかりません</p>
                      <p className="mt-1 text-sm text-slate-500">検索語を変更するか、新しいアプリケーションを追加してください。</p>
                    </div>
                  )}
                </div>
                <aside className="border-t border-slate-200 bg-slate-50/40 xl:border-l xl:border-t-0">
                  {selected ? (
                    <ClientPaneView client={selected} busy={busy} onEdit={() => openEdit(selected)} onDelete={() => setDialog('delete')} />
                  ) : (
                    <div className="flex h-full min-h-80 items-center justify-center p-8 text-center text-sm text-slate-500">アプリケーションを選択すると詳細が表示されます。</div>
                  )}
                </aside>
              </div>
              <div className="border-t border-slate-200 bg-slate-50/70 px-5 py-3 text-xs text-slate-500">{filtered.length} 件を表示</div>
            </Card>
      </AdminShell>

      {(dialog === 'create' || dialog === 'edit') && (
        <ClientFormDialog mode={dialog} form={form} busy={busy} onChange={setForm} onClose={() => setDialog(null)} onSubmit={handleSubmit} />
      )}
      {dialog === 'delete' && selected && (
        <DeleteDialog client={selected} busy={busy} onClose={() => setDialog(null)} onConfirm={() => void handleDelete()} />
      )}
      {issuedSecret && <SecretDialog value={issuedSecret} onClose={() => setIssuedSecret(null)} />}
    </>
  )
}

// ClientPaneView は一覧右の詳細ビュー。同一画面で見比べられるよう主要メタデータを
// 残しつつ、上部に「詳細 / 編集」を置いて専用詳細ページ・編集へすぐ飛べる
// ようにする (wi-39)。全メタデータは詳細ページで確認できる。
function ClientPaneView({ client, busy, onEdit, onDelete }: { client: AdminClient; busy: boolean; onEdit: () => void; onDelete: () => void }) {
  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-slate-200 bg-white p-5">
        <h2 className="text-lg font-semibold">{client.client_name || client.client_id}</h2>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{client.client_id}</p>
        <div className="mt-4">
          <AdminPaneActions
            detailHref={tenantURL(`/admin/clients/${encodeURIComponent(client.client_id)}`)}
            busy={busy}
            onEdit={onEdit}
            menu={
              <DropdownMenuItem className="text-red-700" onSelect={onDelete}>
                <IconTrash size={17} aria-hidden="true" />
                アプリケーションを削除
              </DropdownMenuItem>
            }
          />
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-5 p-5">
        <Detail label="種別" value={client.client_type} />
        <Detail label="認証方式" value={client.token_endpoint_auth_method} />
        <Detail label="Redirect URI" value={client.redirect_uris.join('\n')} mono />
        <Detail label="Grant types" value={client.grant_types.join(', ')} />
        <Detail label="Scope" value={client.scope || '未設定'} />
        <Detail label="セキュリティ" value={[client.require_pushed_authorization_requests ? 'PAR required' : '', client.dpop_bound_access_tokens ? 'DPoP bound' : ''].filter(Boolean).join(', ') || '標準'} />
      </div>
    </div>
  )
}

// ClientDetailBody はアプリケーションの全メタデータを表示する (wi-39)。
function ClientDetailBody({ client }: { client: AdminClient }) {
  const security =
    [
      client.require_pushed_authorization_requests ? 'PAR 必須' : '',
      client.dpop_bound_access_tokens ? 'DPoP バインド' : '',
    ]
      .filter(Boolean)
      .join(', ') || '標準'
  return (
    <div className="grid gap-5 lg:grid-cols-2">
      <Card className="p-5">
        <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">基本情報</h3>
        <div className="mt-3 flex flex-col gap-5">
          <Detail label="表示名" value={client.client_name || '未設定'} />
          <Detail label="Client ID" value={client.client_id} mono />
          <Detail label="種別" value={client.client_type} />
          <Detail label="認証方式" value={client.token_endpoint_auth_method} />
          <Detail label="作成日時" value={client.created_at} />
        </div>
      </Card>
      <Card className="p-5">
        <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">エンドポイント</h3>
        <div className="mt-3 flex flex-col gap-5">
          <Detail label="Redirect URI" value={client.redirect_uris.join('\n') || '未設定'} mono />
          <Detail label="Grant types" value={client.grant_types.join(', ') || '未設定'} />
          <Detail label="Response types" value={client.response_types.join(', ') || '未設定'} />
          <Detail label="Scope" value={client.scope || '未設定'} />
          {client.jwks_uri ? <Detail label="JWKS URI" value={client.jwks_uri} mono /> : null}
          {client.tls_client_auth_subject_dn ? (
            <Detail label="TLS Subject DN" value={client.tls_client_auth_subject_dn} mono />
          ) : null}
        </div>
      </Card>
      <Card className="p-5 lg:col-span-2">
        <h3 className="text-xs font-bold uppercase tracking-[0.1em] text-slate-400">セキュリティ</h3>
        <div className="mt-3 grid gap-5 sm:grid-cols-3">
          <Detail label="プロファイル" value={security} />
          <Detail label="ID Token 署名アルゴリズム" value={client.id_token_signed_response_alg} />
          <Detail label="FAPI プロファイル" value={client.fapi_profile} />
        </div>
      </Card>
    </div>
  )
}

// AdminClientDetailPage はアプリケーションの全メタデータと編集/削除を扱う専用詳細画面 (wi-39)。
export function AdminClientDetailPage({
  csrfToken,
  actorUsername,
  client: initialClient,
}: AdminClientDetailPageData) {
  const [client, setClient] = useState(initialClient)
  const [form, setForm] = useState<ClientForm>(emptyForm)
  const [dialog, setDialog] = useState<'edit' | 'delete' | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

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
          : 'アプリケーション管理操作を完了できませんでした。',
      )
    } finally {
      setBusy(false)
    }
  }

  function openEdit() {
    setForm({
      client_name: client.client_name ?? '',
      client_type: client.client_type,
      redirect_uris: client.redirect_uris.join('\n'),
      grant_types: client.grant_types.join(', '),
      response_types: client.response_types.join(', '),
      token_endpoint_auth_method: client.token_endpoint_auth_method,
      scope: client.scope,
      jwks_uri: client.jwks_uri ?? '',
      tls_client_auth_subject_dn: client.tls_client_auth_subject_dn ?? '',
      require_pushed_authorization_requests: client.require_pushed_authorization_requests,
      dpop_bound_access_tokens: client.dpop_bound_access_tokens,
    })
    setDialog('edit')
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      await updateAdminClient(csrfToken, client.client_id, {
        client_name: optional(form.client_name),
        redirect_uris: lines(form.redirect_uris),
        grant_types: values(form.grant_types),
        response_types: values(form.response_types),
        scope: form.scope.trim(),
        require_pushed_authorization_requests: form.require_pushed_authorization_requests,
        dpop_bound_access_tokens: form.dpop_bound_access_tokens,
      })
      setDialog(null)
      setClient(await getAdminClient(client.client_id))
    }, 'アプリケーションを更新しました。')
  }

  async function handleDelete() {
    await run(async () => {
      await deleteAdminClient(csrfToken, client.client_id)
      window.location.assign(tenantURL('/admin/clients'))
    }, 'アプリケーションを削除しました。')
  }

  return (
    <>
      <AdminShell
        active="clients"
        actorUsername={actorUsername}
        title={client.client_name || client.client_id}
        description={client.client_id}
        actions={
          <div className="flex items-center gap-2">
            <a
              href={tenantURL('/admin/clients')}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
            >
              <IconArrowLeft size={16} aria-hidden="true" />
              アプリケーション一覧
            </a>
            <Button type="button" disabled={busy} onClick={openEdit}>
              <IconEdit size={16} aria-hidden="true" />
              編集
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={busy}
              onClick={() => setDialog('delete')}
            >
              <IconTrash size={16} aria-hidden="true" />
              削除
            </Button>
          </div>
        }
      >
        {error && <Alert variant="destructive">{error}</Alert>}
        {notice && <Alert variant="success">{notice}</Alert>}
        <ClientDetailBody client={client} />
      </AdminShell>

      {dialog === 'edit' && (
        <ClientFormDialog
          mode="edit"
          form={form}
          busy={busy}
          onChange={setForm}
          onClose={() => setDialog(null)}
          onSubmit={handleSubmit}
        />
      )}
      {dialog === 'delete' && (
        <DeleteDialog
          client={client}
          busy={busy}
          onClose={() => setDialog(null)}
          onConfirm={() => void handleDelete()}
        />
      )}
    </>
  )
}

function ClientFormDialog({ mode, form, busy, onChange, onClose, onSubmit }: { mode: 'create' | 'edit'; form: ClientForm; busy: boolean; onChange: (form: ClientForm) => void; onClose: () => void; onSubmit: (event: FormEvent<HTMLFormElement>) => void }) {
  const set = <K extends keyof ClientForm>(key: K, value: ClientForm[K]) => onChange({ ...form, [key]: value })
  return (
    <Dialog title={mode === 'create' ? 'アプリケーションを追加' : 'アプリケーションを編集'} onClose={onClose}>
      <form onSubmit={onSubmit}>
        <div className="grid max-h-[70vh] gap-5 overflow-y-auto p-6">
          <Field label="表示名"><Input value={form.client_name} onChange={(event) => set('client_name', event.target.value)} /></Field>
          {mode === 'create' && (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="種別"><Select value={form.client_type} onChange={(value) => { const type = value as ClientForm['client_type']; set('client_type', type); set('token_endpoint_auth_method', type === 'public' ? 'none' : 'client_secret_basic') }} options={['confidential', 'public']} /></Field>
              <Field label="認証方式"><Select value={form.token_endpoint_auth_method} onChange={(value) => set('token_endpoint_auth_method', value as AdminClient['token_endpoint_auth_method'])} options={['client_secret_basic', 'client_secret_post', 'private_key_jwt', 'tls_client_auth', 'none']} /></Field>
            </div>
          )}
          <Field label="Redirect URI" hint="1行に1 URI"><Textarea required value={form.redirect_uris} onChange={(event) => set('redirect_uris', event.target.value)} /></Field>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Grant types" hint="カンマ区切り"><Input required value={form.grant_types} onChange={(event) => set('grant_types', event.target.value)} /></Field>
            <Field label="Response types" hint="カンマ区切り"><Input value={form.response_types} onChange={(event) => set('response_types', event.target.value)} /></Field>
          </div>
          <Field label="Scope" hint="空白区切り"><Input value={form.scope} onChange={(event) => set('scope', event.target.value)} /></Field>
          {mode === 'create' && form.token_endpoint_auth_method === 'private_key_jwt' && <Field label="JWKS URI"><Input type="url" required value={form.jwks_uri} onChange={(event) => set('jwks_uri', event.target.value)} /></Field>}
          {mode === 'create' && form.token_endpoint_auth_method === 'tls_client_auth' && <Field label="TLS client certificate Subject DN"><Input required value={form.tls_client_auth_subject_dn} onChange={(event) => set('tls_client_auth_subject_dn', event.target.value)} /></Field>}
          <Check label="PAR を必須にする" checked={form.require_pushed_authorization_requests} onChange={(value) => set('require_pushed_authorization_requests', value)} />
          <Check label="DPoP bound access token を要求する" checked={form.dpop_bound_access_tokens} onChange={(value) => set('dpop_bound_access_tokens', value)} />
        </div>
        <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
          <Button type="button" variant="outline" onClick={onClose}>キャンセル</Button>
          <Button type="submit" disabled={busy}>{mode === 'create' ? '作成' : '保存'}</Button>
        </div>
      </form>
    </Dialog>
  )
}

function DeleteDialog({ client, busy, onClose, onConfirm }: { client: AdminClient; busy: boolean; onClose: () => void; onConfirm: () => void }) {
  return (
    <Dialog title="アプリケーションを削除" onClose={onClose}>
      <div className="p-6">
        <div className="flex gap-3 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-900">
          <IconAlertTriangle className="shrink-0" size={20} />
          <p>削除後、この client ID を使う新しい認可・トークン要求は失敗します。参照中のデータがある場合、削除は拒否されます。</p>
        </div>
        <p className="mt-5 text-sm text-slate-600">削除対象:</p>
        <p className="mt-1 break-all font-mono text-sm font-semibold">{client.client_id}</p>
      </div>
      <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
        <Button variant="outline" onClick={onClose}>キャンセル</Button>
        <Button variant="destructive" disabled={busy} onClick={onConfirm}>削除を確定</Button>
      </div>
    </Dialog>
  )
}

function SecretDialog({ value, onClose }: { value: { clientID: string; secret: string }; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  async function copy() {
    await navigator.clipboard.writeText(value.secret)
    setCopied(true)
  }
  return (
    <Dialog title="Client secret を保存してください" onClose={onClose}>
      <div className="p-6">
        <Alert>この secret はこの画面を閉じると再表示できません。安全な保管先へ保存してください。</Alert>
        <p className="mt-5 text-xs font-semibold text-slate-500">Client ID</p>
        <p className="mt-1 break-all font-mono text-sm">{value.clientID}</p>
        <p className="mt-4 text-xs font-semibold text-slate-500">クライアントシークレット</p>
        <div className="mt-1 flex gap-2">
          <code className="min-w-0 flex-1 break-all rounded-lg bg-slate-950 p-3 text-sm text-white">{value.secret}</code>
          <Button variant="outline" onClick={() => void copy()} aria-label="secretをコピー"><IconCopy size={17} />{copied ? 'コピー済み' : 'コピー'}</Button>
        </div>
      </div>
      <div className="flex justify-end border-t border-slate-200 bg-slate-50 px-6 py-4"><Button onClick={onClose}>保存しました</Button></div>
    </Dialog>
  )
}

function Dialog({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/30 p-5 backdrop-blur-[2px]" role="dialog" aria-modal="true" aria-label={title}>
      <button type="button" className="absolute inset-0 cursor-default" aria-label="閉じる" onClick={onClose} />
      <Card className="relative w-full max-w-2xl overflow-hidden shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-5"><h2 className="text-xl font-semibold">{title}</h2><Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる"><IconX size={18} /></Button></div>
        {children}
      </Card>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: number }) {
  return <Card className="p-4"><p className="text-xs font-semibold text-slate-500">{label}</p><p className="mt-2 text-2xl font-semibold">{value}</p></Card>
}

function Detail({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <div><p className="text-xs font-semibold text-slate-500">{label}</p><p className={cn('mt-1 whitespace-pre-wrap break-all text-sm text-slate-900', mono && 'font-mono text-xs')}>{value}</p></div>
}

function Badge({ children }: { children: React.ReactNode }) {
  return <span className="rounded-full bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-700">{children}</span>
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return <div className="grid gap-2"><Label>{label}</Label>{children}{hint && <p className="text-xs text-slate-500">{hint}</p>}</div>
}

function Select({ value, options, onChange }: { value: string; options: string[]; onChange: (value: string) => void }) {
  return <select value={value} onChange={(event) => onChange(event.target.value)} className="h-12 w-full rounded-lg border border-slate-300 bg-white px-3.5 text-sm outline-none focus:border-blue-600 focus:ring-3 focus:ring-blue-600/10">{options.map((option) => <option key={option}>{option}</option>)}</select>
}

function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className="min-h-24 w-full rounded-lg border border-slate-300 bg-white px-3.5 py-3 text-sm outline-none focus:border-blue-600 focus:ring-3 focus:ring-blue-600/10" />
}

function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return <label className="flex items-center gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm font-medium"><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="size-4" />{label}</label>
}

function values(value: string) {
  return value.split(',').map((item) => item.trim()).filter(Boolean)
}

function lines(value: string) {
  return value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
}

function optional(value: string) {
  return value.trim() || undefined
}
