import { type FormEvent, useState } from 'react'
import { AuthenticationAPIError, updateAccountProfile } from '../../api'
import { attributeGroupKey, attributeGroupTitle, attributeLabel } from '../../lib/utils'
import { AccountShell } from '../../components/AccountShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type { AccountProfile, AttributeValue, UserAttributeDef } from '../../types'

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

export function AccountProfilePage({
  csrfToken,
  profile: initial,
  isAdmin,
}: {
  csrfToken: string
  profile: AccountProfile
  isAdmin: boolean
}) {
  const [profile, setProfile] = useState(initial)
  const [name, setName] = useState(initial.name ?? '')
  const [givenName, setGivenName] = useState(initial.given_name ?? '')
  const [familyName, setFamilyName] = useState(initial.family_name ?? '')
  const [attributes, setAttributes] = useState<AttributeDraft>(draftFromProfile(initial))
  const [saving, setSaving] = useState(false)
  const [editing, setEditing] = useState(false)
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
      setName(next.name ?? '')
      setGivenName(next.given_name ?? '')
      setFamilyName(next.family_name ?? '')
      setAttributes(draftFromProfile(next))
      setEditing(false)
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
    <AccountShell
      active="profile"
      username={profile.preferred_username}
      isAdmin={isAdmin}
      title="アカウント情報"
      description="登録されているプロフィール情報を確認できます。"
    >
      <div className="grid gap-6">
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {notice ? <Alert variant="success">{notice}</Alert> : null}

        <Card className="p-5">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h2 className="text-base font-semibold text-slate-900">プロフィール</h2>
              <p className="mt-1 text-sm text-slate-600">
                変更が必要な場合だけ編集モードに切り替えてください。
              </p>
            </div>
            {!editing ? (
              <Button type="button" variant="outline" onClick={() => setEditing(true)}>
                編集
              </Button>
            ) : null}
          </div>

          {!editing ? (
            <>
              <dl className="mt-5 grid gap-3 sm:grid-cols-2">
                <ReadField label="表示名" value={profile.name ?? '未設定'} />
                <ReadField label="名" value={profile.given_name ?? '未設定'} />
                <ReadField label="姓" value={profile.family_name ?? '未設定'} />
                <ReadField label="メール" value={profile.email ?? '未設定'} />
                <ReadField
                  label="メール確認"
                  value={profile.email_verified ? '確認済み' : '未確認'}
                />
                <ReadField label="MFA" value={profile.mfa_enrolled ? '登録済み' : '未登録'} />
                <ReadField label="状態" value={profile.status} />
              </dl>
              <div className="mt-5 grid gap-4">
                <ProfileAttributeGroups
                  defs={profile.readable_attributes}
                  values={profile.attributes}
                />
              </div>
            </>
          ) : (
            <form onSubmit={handleSave} className="mt-5 grid gap-4">
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

              <EditableAttributeGroups
                defs={profile.editable_attributes}
                values={attributes}
                onChange={(key, next) => setAttributes((current) => ({ ...current, [key]: next }))}
              />

              <div className="flex items-center gap-2">
                <Button type="submit" disabled={saving}>
                  {saving ? '保存中…' : '保存'}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  disabled={saving}
                  onClick={() => {
                    setName(profile.name ?? '')
                    setGivenName(profile.given_name ?? '')
                    setFamilyName(profile.family_name ?? '')
                    setAttributes(draftFromProfile(profile))
                    setEditing(false)
                  }}
                >
                  キャンセル
                </Button>
              </div>
            </form>
          )}
        </Card>
      </div>
    </AccountShell>
  )
}

function ReadField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200/80 bg-white/70 px-3 py-2.5">
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="mt-0.5 text-sm font-medium text-slate-900">{value}</dd>
    </div>
  )
}

function groupedAttributes(defs: UserAttributeDef[]) {
  const groups = new Map<ReturnType<typeof attributeGroupKey>, UserAttributeDef[]>()
  for (const def of defs) {
    const key = attributeGroupKey(def)
    groups.set(key, [...(groups.get(key) ?? []), def])
  }
  return (['profile', 'organization', 'custom'] as const)
    .map((key) => ({ key, title: attributeGroupTitle(key), defs: groups.get(key) ?? [] }))
    .filter((group) => group.defs.length > 0)
}

function ProfileAttributeGroups({
  defs,
  values,
}: {
  defs: UserAttributeDef[]
  values: AccountProfile['attributes']
}) {
  const knownKeys = new Set(defs.map((def) => def.key))
  const readOnlyDefs: UserAttributeDef[] = Object.entries(values)
    .filter(([key]) => !knownKeys.has(key))
    .map(([key, value]) => ({
      key,
      type: value.type,
      multi_valued: value.type === 'string_array',
      required: false,
      editable_by_user: false,
      visibility: 'self_readable',
      pii: false,
    }))
  const groups = groupedAttributes([...defs, ...readOnlyDefs])
  if (groups.length === 0) return null
  return (
    <>
      {groups.map((group) => (
        <section key={group.key} className="grid gap-2">
          <h3 className="text-xs font-bold uppercase tracking-normal text-slate-500">
            {group.title}
          </h3>
          <dl className="grid gap-3 sm:grid-cols-2">
            {group.defs.map((def) => (
              <ReadField
                key={def.key}
                label={attributeLabel(def)}
                value={values[def.key] ? valueToDisplayText(values[def.key]) : '未設定'}
              />
            ))}
          </dl>
        </section>
      ))}
    </>
  )
}

function EditableAttributeGroups({
  defs,
  values,
  onChange,
}: {
  defs: UserAttributeDef[]
  values: AttributeDraft
  onChange: (key: string, next: string) => void
}) {
  const groups = groupedAttributes(defs)
  if (groups.length === 0) return null
  return (
    <div className="grid gap-4 rounded-lg border border-slate-200 p-4">
      <p className="text-sm font-medium text-slate-700">追加項目</p>
      {groups.map((group) => (
        <fieldset
          key={group.key}
          className="grid gap-3 border-t border-slate-100 pt-4 first:border-t-0 first:pt-0"
        >
          <legend className="text-xs font-bold uppercase tracking-normal text-slate-500">
            {group.title}
          </legend>
          {group.defs.map((def) => (
            <AttributeField
              key={def.key}
              def={def}
              value={values[def.key] ?? ''}
              onChange={(next) => onChange(def.key, next)}
            />
          ))}
        </fieldset>
      ))}
    </div>
  )
}

function valueToDisplayText(value: AttributeValue): string {
  const text = valueToText(value)
  if (value.type === 'boolean') return text === 'true' ? 'はい' : 'いいえ'
  return text || '未設定'
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
        {attributeLabel(def)}
      </label>
    )
  }
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{attributeLabel(def)}</Label>
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
