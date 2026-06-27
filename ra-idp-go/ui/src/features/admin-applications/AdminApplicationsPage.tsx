import {
  IconApps,
  IconArrowLeft,
  IconCheck,
  IconCopy,
  IconExternalLink,
  IconKey,
  IconLink,
  IconPencil,
  IconPlus,
  IconRefresh,
  IconServer,
  IconTrash,
  IconUserPlus,
  IconWorldShare,
  IconX,
} from '@tabler/icons-react'
import { type FormEvent, type ReactNode, useEffect, useMemo, useState } from 'react'
import {
  assignApplication,
  AuthenticationAPIError,
  createAdminApplication,
  deleteAdminApplication,
  listAdminApplications,
  listAdminGroups,
  listAdminUsers,
  listApplicationAssignments,
  tenantURL,
  unassignApplication,
  updateAdminApplication,
  updateApplicationOidcConfig,
  updateApplicationWsFedConfig,
} from '../../api'
import { AdminPaneActions } from '../../components/AdminPaneActions'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { DropdownMenuItem } from '../../components/ui/dropdown-menu'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import { Select, type SelectOption } from '../../components/ui/select'
import type {
  AdminApplication,
  AdminApplicationDetail,
  AdminGroup,
  AdminUser,
  ApplicationAssignment,
  ApplicationStatus,
} from '../../types'

type AppType = 'oidc' | 'wsfed' | 'weblink' | 'service'

const APP_TYPES: { type: AppType; label: string; description: string; icon: typeof IconKey }[] = [
  {
    type: 'oidc',
    label: 'OIDC / OAuth2',
    description: 'OpenID Connect / OAuth2 でログインする最新のアプリ。',
    icon: IconKey,
  },
  {
    type: 'wsfed',
    label: 'WS-Federation',
    description: 'WS-Fed / SAML トークンを使う従来型のアプリ。',
    icon: IconWorldShare,
  },
  {
    type: 'weblink',
    label: 'Web リンク',
    description: 'SSO なしで外部 URL を開くだけのブックマーク。',
    icon: IconLink,
  },
  {
    type: 'service',
    label: 'サービス (M2M)',
    description: 'client_credentials で動く API / バックエンド連携。ログイン画面なし。',
    icon: IconServer,
  },
]

