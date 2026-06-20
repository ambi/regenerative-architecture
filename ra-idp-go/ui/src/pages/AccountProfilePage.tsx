import { IconArrowLeft } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, tenantURL, updateAccountProfile } from '../api'
import { AuthShell } from '../components/AuthShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import type {
  AccountProfile,
  AccountProfilePage as PageProps,
  AttributeValue,
  UserAttributeDef,
} from '../types'

// 編集フォーム上の属性値は文字列で保持し、保存時に AttributeValue へ整形する。
type AttributeDraft = Record<string, string>

function draftFromProfile(profile: AccountProfile): AttributeDraft {
  const draft: AttributeDraft = {}
  for (const def of profile.editable_attributes) {
    const value = profile.attributes[def.key]
    draft[def.key] = value ? valueToText(value) : ''
  }
  return draft
}

function valueToText(value: AttributeValue): string {
  switch (value.type) {
    case 'string':
      return value.string ?? ''
    case 'date':
      return value.date ?? ''
    case 'number':
      return value.number?.toString() ?? ''
    case 'boolean':
      return value.boolean ? 'true' : 'false'
    case 'string_array':
      return (value.string_array ?? []).join(', ')
    default:
      return ''
  }
}

// textToValue は空入力なら undefined を返し、その key を送らない (self-delete はしない)。
function textToValue(def: UserAttributeDef, text: string): AttributeValue | undefined {
  const trimmed = text.trim()
  switch (def.type) {
    case 'boolean':
      return { type: 'boolean', boolean: text === 'true' }
    case 'number':
      return trimmed ? { type: 'number', number: Number(trimmed) } : undefined
    case 'date':
      return trimmed ? { type: 'date', date: trimmed } : undefined
    case 'string_array': {
      const items = trimmed
        .split(',')
        .map((item) => item.trim())
        .filter((item) => item.length > 0)
      return items.length ? { type: 'string_array', string_array: items } : undefined
    }
    default:
      return trimmed ? { type: 'string', string: trimmed } : undefined
  }
}

export function AccountProfilePage({ csrfToken, profile: initial }: PageProps) {
  const [profile, setProfile] = useState(initial)
  const [name, setName] = useState(initial.name ?? '')
  const [givenName, setGivenName] = useState(initial.given_name ?? '')
  const [familyName, setFamilyName] = useState(initial.family_name ?? '')
  const [attributes, setAttributes] = useState<AttributeDraft>(draftFromProfile(initial))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const nextAttributes: AccountProfile['attributes'] = {}
      for (const def of profile.editable_attributes) {
        const value = textToValue(def, attributes[def.key] ?? '')
        if (value) {
          nextAttributes[def.key] = value
        }
      }
      const next = await updateAccountProfile(csrfToken, {
        name: name.trim() || undefined,
        given_name: givenName.trim() || undefined,
        family_name: familyName.trim() || undefined,
        attributes: nextAttributes,
      })
      setProfile(next)
      setAttributes(draftFromProfile(next))
      setNotice('プロフィールを更新しました。')
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'プロフィールを更新できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <AuthShell
      asideTitle="プロフィール"
      asideText="表示名と、編集が許可された属性を更新できます。"
    >
      <div className="grid gap-6">
        <header>
          <h1 className="text-xl font-semibold text-slate-900">プロフィール</h1>
          <p className="mt-1 text-sm text-slate-600">
            <span className="font-mono">{profile.preferred_username}</span> としてサインイン中
          </p>
        </header>

        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {notice ? <Alert variant="success">{notice}</Alert> : null}

        <dl className="grid grid-cols-2 gap-3 rounded-md border border-slate-200 bg-slate-50 px-4 py-3 text-sm">
          <ReadField label="メール" value={profile.email ?? '—'} />
          <ReadField
            label="メール検証"
            value={profile.email_verified ? '検証済み' : '未検証'}
          />
          <ReadField label="MFA" value={profile.mfa_enrolled ? '登録済み' : '未登録'} />
          <ReadField label="状態" value={profile.status} />
        </dl>

        <form onSubmit={handleSave} className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="name">表示名</Label>
            <Input id="name" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="given-name">名 (given_name)</Label>
              <Input
                id="given-name"
                value={givenName}
                onChange={(event) => setGivenName(event.target.value)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="family-name">姓 (family_name)</Label>
              <Input
                id="family-name"
                value={familyName}
                onChange={(event) => setFamilyName(event.target.value)}
              />
            </div>
          </div>

          {profile.editable_attributes.length > 0 ? (
            <fieldset className="grid gap-4 rounded-lg border border-slate-200 p-4">
              <legend className="px-1 text-sm font-medium text-slate-700">属性</legend>
              {profile.editable_attributes.map((def) => (
                <AttributeField
                  key={def.key}
                  def={def}
                  value={attributes[def.key] ?? ''}
                  onChange={(next) =>
                    setAttributes((current) => ({ ...current, [def.key]: next }))
                  }
                />
              ))}
            </fieldset>
          ) : null}

          <div className="flex items-center gap-3">
            <Button type="submit" disabled={saving}>
              {saving ? '保存中…' : '保存'}
            </Button>
            <a
              href={tenantURL('/account/password')}
              className="inline-flex items-center gap-1 text-sm font-medium text-slate-500 hover:text-slate-700"
            >
              <IconArrowLeft size={15} aria-hidden="true" />
              パスワード変更へ
            </a>
          </div>
        </form>
      </div>
    </AuthShell>
  )
}

function ReadField({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="mt-0.5 text-sm font-medium text-slate-900">{value}</dd>
    </div>
  )
}

function AttributeField({
  def,
  value,
  onChange,
}: {
  def: UserAttributeDef
  value: string
  onChange: (next: string) => void
}) {
  const id = `attr-${def.key}`
  if (def.type === 'boolean') {
    return (
      <label htmlFor={id} className="inline-flex items-center gap-2 text-sm text-slate-700">
        <input
          id={id}
          type="checkbox"
          checked={value === 'true'}
          onChange={(event) => onChange(event.target.checked ? 'true' : 'false')}
          className="h-4 w-4 rounded border-slate-300"
        />
        {def.claim_name ?? def.key}
      </label>
    )
  }
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{def.claim_name ?? def.key}</Label>
      <Input
        id={id}
        type={def.type === 'number' ? 'number' : def.type === 'date' ? 'date' : 'text'}
        value={value}
        placeholder={def.type === 'string_array' ? 'カンマ区切り' : undefined}
        onChange={(event) => onChange(event.target.value)}
      />
      {def.type === 'string_array' ? (
        <p className="text-xs text-slate-500">複数値はカンマ区切りで入力します。</p>
      ) : null}
    </div>
  )
}
