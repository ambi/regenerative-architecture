import { IconMail, IconShieldLock, IconTag } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, tenantURL, updateAdminSettings } from '../api'
import { AdminShell } from '../components/AdminShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { cn } from '../lib/utils'
import type { AdminSettings, AdminSettingsPage as AdminSettingsPageData } from '../types'

const DEFAULT_TENANT_ID = 'default'

type TabKey = 'general' | 'password-policy' | 'email'

type Tab = {
  key: TabKey
  label: string
  description: string
  icon: typeof IconTag
  disabled?: boolean
}

const tabs: Tab[] = [
  {
    key: 'general',
    label: '一般',
    description: 'テナント表示名などの基本情報を管理します。',
    icon: IconTag,
  },
  {
    key: 'password-policy',
    label: 'パスワードポリシー',
    description:
      'テナント単位の上書き値。空欄のフィールドは ra-idp の標準値が適用されます。',
    icon: IconShieldLock,
  },
  {
    key: 'email',
    label: 'メール送信',
    description: '別 WI で扱う予定です。現状は環境変数経由で設定します。',
    icon: IconMail,
    disabled: true,
  },
]

export function AdminSettingsPage({
  csrfToken,
  actorUsername,
  actorRoles,
  actorTenantID,
  settings: initial,
}: AdminSettingsPageData) {
  const [settings, setSettings] = useState(initial)
  const [active, setActive] = useState<TabKey>('general')
  const isSystemAdminOnDefault =
    actorRoles.includes('system_admin') && actorTenantID === DEFAULT_TENANT_ID

  return (
    <AdminShell
      active="settings"
      actorUsername={actorUsername}
      title="設定"
      description="このテナントの管理者が触れる設定を集約します。"
    >
      {isSystemAdminOnDefault ? (
        <Alert>
          <p className="text-sm text-slate-700">
            他テナントの設定を編集するには
            <a
              href={`/realms/${DEFAULT_TENANT_ID}/admin/tenants`}
              className="ml-1 font-medium text-blue-700 hover:underline"
            >
              テナント
            </a>
            ページを利用してください。
          </p>
        </Alert>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav className="flex flex-col gap-1" aria-label="設定タブ">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => !tab.disabled && setActive(tab.key)}
              disabled={tab.disabled}
              aria-current={active === tab.key ? 'page' : undefined}
              className={cn(
                'flex items-center gap-3 rounded-lg px-3 py-2 text-left text-sm font-medium',
                tab.disabled
                  ? 'cursor-not-allowed text-slate-400'
                  : active === tab.key
                    ? 'bg-blue-50 text-blue-800'
                    : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
              )}
            >
              <tab.icon size={18} stroke={1.8} aria-hidden="true" />
              <span className="flex-1">{tab.label}</span>
              {tab.disabled ? (
                <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-slate-500">
                  予定
                </span>
              ) : null}
            </button>
          ))}
        </nav>

        <div className="min-w-0">
          {active === 'general' ? (
            <GeneralTab
              csrfToken={csrfToken}
              settings={settings}
              onSaved={(next) => setSettings(next)}
            />
          ) : null}
          {active === 'password-policy' ? (
            <PasswordPolicyTab
              csrfToken={csrfToken}
              settings={settings}
              onSaved={(next) => setSettings(next)}
            />
          ) : null}
          {active === 'email' ? (
            <Card className="p-6">
              <h2 className="text-base font-semibold text-slate-900">メール送信</h2>
              <p className="mt-2 text-sm text-slate-600">
                送信先 SMTP の設定は ADR-035 に従い環境変数で管理しています。UI 経由の編集は
                別 WI で扱います。
              </p>
            </Card>
          ) : null}
        </div>
      </div>
    </AdminShell>
  )
}

function GeneralTab({
  csrfToken,
  settings,
  onSaved,
}: {
  csrfToken: string
  settings: AdminSettings
  onSaved: (next: AdminSettings) => void
}) {
  const [displayName, setDisplayName] = useState(settings.display_name)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const trimmed = displayName.trim()
      if (!trimmed) {
        setError('表示名を入力してください。')
        return
      }
      if (trimmed === settings.display_name) {
        setNotice('変更はありません。')
        return
      }
      const next = await updateAdminSettings(csrfToken, { display_name: trimmed })
      onSaved(next)
      setNotice('表示名を更新しました。')
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '設定を更新できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card className="p-6">
      <header>
        <h2 className="text-base font-semibold text-slate-900">一般</h2>
        <p className="mt-1 text-sm text-slate-600">テナントの表示名を変更します。</p>
      </header>
      <form onSubmit={handleSave} className="mt-5 grid gap-4">
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {notice ? <Alert variant="success">{notice}</Alert> : null}
        <div className="grid gap-1.5">
          <Label htmlFor="tenant-id">テナント ID</Label>
          <Input
            id="tenant-id"
            value={settings.tenant_id}
            readOnly
            aria-readonly="true"
            className="bg-slate-50 font-mono"
            tabIndex={-1}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="display-name">表示名</Label>
          <Input
            id="display-name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            maxLength={200}
          />
          <p className="text-xs text-slate-500">管理画面と承諾画面に表示される名前です。</p>
        </div>
        <div className="flex items-center gap-2">
          <Button type="submit" disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </Button>
          <a
            href={tenantURL('/admin')}
            className="text-sm font-medium text-slate-500 hover:text-slate-700"
          >
            キャンセル
          </a>
        </div>
      </form>
    </Card>
  )
}