const NAMEID_FORMATS: SelectOption[] = [
  { value: 'urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified', label: 'Unspecified' },
  { value: 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress', label: 'Email アドレス' },
  { value: 'urn:oasis:names:tc:SAML:2.0:nameid-format:persistent', label: 'Persistent' },
]

const STATUS_OPTIONS: SelectOption[] = [
  { value: 'active', label: '有効' },
  { value: 'disabled', label: '無効' },
]

const DEFAULT_NAMEID_FORMAT = NAMEID_FORMATS[0].value
const DEFAULT_NAMEID_SOURCE = 'sub'

function listURL(): string {
  return tenantURL('/admin/applications')
}
function detailURL(id: string): string {
  return tenantURL(`/admin/applications/${encodeURIComponent(id)}`)
}
function editURL(id: string): string {
  return tenantURL(`/admin/applications/${encodeURIComponent(id)}/edit`)
}

function messageOf(cause: unknown, fallback: string): string {
  return cause instanceof AuthenticationAPIError ? cause.message : fallback
}

// parseList は空白・カンマ・改行区切りの入力を一意な URL 配列へ正規化する。
function parseList(value: string): string[] {
  return [
    ...new Set(
      value
        .split(/[\s,]+/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ]
}

function initials(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || '??'
}

function AppIcon({ app, size = 'md' }: { app: AdminApplication; size?: 'sm' | 'md' }) {
  const dim = size === 'sm' ? 'size-9 text-xs' : 'size-11 text-sm'
  if (app.icon_url) {
    return <img src={app.icon_url} alt="" className={`${dim} rounded-lg object-cover`} />
  }
  return (
    <span
      className={`flex ${dim} items-center justify-center rounded-lg border border-blue-100 bg-blue-50 font-bold text-blue-700`}
    >
      {initials(app.name)}
    </span>
  )
}

function StatusBadge({ status }: { status: AdminApplication['status'] }) {
  const active = status === 'active'
  return (
    <span
      className={`rounded-md px-2 py-0.5 text-xs font-medium ${
        active ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500'
      }`}
    >
      {active ? '有効' : '無効'}
    </span>
  )
}

function kindLabel(app: AdminApplication): string {
  if (app.kind === 'weblink') return 'Web リンク'
  if (app.kind === 'service') return 'サービス (M2M)'
  const binding = app.bindings[0]?.type
  if (binding === 'wsfed') return 'WS-Federation'
  if (binding === 'saml') return 'SAML'
  if (binding === 'oidc') return 'OIDC'
  return 'フェデレーション'
}

function KindBadge({ app }: { app: AdminApplication }) {
  return (
    <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
      {kindLabel(app)}
    </span>
  )
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h3 className="text-xs font-bold uppercase tracking-normal text-slate-400">{children}</h3>
}

function ReadOnlyField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-xs font-bold uppercase tracking-normal text-slate-400">{label}</dt>
      <dd className="mt-1 text-sm text-slate-700">{children}</dd>
    </div>
  )
}

function UriList({ values }: { values: string[] }) {
  if (values.length === 0) return <span className="text-slate-400">—</span>
  return (
    <ul className="grid gap-1">
      {values.map((v) => (
        <li key={v} className="break-all font-mono text-xs text-slate-700">
          {v}
        </li>
      ))}
    </ul>
  )
}

// CopyableValue は変更できない値 (client_id / secret 等) を入力欄ではなくテキストとして
// 表示し、コピーボタンだけを添える。フォームに見せないことで「編集不可」を明示する。
function CopyableValue({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <div className="flex items-center gap-2">
      <code className="min-w-0 flex-1 break-all rounded-md bg-slate-50 px-3 py-2 font-mono text-xs text-slate-800">
        {value}
      </code>
      <Button
        type="button"
        variant="outline"
        className="size-9 shrink-0 px-0"
        aria-label="コピー"
        onClick={() => {
          void navigator.clipboard?.writeText(value)
          setCopied(true)
          setTimeout(() => setCopied(false), 1500)
        }}
      >
        {copied ? (
          <IconCheck size={16} className="text-emerald-600" aria-hidden="true" />
        ) : (
          <IconCopy size={16} aria-hidden="true" />
        )}
      </Button>
    </div>
  )
}

function CopyableField({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1.5">
      <Label>{label}</Label>
      <CopyableValue value={value} />
    </div>
  )
}

// ===========================================================================
// 一覧画面
// ===========================================================================

export function AdminApplicationsPage({
  csrfToken,
  actorUsername,
  applications: initial,
}: {
  csrfToken: string
  actorUsername?: string
  applications: AdminApplication[]
}) {
  const [applications, setApplications] = useState(initial)
  const [selectedID, setSelectedID] = useState<string>(() => initial[0]?.application_id ?? '')
  const [showCreate, setShowCreate] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const selected = applications.find((a) => a.application_id === selectedID) ?? null

  async function refresh(preferredID = selectedID) {
    const next = await listAdminApplications()
    setApplications(next)
    setSelectedID(
      next.find((a) => a.application_id === preferredID)?.application_id ??
        next[0]?.application_id ??
        '',
    )
  }

  async function run(action: () => Promise<void>, success: string) {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(success)
    } catch (cause) {
      setError(messageOf(cause, '操作を完了できませんでした。'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AdminShell
      active="applications"
      actorUsername={actorUsername}
      title="アプリケーション"
      description="OIDC・WS-Federation・Web リンク・サービス (M2M) を 1 か所で登録し、利用者への割り当てを管理します。"
      actions={
        <>
          <Button
            variant="outline"
            className="size-9 px-0"
            aria-label="一覧を再読み込み"
            onClick={() => run(() => refresh(), '一覧を更新しました。')}
            disabled={busy}
          >
            <IconRefresh size={16} aria-hidden="true" />
          </Button>
          <Button onClick={() => setShowCreate(true)} disabled={busy}>
            <IconPlus size={16} aria-hidden="true" />
            アプリケーションを追加
          </Button>
        </>
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,420px)]">
        <Card className="overflow-hidden">
          {applications.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center px-6 text-center text-sm text-slate-500">
              <IconApps size={28} className="text-slate-300" aria-hidden="true" />
              <p className="mt-3">アプリケーションはまだありません。</p>
            </div>
          ) : (
            <ul>
              {applications.map((app) => (
                <li key={app.application_id}>
                  <button
                    type="button"
                    onClick={() => setSelectedID(app.application_id)}
                    className={`flex w-full items-center gap-3 border-t border-slate-100 px-4 py-3 text-left first:border-t-0 hover:bg-slate-50 ${
                      selectedID === app.application_id ? 'bg-blue-50/60' : ''
                    }`}
                  >
                    <AppIcon app={app} size="sm" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-semibold text-slate-900">{app.name}</span>
                        <StatusBadge status={app.status} />
                      </div>
                      <div className="mt-0.5">
                        <KindBadge app={app} />
                      </div>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <ApplicationSummaryCard
          key={selectedID || 'none'}
          app={selected}
          busy={busy}
          onDelete={(id) =>
            run(async () => {
              await deleteAdminApplication(csrfToken, id)
              await refresh()
            }, 'アプリケーションを削除しました。')
          }
        />
      </div>

      {showCreate ? (
        <CreateApplicationDialog
          csrfToken={csrfToken}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            window.location.assign(detailURL(id))
          }}
        />
      ) : null}
    </AdminShell>
  )
}

function ApplicationSummaryCard({
  app,
  busy,
  onDelete,
}: {
  app: AdminApplication | null
  busy: boolean
  onDelete: (id: string) => void
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)

  if (!app) {
    return (
      <Card className="flex min-h-48 items-center justify-center p-6 text-sm text-slate-500">
        アプリケーションを選択してください。
      </Card>
    )
  }

  return (
    <Card className="overflow-hidden">
      <div className="border-b border-slate-200 p-5">
        <div className="flex items-start gap-3">
          <AppIcon app={app} />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-lg font-semibold text-slate-950">{app.name}</h2>
              <StatusBadge status={app.status} />
            </div>
            <div className="mt-1">
              <KindBadge app={app} />
            </div>
          </div>
        </div>
        <div className="mt-4">
          <AdminPaneActions
            detailHref={detailURL(app.application_id)}
            busy={busy}
            menu={
              <DropdownMenuItem className="text-red-700" onSelect={() => setConfirmDelete(true)}>
                <IconTrash size={17} aria-hidden="true" />
                アプリケーションを削除
              </DropdownMenuItem>
            }
          />
        </div>
      </div>
      {confirmDelete ? (
        <Alert
          variant="destructive"
          className="m-5 flex flex-wrap items-center justify-between gap-2"
        >
          <span>このアプリケーションを削除しますか？割り当ても解除されます。</span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setConfirmDelete(false)} disabled={busy}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={busy}
              onClick={() => onDelete(app.application_id)}
            >
              <IconTrash size={14} aria-hidden="true" />
              削除を確定
            </Button>
          </div>
        </Alert>
      ) : null}
      <dl className="grid gap-4 p-5">
        {app.kind === 'service' ? (
          <p className="text-xs text-slate-500">
            client_credentials グラントで動く M2M
            クライアントです。詳細はアプリケーションを開いて確認できます。
          </p>
        ) : (
          <ReadOnlyField label="起動 URL">
            {app.launch_url ? (
              <a
                href={app.launch_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 break-all font-mono text-xs text-blue-700 hover:underline"
              >
                {app.launch_url}
                <IconExternalLink size={13} aria-hidden="true" />
              </a>
            ) : (
              <span className="text-slate-400">未設定</span>
            )}
          </ReadOnlyField>
        )}
      </dl>
    </Card>
  )
}

// ===========================================================================
// 詳細画面 (read-only)
// ===========================================================================

export function AdminApplicationDetailPage({
  csrfToken,
  actorUsername,
  detail,
}: {
  csrfToken: string
  actorUsername?: string
  detail: AdminApplicationDetail
}) {
  const app = detail.application
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleDelete() {
    setBusy(true)
    setError('')
    try {
      await deleteAdminApplication(csrfToken, app.application_id)
      window.location.assign(listURL())
    } catch (cause) {
      setError(messageOf(cause, 'アプリケーションを削除できませんでした。'))
      setBusy(false)
    }
  }

  return (
    <AdminShell
      active="applications"
      actorUsername={actorUsername}
      title={app.name}
      description={kindLabel(app)}
      actions={
        <div className="flex items-center gap-2">
          <a
            href={listURL()}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
          >
            <IconArrowLeft size={16} aria-hidden="true" />
            一覧
          </a>
          <Button asChild>
            <a href={editURL(app.application_id)}>
              <IconPencil size={16} aria-hidden="true" />
              編集
            </a>
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={busy}
            onClick={() => setConfirmDelete(true)}
          >
            <IconTrash size={16} aria-hidden="true" />
            削除
          </Button>
        </div>
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {confirmDelete ? (
        <Alert variant="destructive" className="flex flex-wrap items-center justify-between gap-2">
          <span>このアプリケーションを削除しますか？割り当ても解除されます。</span>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setConfirmDelete(false)} disabled={busy}>
              取消
            </Button>
            <Button variant="destructive" disabled={busy} onClick={() => void handleDelete()}>
              <IconTrash size={14} aria-hidden="true" />
              削除を確定
            </Button>
          </div>
        </Alert>
      ) : null}

      <div className="grid max-w-3xl gap-6">
        <Card className="overflow-hidden">
          <div className="flex items-start gap-3 border-b border-slate-200 p-5">
            <AppIcon app={app} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h2 className="truncate text-lg font-semibold text-slate-950">{app.name}</h2>
                <StatusBadge status={app.status} />
              </div>
              <div className="mt-1">
                <KindBadge app={app} />
              </div>
            </div>
          </div>

          <div className="grid gap-6 p-5">
            {app.kind !== 'service' ? (
              <dl className="grid gap-4">
                <ReadOnlyField label="起動 URL">
                  {app.launch_url ? (
                    <a
                      href={app.launch_url}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 break-all font-mono text-xs text-blue-700 hover:underline"
                    >
                      {app.launch_url}
                      <IconExternalLink size={13} aria-hidden="true" />
                    </a>
                  ) : (
                    <span className="text-slate-400">未設定</span>
                  )}
                </ReadOnlyField>
              </dl>
            ) : null}

            {detail.oidc ? (
              <section className="grid gap-3 border-t border-slate-100 pt-5 first:border-t-0 first:pt-0">
                <div className="flex items-center gap-2">
                  <IconKey size={16} className="text-slate-400" aria-hidden="true" />
                  <SectionTitle>
                    {app.kind === 'service' ? 'サービス (M2M)' : 'OIDC / OAuth2'}
                  </SectionTitle>
                </div>
                <CopyableField label="クライアント ID" value={detail.oidc.client_id} />
                {app.kind !== 'service' ? (
                  <ReadOnlyField label="リダイレクト URI">
                    <UriList values={detail.oidc.redirect_uris} />
                  </ReadOnlyField>
                ) : null}
                <ReadOnlyField label="スコープ">
                  <span className="font-mono text-xs">{detail.oidc.scope || '—'}</span>
                </ReadOnlyField>
                {app.kind === 'service' ? (
                  <p className="text-xs text-slate-500">
                    client_credentials グラントで動く M2M
                    クライアントです。ログイン画面・利用者割り当ては持ちません。
                  </p>
                ) : null}
              </section>
            ) : null}

            {detail.wsfed ? (
              <section className="grid gap-3 border-t border-slate-100 pt-5">
                <div className="flex items-center gap-2">
                  <IconWorldShare size={16} className="text-slate-400" aria-hidden="true" />
                  <SectionTitle>WS-Federation</SectionTitle>
                </div>
                <CopyableField label="wtrealm" value={detail.wsfed.wtrealm} />
                <ReadOnlyField label="Reply URL">
                  <UriList values={detail.wsfed.reply_urls} />
                </ReadOnlyField>
                <ReadOnlyField label="NameID 形式">
                  <span className="break-all font-mono text-xs">
                    {NAMEID_FORMATS.find((f) => f.value === detail.wsfed?.name_id_format)?.label ??
                      detail.wsfed.name_id_format}
                  </span>
                </ReadOnlyField>
                <ReadOnlyField label="NameID ソース属性">
                  <span className="font-mono text-xs">{detail.wsfed.name_id_source}</span>
                </ReadOnlyField>
              </section>
            ) : null}

            {app.kind !== 'service' ? (
              <section className="grid gap-3 border-t border-slate-100 pt-5">
                <SectionTitle>割り当て (ユーザー / グループ)</SectionTitle>
                <AssignmentList appID={app.application_id} onError={setError} />
              </section>
            ) : null}
          </div>
        </Card>
      </div>
    </AdminShell>
  )
}

// ===========================================================================
// 編集画面 (基本情報・プロトコル設定・割り当て)
// ===========================================================================

export function AdminApplicationEditPage({
  csrfToken,
  actorUsername,
  detail,
}: {
  csrfToken: string
  actorUsername?: string
  detail: AdminApplicationDetail
}) {
  const app = detail.application
  const [name, setName] = useState(app.name)
  const [iconURL, setIconURL] = useState(app.icon_url ?? '')
  const [launchURL, setLaunchURL] = useState(app.launch_url ?? '')
  const [status, setStatus] = useState<ApplicationStatus>(app.status)
  const [redirects, setRedirects] = useState((detail.oidc?.redirect_uris ?? []).join('\n'))
  const [scope, setScope] = useState(detail.oidc?.scope ?? '')
  const [replies, setReplies] = useState((detail.wsfed?.reply_urls ?? []).join('\n'))
  const [nameIDFormat, setNameIDFormat] = useState(
    detail.wsfed?.name_id_format || DEFAULT_NAMEID_FORMAT,
  )
  const [nameIDSource, setNameIDSource] = useState(
    detail.wsfed?.name_id_source || DEFAULT_NAMEID_SOURCE,
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const nameInvalid = name.trim() === ''

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (nameInvalid) return
    setSaving(true)
    setError('')
    try {
      const metaPatch: Record<string, unknown> = {}
      if (name.trim() !== app.name) metaPatch.name = name.trim()
      if (iconURL.trim() !== (app.icon_url ?? '')) metaPatch.icon_url = iconURL.trim()
      if (app.kind !== 'service' && launchURL.trim() !== (app.launch_url ?? '')) {
        metaPatch.launch_url = launchURL.trim()
      }
      if (status !== app.status) metaPatch.status = status
      if (Object.keys(metaPatch).length > 0) {
        await updateAdminApplication(csrfToken, app.application_id, metaPatch)
      }
      if (detail.oidc) {
        const nextRedirects = parseList(redirects)
        const redirectsChanged =
          app.kind !== 'service' && nextRedirects.join(',') !== detail.oidc.redirect_uris.join(',')
        const scopeChanged = scope.trim() !== detail.oidc.scope
        if (redirectsChanged || scopeChanged) {
          await updateApplicationOidcConfig(csrfToken, app.application_id, {
            redirect_uris: redirectsChanged ? nextRedirects : undefined,
            scope: scopeChanged ? scope.trim() : undefined,
          })
        }
      }
      if (detail.wsfed) {
        const nextReplies = parseList(replies)
        const changed =
          nextReplies.join(',') !== detail.wsfed.reply_urls.join(',') ||
          nameIDFormat !== detail.wsfed.name_id_format ||
          nameIDSource.trim() !== detail.wsfed.name_id_source
        if (changed) {
          await updateApplicationWsFedConfig(csrfToken, app.application_id, {
            reply_urls: nextReplies,
            name_id_format: nameIDFormat,
            name_id_source: nameIDSource.trim(),
          })
        }
      }
      window.location.assign(detailURL(app.application_id))
    } catch (cause) {
      setError(messageOf(cause, 'アプリケーションを更新できませんでした。'))
      setSaving(false)
    }
  }

  return (
    <AdminShell
      active="applications"
      actorUsername={actorUsername}
      title={`${app.name} を編集`}
      description={kindLabel(app)}
      actions={
        <a
          href={detailURL(app.application_id)}
          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50"
        >
          <IconArrowLeft size={16} aria-hidden="true" />
          詳細に戻る
        </a>
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <div className="grid max-w-3xl gap-6">
        <Card className="p-6">
          <form onSubmit={submit} className="grid gap-6">
            <section className="grid gap-4">
              <SectionTitle>基本情報</SectionTitle>
              <div className="grid gap-1.5">
                <Label htmlFor="edit-name">名前</Label>
                <Input
                  id="edit-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  aria-invalid={nameInvalid}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="edit-icon">アイコン URL</Label>
                <Input
                  id="edit-icon"
                  value={iconURL}
                  onChange={(e) => setIconURL(e.target.value)}
                  placeholder="https://…/icon.png"
                />
              </div>
              {app.kind !== 'service' ? (
                <div className="grid gap-1.5">
                  <Label htmlFor="edit-launch">起動 URL</Label>
                  <Input
                    id="edit-launch"
                    value={launchURL}
                    onChange={(e) => setLaunchURL(e.target.value)}
                    placeholder="https://app.example.com/launch"
                  />
                </div>
              ) : null}
              <div className="grid gap-1.5">
                <Label>状態</Label>
                <Select
                  value={status}
                  onValueChange={(v) => setStatus(v as ApplicationStatus)}
                  options={STATUS_OPTIONS}
                  className="w-40"
                />
              </div>
            </section>

            {detail.oidc ? (
              <section className="grid gap-4 border-t border-slate-200 pt-5">
                <div className="flex items-center gap-2">
                  <IconKey size={16} className="text-slate-400" aria-hidden="true" />
                  <SectionTitle>
                    {app.kind === 'service' ? 'サービス (M2M)' : 'OIDC / OAuth2'}
                  </SectionTitle>
                </div>
                <CopyableField label="クライアント ID" value={detail.oidc.client_id} />
                {app.kind !== 'service' ? (
                  <div className="grid gap-1.5">
                    <Label htmlFor="edit-redirects">リダイレクト URI</Label>
                    <textarea
                      id="edit-redirects"
                      value={redirects}
                      onChange={(e) => setRedirects(e.target.value)}
                      rows={3}
                      className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs focus:border-blue-600 focus:outline-none focus:ring-3 focus:ring-blue-600/10"
                      placeholder="https://app.example.com/callback"
                    />
                    <p className="text-xs text-slate-500">改行区切りで複数指定できます。</p>
                  </div>
                ) : null}
                <div className="grid gap-1.5">
                  <Label htmlFor="edit-scope">スコープ</Label>
                  <Input
                    id="edit-scope"
                    value={scope}
                    onChange={(e) => setScope(e.target.value)}
                    className="font-mono text-xs"
                    placeholder="openid profile email"
                  />
                </div>
              </section>
            ) : null}

            {detail.wsfed ? (
              <section className="grid gap-4 border-t border-slate-200 pt-5">
                <div className="flex items-center gap-2">
                  <IconWorldShare size={16} className="text-slate-400" aria-hidden="true" />
                  <SectionTitle>WS-Federation</SectionTitle>
                </div>
                <CopyableField label="wtrealm" value={detail.wsfed.wtrealm} />
                <div className="grid gap-1.5">
                  <Label htmlFor="edit-replies">Reply URL</Label>
                  <textarea
                    id="edit-replies"
                    value={replies}
                    onChange={(e) => setReplies(e.target.value)}
                    rows={2}
                    className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs focus:border-blue-600 focus:outline-none focus:ring-3 focus:ring-blue-600/10"
                    placeholder="https://app.example.com/wsfed"
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label>NameID 形式</Label>
                  <Select
                    value={nameIDFormat}
                    onValueChange={setNameIDFormat}
                    options={NAMEID_FORMATS}
                    className="w-full"
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="edit-nameid-source">NameID ソース属性</Label>
                  <Input
                    id="edit-nameid-source"
                    value={nameIDSource}
                    onChange={(e) => setNameIDSource(e.target.value)}
                    placeholder="sub"
                  />
                </div>
              </section>
            ) : null}

            <div className="flex justify-end gap-2 border-t border-slate-200 pt-5">
              <Button asChild variant="outline">
                <a href={detailURL(app.application_id)}>キャンセル</a>
              </Button>
              <Button type="submit" disabled={saving || nameInvalid}>
                {saving ? '保存中…' : '保存'}
              </Button>
            </div>
          </form>
        </Card>

        {app.kind !== 'service' ? (
          <Card className="p-6">
            <AssignmentManager
              appID={app.application_id}
              csrfToken={csrfToken}
              onError={setError}
            />
          </Card>
        ) : null}
      </div>
    </AdminShell>
  )
}

// ===========================================================================
// 割り当て (read-only リスト / 管理)
// ===========================================================================

function useAssignmentData(appID: string, onError: (msg: string) => void) {
  const [assignments, setAssignments] = useState<ApplicationAssignment[]>([])
  const [users, setUsers] = useState<AdminUser[]>([])
  const [groups, setGroups] = useState<AdminGroup[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let cancelled = false
    void Promise.all([listApplicationAssignments(appID), listAdminUsers(), listAdminGroups()])
      .then(([a, u, g]) => {
        if (cancelled) return
        setAssignments(a)
        setUsers(u)
        setGroups(g)
        setLoaded(true)
      })
      .catch((cause) => onError(messageOf(cause, '割り当てを取得できませんでした。')))
    return () => {
      cancelled = true
    }
  }, [appID, onError])

  return { assignments, setAssignments, users, groups, loaded }
}

function useDisplayName(users: AdminUser[], groups: AdminGroup[]) {
  const userName = useMemo(() => new Map(users.map((u) => [u.sub, u.preferred_username])), [users])
  const groupName = useMemo(() => new Map(groups.map((g) => [g.id, g.name])), [groups])
  return (a: ApplicationAssignment): string => {
    if (a.subject_type === 'user') return userName.get(a.subject_id) ?? a.subject_id
    return groupName.get(a.subject_id) ?? a.subject_id
  }
}

function AssignmentChip({ a, displayName }: { a: ApplicationAssignment; displayName: string }) {
  return (
    <span className="flex items-center gap-2">
      <span
        className={`rounded px-1.5 py-0.5 text-xs ${
          a.subject_type === 'user' ? 'bg-blue-50 text-blue-700' : 'bg-violet-50 text-violet-700'
        }`}
      >
        {a.subject_type === 'user' ? 'ユーザー' : 'グループ'}
      </span>
      <span className="font-medium text-slate-800">{displayName}</span>
    </span>
  )
}

function AssignmentList({ appID, onError }: { appID: string; onError: (msg: string) => void }) {
  const { assignments, users, groups, loaded } = useAssignmentData(appID, onError)
  const displayName = useDisplayName(users, groups)

  if (!loaded) return <p className="text-xs text-slate-400">読み込み中…</p>
  if (assignments.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-slate-200 px-3 py-4 text-center text-xs text-slate-400">
        割り当てはありません。未割り当ての利用者はログインできません。
      </p>
    )
  }
  return (
    <ul className="grid gap-2">
      {assignments.map((a) => (
        <li
          key={`${a.subject_type}:${a.subject_id}`}
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm"
        >
          <AssignmentChip a={a} displayName={displayName(a)} />
        </li>
      ))}
    </ul>
  )
}

function AssignmentManager({
  appID,
  csrfToken,
  onError,
}: {
  appID: string
  csrfToken: string
  onError: (msg: string) => void
}) {
  const { assignments, setAssignments, users, groups, loaded } = useAssignmentData(appID, onError)
  const displayName = useDisplayName(users, groups)
  const [subjectType, setSubjectType] = useState<'user' | 'group'>('user')
  const [subjectID, setSubjectID] = useState('')
  const [busy, setBusy] = useState(false)

  const assignedKeys = useMemo(
    () => new Set(assignments.map((a) => `${a.subject_type}:${a.subject_id}`)),
    [assignments],
  )

  const options: SelectOption[] = useMemo(() => {
    const source =
      subjectType === 'user'
        ? users.map((u) => ({ value: u.sub, label: u.preferred_username }))
        : groups.map((g) => ({ value: g.id, label: g.name }))
    return source.filter((o) => !assignedKeys.has(`${subjectType}:${o.value}`))
  }, [subjectType, users, groups, assignedKeys])

  async function add(event: FormEvent) {
    event.preventDefault()
    if (!subjectID) return
    setBusy(true)
    try {
      const created = await assignApplication(csrfToken, appID, {
        subject_type: subjectType,
        subject_id: subjectID,
      })
      setAssignments((current) => [...current, created])
      setSubjectID('')
    } catch (cause) {
      onError(messageOf(cause, '割り当てを追加できませんでした。'))
    } finally {
      setBusy(false)
    }
  }

  async function remove(a: ApplicationAssignment) {
    try {
      await unassignApplication(csrfToken, appID, a.subject_type, a.subject_id)
      setAssignments((current) =>
        current.filter(
          (x) => !(x.subject_type === a.subject_type && x.subject_id === a.subject_id),
        ),
      )
    } catch (cause) {
      onError(messageOf(cause, '割り当てを解除できませんでした。'))
    }
  }

  return (
    <section className="grid gap-3">
      <SectionTitle>割り当て (ユーザー / グループ)</SectionTitle>
      <p className="text-xs text-slate-500">
        割り当てられた利用者だけがポータルに表示され、ログインできます。未割り当ての利用者はフェデレーションも拒否されます。
      </p>
      {!loaded ? (
        <p className="text-xs text-slate-400">読み込み中…</p>
      ) : assignments.length === 0 ? (
        <p className="rounded-lg border border-dashed border-slate-200 px-3 py-4 text-center text-xs text-slate-400">
          割り当てはありません。
        </p>
      ) : (
        <ul className="grid gap-2">
          {assignments.map((a) => (
            <li
              key={`${a.subject_type}:${a.subject_id}`}
              className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm"
            >
              <AssignmentChip a={a} displayName={displayName(a)} />
              <Button
                variant="ghost"
                className="text-rose-700 hover:bg-rose-50"
                onClick={() => remove(a)}
              >
                <IconX size={14} aria-hidden="true" />
                解除
              </Button>
            </li>
          ))}
        </ul>
      )}

      <form className="flex flex-wrap items-end gap-2" onSubmit={add}>
        <div className="grid gap-1.5">
          <Label>対象</Label>
          <Select
            value={subjectType}
            onValueChange={(v) => {
              setSubjectType(v as 'user' | 'group')
              setSubjectID('')
            }}
            options={[
              { value: 'user', label: 'ユーザー' },
              { value: 'group', label: 'グループ' },
            ]}
            className="w-32"
          />
        </div>
        <div className="grid flex-1 gap-1.5">
          <Label>{subjectType === 'user' ? 'ユーザーを選択' : 'グループを選択'}</Label>
          <Select
            value={subjectID}
            onValueChange={setSubjectID}
            options={options}
            placeholder={options.length === 0 ? '対象がありません' : '選択…'}
            className="w-full"
            disabled={options.length === 0}
          />
        </div>
        <Button type="submit" disabled={busy || !subjectID}>
          <IconUserPlus size={16} aria-hidden="true" />
          割り当て
        </Button>
      </form>
    </section>
  )
}

// ===========================================================================
// 作成ダイアログ
// ===========================================================================

function CreateApplicationDialog({
  csrfToken,
  onClose,
  onCreated,
}: {
  csrfToken: string
  onClose: () => void
  onCreated: (id: string) => void
}) {
  const [type, setType] = useState<AppType>('oidc')
  const [name, setName] = useState('')
  const [iconURL, setIconURL] = useState('')
  const [launchURL, setLaunchURL] = useState('')
  const [redirectURIs, setRedirectURIs] = useState('')
  const [scope, setScope] = useState('')
  const [wtrealm, setWtrealm] = useState('')
  const [replyURLs, setReplyURLs] = useState('')
  const [nameIDFormat, setNameIDFormat] = useState(DEFAULT_NAMEID_FORMAT)
  const [nameIDSource, setNameIDSource] = useState(DEFAULT_NAMEID_SOURCE)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [secret, setSecret] = useState<{ clientID: string; clientSecret: string } | null>(null)
  const [createdID, setCreatedID] = useState('')

  const nameInvalid = name.trim() === ''

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (nameInvalid) return
    setSaving(true)
    setError('')
    try {
      const result = await createAdminApplication(csrfToken, {
        name: name.trim(),
        type,
        icon_url: iconURL.trim() || undefined,
        launch_url: launchURL.trim() || undefined,
        redirect_uris: type === 'oidc' ? parseList(redirectURIs) : undefined,
        scope: type === 'service' ? scope.trim() || undefined : undefined,
        wtrealm: type === 'wsfed' ? wtrealm.trim() : undefined,
        reply_urls: type === 'wsfed' ? parseList(replyURLs) : undefined,
        name_id_format: type === 'wsfed' ? nameIDFormat : undefined,
        name_id_source: type === 'wsfed' ? nameIDSource.trim() : undefined,
      })
      const id = result.application.application_id
      if (result.client_secret && result.client_id) {
        // OIDC / サービスは client_secret を一度だけ提示し、確認後に詳細へ遷移する。
        setCreatedID(id)
        setSecret({ clientID: result.client_id, clientSecret: result.client_secret })
        return
      }
      onCreated(id)
    } catch (cause) {
      setError(messageOf(cause, 'アプリケーションを作成できませんでした。'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/30 p-5 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="app-create-title"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label="閉じる"
        onClick={onClose}
      />
      <Card className="relative flex max-h-[88vh] w-full max-w-xl flex-col overflow-hidden shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <div>
            <p className="text-xs font-bold uppercase tracking-normal text-blue-700">
              アプリケーション
            </p>
            <h2 id="app-create-title" className="mt-1 text-xl font-semibold">
              {secret ? '作成しました' : 'アプリケーションを追加'}
            </h2>
          </div>
          <Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる">
            <IconX size={18} aria-hidden="true" />
          </Button>
        </div>

        {secret ? (
          <div className="grid gap-4 p-6">
            <Alert variant="success">
              クライアントを作成しました。クライアントシークレットは
              <strong>この画面でしか確認できません</strong>。安全に保管してください。
            </Alert>
            <CopyableField label="クライアント ID" value={secret.clientID} />
            <CopyableField label="クライアントシークレット" value={secret.clientSecret} />
            <div className="flex justify-end">
              <Button type="button" onClick={() => onCreated(createdID)}>
                <IconCheck size={16} aria-hidden="true" />
                保管しました
              </Button>
            </div>
          </div>
        ) : (
          <form onSubmit={submit} className="flex min-h-0 flex-1 flex-col">
            <div className="min-h-0 flex-1 overflow-y-auto">
              <div className="grid gap-6 p-6">
                <section className="grid gap-2">
                  <SectionTitle>種別</SectionTitle>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {APP_TYPES.map((option) => {
                      const Icon = option.icon
                      const active = type === option.type
                      return (
                        <button
                          key={option.type}
                          type="button"
                          onClick={() => setType(option.type)}
                          className={`grid gap-1.5 rounded-xl border p-3 text-left transition ${
                            active
                              ? 'border-blue-500 bg-blue-50/60 ring-2 ring-blue-500/20'
                              : 'border-slate-200 hover:border-slate-300'
                          }`}
                        >
                          <Icon
                            size={20}
                            className={active ? 'text-blue-600' : 'text-slate-400'}
                            aria-hidden="true"
                          />
                          <span className="text-sm font-semibold text-slate-900">
                            {option.label}
                          </span>
                          <span className="text-xs leading-snug text-slate-500">
                            {option.description}
                          </span>
                        </button>
                      )
                    })}
                  </div>
                </section>

                <section className="grid gap-4 border-t border-slate-200 pt-5">
                  <SectionTitle>基本情報</SectionTitle>
                  <div className="grid gap-1.5">
                    <Label htmlFor="app-name">名前</Label>
                    <Input
                      id="app-name"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      required
                      placeholder="Payroll"
                    />
                  </div>
                  <div className="grid gap-1.5">
                    <Label htmlFor="app-icon">アイコン URL (任意)</Label>
                    <Input
                      id="app-icon"
                      value={iconURL}
                      onChange={(e) => setIconURL(e.target.value)}
                      placeholder="https://…/icon.png"
                    />
                  </div>
                  {type !== 'service' ? (
                    <div className="grid gap-1.5">
                      <Label htmlFor="app-launch">
                        {type === 'weblink' ? 'リンク先 URL' : '起動 URL (任意)'}
                      </Label>
                      <Input
                        id="app-launch"
                        value={launchURL}
                        onChange={(e) => setLaunchURL(e.target.value)}
                        placeholder="https://app.example.com"
                        required={type === 'weblink'}
                      />
                      {type !== 'weblink' ? (
                        <p className="text-xs text-slate-500">
                          ポータルのタイルから開く初期ログイン URL。後から設定もできます。
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                </section>

                {type === 'service' ? (
                  <section className="grid gap-4 border-t border-slate-200 pt-5">
                    <SectionTitle>サービス (M2M)</SectionTitle>
                    <div className="grid gap-1.5">
                      <Label htmlFor="app-scope">スコープ (任意)</Label>
                      <Input
                        id="app-scope"
                        value={scope}
                        onChange={(e) => setScope(e.target.value)}
                        className="font-mono text-xs"
                        placeholder="catalog:read invoice:read"
                      />
                      <p className="text-xs text-slate-500">
                        client_credentials で発行されるトークンのスコープ。クライアント ID
                        とシークレットは自動生成されます。
                      </p>
                    </div>
                  </section>
                ) : null}

                {type === 'oidc' ? (
                  <section className="grid gap-4 border-t border-slate-200 pt-5">
                    <SectionTitle>OIDC / OAuth2</SectionTitle>
                    <div className="grid gap-1.5">
                      <Label htmlFor="app-redirects">リダイレクト URI</Label>
                      <textarea
                        id="app-redirects"
                        value={redirectURIs}
                        onChange={(e) => setRedirectURIs(e.target.value)}
                        rows={3}
                        required
                        className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs focus:border-blue-600 focus:outline-none focus:ring-3 focus:ring-blue-600/10"
                        placeholder="https://app.example.com/callback"
                      />
                      <p className="text-xs text-slate-500">
                        改行区切りで複数指定できます。クライアント ID
                        とシークレットは自動生成されます。
                      </p>
                    </div>
                  </section>
                ) : null}

                {type === 'wsfed' ? (
                  <section className="grid gap-4 border-t border-slate-200 pt-5">
                    <SectionTitle>WS-Federation</SectionTitle>
                    <div className="grid gap-1.5">
                      <Label htmlFor="app-wtrealm">wtrealm</Label>
                      <Input
                        id="app-wtrealm"
                        value={wtrealm}
                        onChange={(e) => setWtrealm(e.target.value)}
                        required
                        className="font-mono text-xs"
                        placeholder="urn:app:example"
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor="app-replies">Reply URL</Label>
                      <textarea
                        id="app-replies"
                        value={replyURLs}
                        onChange={(e) => setReplyURLs(e.target.value)}
                        rows={2}
                        required
                        className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs focus:border-blue-600 focus:outline-none focus:ring-3 focus:ring-blue-600/10"
                        placeholder="https://app.example.com/wsfed"
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label>NameID 形式</Label>
                      <Select
                        value={nameIDFormat}
                        onValueChange={setNameIDFormat}
                        options={NAMEID_FORMATS}
                        className="w-full"
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor="app-nameid-source">NameID ソース属性</Label>
                      <Input
                        id="app-nameid-source"
                        value={nameIDSource}
                        onChange={(e) => setNameIDSource(e.target.value)}
                        placeholder="sub"
                      />
                    </div>
                  </section>
                ) : null}

                {error ? <Alert variant="destructive">{error}</Alert> : null}
              </div>
            </div>
            <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
              <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
                キャンセル
              </Button>
              <Button type="submit" disabled={saving || nameInvalid}>
                {saving ? '作成中…' : '作成'}
              </Button>
            </div>
          </form>
        )}
      </Card>
    </div>
  )
}
