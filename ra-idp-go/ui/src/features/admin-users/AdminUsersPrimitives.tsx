import type { IconUser, IconUsers } from '@tabler/icons-react'
import type { ComponentProps } from 'react'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import { cn } from '../../lib/utils'
import type { AdminUser } from '../../types'

export function Metric({
  label,
  value,
  icon: Icon,
  tone,
}: {
  label: string
  value: number
  icon: typeof IconUsers
  tone: 'blue' | 'green' | 'violet' | 'amber'
}) {
  const tones = {
    blue: 'bg-blue-50 text-blue-700',
    green: 'bg-emerald-50 text-emerald-700',
    violet: 'bg-violet-50 text-violet-700',
    amber: 'bg-amber-50 text-amber-700',
  }
  return (
    <Card className="flex items-center gap-4 p-4">
      <span className={cn('flex size-10 items-center justify-center rounded-xl', tones[tone])}>
        <Icon size={20} stroke={1.8} aria-hidden="true" />
      </span>
      <div>
        <p className="text-2xl font-semibold tracking-tight text-slate-950">{value}</p>
        <p className="text-xs font-medium text-slate-500">{label}</p>
      </div>
    </Card>
  )
}

export function UserAvatar({ user, large = false }: { user: AdminUser; large?: boolean }) {
  const label = (user.name || user.preferred_username).slice(0, 2).toUpperCase()
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-100 to-indigo-100 font-bold text-blue-800 ring-1 ring-inset ring-blue-200/70',
        large ? 'size-11 text-sm' : 'size-9 text-xs',
      )}
    >
      {label}
    </span>
  )
}

export function RoleList({ roles }: { roles: string[] }) {
  if (roles.length === 0) return <span className="text-xs text-slate-400">権限なし</span>
  return (
    <div className="flex flex-wrap gap-1.5">
      {roles.slice(0, 2).map((role) => (
        <span
          key={role}
          className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[0.68rem] font-semibold text-slate-700"
        >
          {role}
        </span>
      ))}
      {roles.length > 2 && (
        <span className="rounded-md bg-slate-100 px-2 py-1 text-[0.68rem] font-semibold text-slate-500">
          +{roles.length - 2}
        </span>
      )}
    </div>
  )
}

export function StatusBadge({
  disabled,
  compact = false,
}: {
  disabled: boolean
  compact?: boolean
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full font-semibold',
        compact ? 'px-2 py-0.5 text-[0.65rem]' : 'px-2.5 py-1 text-xs',
        disabled ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700',
      )}
    >
      <span className={cn('size-1.5 rounded-full', disabled ? 'bg-red-500' : 'bg-emerald-500')} />
      {disabled ? '無効' : '有効'}
    </span>
  )
}

export function DetailRow({
  icon: Icon,
  label,
  value,
  mono = false,
}: {
  icon: typeof IconUser
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="grid grid-cols-[24px_90px_minmax(0,1fr)] items-start gap-2">
      <Icon size={16} className="mt-0.5 text-slate-400" aria-hidden="true" />
      <dt className="text-slate-500">{label}</dt>
      <dd className={cn('min-w-0 break-all text-slate-800', mono && 'font-mono text-xs')}>
        {value}
      </dd>
    </div>
  )
}

type FieldProps = {
  id: string
  label: string
  type?: string
  placeholder?: string
  required?: boolean
  minLength?: number
  description?: string
} & Omit<
  ComponentProps<typeof Input>,
  'id' | 'name' | 'type' | 'placeholder' | 'required' | 'minLength'
>

export function Field({ id, label, type = 'text', description, ...props }: FieldProps) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} name={id} type={type} {...props} />
      {description && <p className="text-xs leading-5 text-slate-500">{description}</p>}
    </div>
  )
}

export function parseRoles(value: string) {
  return [
    ...new Set(
      value
        .split(',')
        .map((role) => role.trim())
        .filter(Boolean),
    ),
  ]
}

export function optionalValue(value: FormDataEntryValue | null) {
  const normalized = String(value ?? '').trim()
  return normalized || undefined
}

export function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('ja-JP', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