function PasswordPolicyTab({
  csrfToken,
  settings,
  onSaved,
}: {
  csrfToken: string
  settings: AdminSettings
  onSaved: (next: AdminSettings) => void
}) {
  const override = settings.password_policy_override
  const defaults = settings.password_policy_defaults
  const [minLength, setMinLength] = useState(override?.min_length?.toString() ?? '')
  const [maxLength, setMaxLength] = useState(override?.max_length?.toString() ?? '')
  const [historyDepth, setHistoryDepth] = useState(override?.history_depth?.toString() ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const policy: AdminSettings['password_policy_override'] = {}
      if (minLength.trim()) policy.min_length = Number.parseInt(minLength, 10)
      if (maxLength.trim()) policy.max_length = Number.parseInt(maxLength, 10)
      if (historyDepth.trim()) policy.history_depth = Number.parseInt(historyDepth, 10)
      const next = await updateAdminSettings(csrfToken, {
        password_policy_override: policy,
      })
      onSaved(next)
      setMinLength(next.password_policy_override?.min_length?.toString() ?? '')
      setMaxLength(next.password_policy_override?.max_length?.toString() ?? '')
      setHistoryDepth(next.password_policy_override?.history_depth?.toString() ?? '')
      setNotice('パスワードポリシーを更新しました。')
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'パスワードポリシーを更新できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card className="p-6">
      <header>
        <h2 className="text-base font-semibold text-slate-900">パスワードポリシー</h2>
        <p className="mt-1 text-sm text-slate-600">
          指定したフィールドのみテナント固有の値で上書きします。空欄のフィールドは
          ra-idp の標準値が適用されます。
        </p>
        <dl className="mt-3 grid grid-cols-3 gap-3 rounded-md border border-slate-200 bg-slate-50 px-4 py-3 text-xs">
          <div>
            <dt className="text-slate-500">標準 最小長</dt>
            <dd className="mt-0.5 text-sm font-semibold text-slate-900">
              {defaults.min_length} 文字
            </dd>
          </div>
          <div>
            <dt className="text-slate-500">標準 最大長</dt>
            <dd className="mt-0.5 text-sm font-semibold text-slate-900">
              {defaults.max_length} 文字
            </dd>
          </div>
          <div>
            <dt className="text-slate-500">標準 履歴件数</dt>
            <dd className="mt-0.5 text-sm font-semibold text-slate-900">
              {defaults.history_depth} 件
            </dd>
          </div>
        </dl>
        <p className="mt-2 text-xs text-slate-500">
          標準値より弱い設定 (最小長を下げる / 最大長を上げる / 履歴件数を減らす) は
          サーバ側で拒否されます。
        </p>
      </header>
      <form onSubmit={handleSave} className="mt-5 grid gap-4">
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {notice ? <Alert variant="success">{notice}</Alert> : null}
        <div className="grid gap-4 sm:grid-cols-3">
          <PolicyField
            id="min-length"
            label="最小長 (min_length)"
            value={minLength}
            onChange={setMinLength}
            min={defaults.min_length}
            max={defaults.max_length}
            placeholder={defaults.min_length.toString()}
            hint={`${defaults.min_length} 以上`}
          />
          <PolicyField
            id="max-length"
            label="最大長 (max_length)"
            value={maxLength}
            onChange={setMaxLength}
            min={defaults.min_length}
            max={defaults.max_length}
            placeholder={defaults.max_length.toString()}
            hint={`${defaults.max_length} 以下`}
          />
          <PolicyField
            id="history-depth"
            label="履歴件数 (history_depth)"
            value={historyDepth}
            onChange={setHistoryDepth}
            min={defaults.history_depth}
            max={50}
            placeholder={defaults.history_depth.toString()}
            hint={`${defaults.history_depth} 以上`}
          />
        </div>
        <div>
          <Button type="submit" disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </Button>
        </div>
      </form>
    </Card>
  )
}

function PolicyField({
  id,
  label,
  value,
  onChange,
  min,
  max,
  placeholder,
  hint,
}: {
  id: string
  label: string
  value: string
  onChange: (next: string) => void
  min: number
  max: number
  placeholder: string
  hint: string
}) {
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        type="number"
        min={min}
        max={max}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
      <p className="text-xs text-slate-500">{hint}</p>
    </div>
  )
}
