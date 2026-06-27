import {
  IconActivity,
  IconArrowRight,
  IconBuildingCommunity,
  IconCheckupList,
  IconChevronRight,
  IconKey,
  IconShieldLock,
  IconUserPlus,
  IconUsers,
} from '@tabler/icons-react'
import { tenantURL } from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Card } from '../../components/ui/card'
import { cn } from '../../lib/utils'
import type { AdminAuditEvent } from '../../types'

const DEFAULT_TENANT_ID = 'default'

export function AdminDashboardPage({
  actorUsername,
  actorRoles,
  actorTenantID,
  userCount,
  activeUserCount,
  disabledUserCount,
  clientCount,
  grantedConsentCount,
  auditEventCount24h,
  recentEvents,
}: {
  actorUsername?: string
  actorRoles: string[]
  actorTenantID: string
  userCount: number
  activeUserCount: number
  disabledUserCount: number
  clientCount: number
  grantedConsentCount: number
  auditEventCount24h: number
  recentEvents: AdminAuditEvent[]
}) {
  const showTenantsLink = actorRoles.includes('system_admin') && actorTenantID === DEFAULT_TENANT_ID

  return (
    <AdminShell
      active="dashboard"
      actorUsername={actorUsername}
      title="ダッシュボード"
      description="テナント内のユーザー、アプリケーション、同意、監査ログの状況を一目で確認できます。"
    >
      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="サマリー">
        <MetricCard
          label="総ユーザー"
          value={userCount}
          icon={IconUsers}
          tone="blue"
          hint={`有効 ${activeUserCount} / 無効 ${disabledUserCount}`}
        />
        <MetricCard label="登録アプリケーション" value={clientCount} icon={IconKey} tone="violet" />
        <MetricCard
          label="付与済みの同意"
          value={grantedConsentCount}
          icon={IconCheckupList}
          tone="green"
        />
        <MetricCard
          label="監査イベント (24h)"
          value={auditEventCount24h}
          icon={IconActivity}
          tone="amber"
        />
      </section>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <Card className="overflow-hidden">
          <div className="flex items-center justify-between border-b border-slate-200 px-5 py-4">
            <div>
              <h2 className="text-sm font-semibold text-slate-900">直近の監査イベント</h2>
              <p className="mt-0.5 text-xs text-slate-500">過去 24 時間の最新 5 件です。</p>
            </div>
            <a
              href={tenantURL('/admin/audit_events')}
              className="inline-flex items-center gap-1 text-xs font-semibold text-blue-700 hover:text-blue-800"
            >
              すべて表示
              <IconChevronRight size={14} aria-hidden="true" />
            </a>
          </div>
          {recentEvents.length === 0 ? (
            <div className="px-5 py-10 text-center text-sm text-slate-500">
              直近 24 時間に記録された監査イベントはありません。
            </div>
          ) : (
            <ul className="divide-y divide-slate-100">
              {recentEvents.map((event) => (
                <li key={event.id}>
                  <a
                    href={`${tenantURL('/admin/audit_events')}?type=${encodeURIComponent(event.type)}`}
                    className="flex items-start gap-3 px-5 py-3.5 transition-colors hover:bg-slate-50"
                  >
                    <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-slate-100 text-slate-500">
                      <IconActivity size={16} aria-hidden="true" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-semibold text-slate-900">{event.type}</p>
                      <p className="mt-0.5 truncate text-xs text-slate-500">
                        {formatDateTime(event.occurred_at)} · {summarizeActor(event)}
                      </p>
                    </div>
                    <IconChevronRight
                      size={16}
                      className="mt-1 shrink-0 text-slate-400"
                      aria-hidden="true"
                    />
                  </a>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="p-5">
          <h2 className="text-sm font-semibold text-slate-900">クイックリンク</h2>
          <p className="mt-0.5 text-xs text-slate-500">よく使う操作にすばやくアクセスします。</p>
          <ul className="mt-4 grid gap-2">
            <QuickLink
              href={tenantURL('/admin/users')}
              icon={IconUserPlus}
              label="ユーザーを追加"
              description="新しい組織アカウントを作成します。"
            />
            <QuickLink
              href={tenantURL('/admin/applications')}
              icon={IconKey}
              label="アプリケーションを追加"
              description="アプリケーションを登録します。"
            />
            <QuickLink
              href={tenantURL('/admin/keys')}
              icon={IconShieldLock}
              label="署名鍵を確認"
              description="JWT 署名鍵の状態とローテーションを確認します。"
            />
            <QuickLink
              href={tenantURL('/admin/audit_events')}
              icon={IconActivity}
              label="監査ログを開く"
              description="DomainEvent を絞り込み・調査します。"
            />
            {showTenantsLink ? (
              <QuickLink
                href={`/realms/${DEFAULT_TENANT_ID}/admin/tenants`}
                icon={IconBuildingCommunity}
                label="テナントを管理"
                description="システム管理者のみ。テナントの作成・無効化を行います。"
              />
            ) : null}
          </ul>
        </Card>
      </div>
    </AdminShell>
  )
}

function MetricCard({
  label,
  value,
  icon: Icon,
  tone,
  hint,
}: {
  label: string
  value: number
  icon: typeof IconUsers
  tone: 'blue' | 'green' | 'violet' | 'amber'
  hint?: string
}) {
  const tones = {
    blue: 'bg-blue-50 text-blue-700 ring-blue-100',
    green: 'bg-emerald-50 text-emerald-700 ring-emerald-100',
    violet: 'bg-indigo-50 text-indigo-700 ring-indigo-100',
    amber: 'bg-amber-50 text-amber-700 ring-amber-100',
  }
  return (
    <Card className="group flex items-center gap-4 p-4 transition-[border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-[0_22px_58px_-34px_rgb(15_23_42/48%)]">
      <span
        className={cn('flex size-10 items-center justify-center rounded-lg ring-1', tones[tone])}
      >
        <Icon size={20} stroke={1.8} aria-hidden="true" />
      </span>
      <div className="min-w-0">
        <p className="text-2xl font-semibold tracking-normal text-slate-950">{value}</p>
        <p className="truncate text-xs font-medium text-slate-500">{label}</p>
        {hint ? <p className="mt-1 truncate text-[0.68rem] text-slate-400">{hint}</p> : null}
      </div>
    </Card>
  )
}

function QuickLink({
  href,
  icon: Icon,
  label,
  description,
}: {
  href: string
  icon: typeof IconUsers
  label: string
  description: string
}) {
  return (
    <li>
      <a
        href={href}
        className="flex items-start gap-3 rounded-lg border border-slate-200/80 bg-white/70 p-3 transition-[background-color,border-color,box-shadow] hover:border-slate-300 hover:bg-white hover:shadow-xs"
      >
        <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-slate-100 text-slate-700">
          <Icon size={18} stroke={1.8} aria-hidden="true" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-slate-900">{label}</span>
          <span className="mt-0.5 block text-xs leading-5 text-slate-500">{description}</span>
        </span>
        <IconArrowRight size={16} className="mt-1 shrink-0 text-slate-400" aria-hidden="true" />
      </a>
    </li>
  )
}

function summarizeActor(event: AdminAuditEvent): string {
  const payload = event.payload as { actor_sub?: string; sub?: string; target_sub?: string }
  const actor = payload.actor_sub ?? payload.sub
  const target = payload.target_sub
  if (actor && target && actor !== target) return `${actor} → ${target}`
  if (actor) return actor
  if (target) return target
  return event.tenant_id
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('ja-JP', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
