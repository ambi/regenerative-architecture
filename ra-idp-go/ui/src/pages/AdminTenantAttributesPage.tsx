import { IconPlus, IconTrash } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, updateTenantUserAttributeSchema } from '../api'
import { AdminShell } from '../components/AdminShell'
import { Alert } from '../components/ui/alert'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import type {
  AdminTenantAttributesPage as PageProps,
  AttributeType,
  AttrVisibility,
  UserAttributeDef,
} from '../types'

const ATTRIBUTE_TYPES: AttributeType[] = ['string', 'number', 'boolean', 'date', 'string_array']
const VISIBILITIES: AttrVisibility[] = [
  'private',
  'self_readable',
  'admin_readable',
  'claim_exposed',
]

const VISIBILITY_LABEL: Record<AttrVisibility, string> = {
  private: '非公開',
  self_readable: '本人のみ参照',
  admin_readable: '管理者のみ参照',
  claim_exposed: 'claim として開示',
}

function newAttribute(): UserAttributeDef {
  return {
    key: '',
    type: 'string',
    multi_valued: false,
    required: false,
    editable_by_user: false,
    visibility: 'admin_readable',
    pii: true,
  }
}

export function AdminTenantAttributesPage({ csrfToken, actorUsername, schema }: PageProps) {
  const [attributes, setAttributes] = useState<UserAttributeDef[]>(schema.attributes)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  function patch(index: number, change: Partial<UserAttributeDef>) {
    setAttributes((current) =>
      current.map((def, i) => (i === index ? { ...def, ...change } : def)),
    )
  }

  function addAttribute() {
    setAttributes((current) => [...current, newAttribute()])
  }

  function removeAttribute(index: number) {
    setAttributes((current) => current.filter((_, i) => i !== index))
  }

  async function handleSave() {
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const cleaned = attributes.map((def) => ({
        ...def,
        key: def.key.trim(),
        multi_valued: def.type === 'string_array',
        claim_name: def.claim_name?.trim() || undefined,
        oidc_scope: def.oidc_scope?.trim() || undefined,
      }))
      const next = await updateTenantUserAttributeSchema(csrfToken, cleaned)
      setAttributes(next.attributes)
      setNotice('属性スキーマを保存しました。')
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '属性スキーマを保存できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <AdminShell
      active="tenant-attributes"
      actorUsername={actorUsername}
      title="ユーザー属性"
      description="このテナント固有の custom 属性を定義します。組み込み属性はコードが提供します。"
    >
      <div className="grid gap-6">
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {notice ? <Alert variant="success">{notice}</Alert> : null}

        <Card className="p-6">
          <header className="flex items-center justify-between gap-4">
            <div>
              <h2 className="text-base font-semibold text-slate-900">custom 属性</h2>
              <p className="mt-1 text-sm text-slate-600">
                key は snake_case (英字始まり)。組み込み属性と同じ key は使えません。
                変更は「保存」で全置換されます。
              </p>
            </div>
            <Button type="button" variant="outline" onClick={addAttribute}>
              <IconPlus size={16} stroke={1.8} aria-hidden="true" />
              <span className="ml-1">属性を追加</span>
            </Button>
          </header>

          {attributes.length === 0 ? (
            <p className="mt-6 rounded-md border border-dashed border-slate-200 px-4 py-8 text-center text-sm text-slate-500">
              custom 属性はまだありません。「属性を追加」で定義できます。
            </p>
          ) : (
            <ul className="mt-5 grid gap-4">
              {attributes.map((def, index) => (
                <li
                  // biome-ignore lint/suspicious/noArrayIndexKey: 行は順序で識別する編集フォーム
                  key={index}
                  className="grid gap-4 rounded-lg border border-slate-200 p-4"
                >
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-1.5">
                      <Label htmlFor={`key-${index}`}>key</Label>
                      <Input
                        id={`key-${index}`}
                        value={def.key}
                        placeholder="region"
                        className="font-mono"
                        onChange={(event) => patch(index, { key: event.target.value })}
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor={`type-${index}`}>type</Label>
                      <select
                        id={`type-${index}`}
                        value={def.type}
                        onChange={(event) =>
                          patch(index, { type: event.target.value as AttributeType })
                        }
                        className="h-9 rounded-md border border-slate-300 bg-white px-3 text-sm"
                      >
                        {ATTRIBUTE_TYPES.map((type) => (
                          <option key={type} value={type}>
                            {type}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor={`visibility-${index}`}>可視性</Label>
                      <select
                        id={`visibility-${index}`}
                        value={def.visibility}
                        onChange={(event) =>
                          patch(index, { visibility: event.target.value as AttrVisibility })
                        }
                        className="h-9 rounded-md border border-slate-300 bg-white px-3 text-sm"
                      >
                        {VISIBILITIES.map((v) => (
                          <option key={v} value={v}>
                            {VISIBILITY_LABEL[v]}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor={`claim-${index}`}>claim 名 (任意)</Label>
                      <Input
                        id={`claim-${index}`}
                        value={def.claim_name ?? ''}
                        placeholder="claim_exposed の場合のみ"
                        className="font-mono"
                        onChange={(event) => patch(index, { claim_name: event.target.value })}
                      />
                    </div>
                    <div className="grid gap-1.5">
                      <Label htmlFor={`scope-${index}`}>OIDC scope (任意)</Label>
                      <Input
                        id={`scope-${index}`}
                        value={def.oidc_scope ?? ''}
                        placeholder="未指定なら custom_attribute"
                        className="font-mono"
                        onChange={(event) => patch(index, { oidc_scope: event.target.value })}
                      />
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
                    <Toggle
                      id={`required-${index}`}
                      label="必須"
                      checked={def.required}
                      onChange={(checked) => patch(index, { required: checked })}
                    />
                    <Toggle
                      id={`editable-${index}`}
                      label="本人が編集可"
                      checked={def.editable_by_user}
                      onChange={(checked) => patch(index, { editable_by_user: checked })}
                    />
                    <Toggle
                      id={`pii-${index}`}
                      label="PII (監査で hash 化)"
                      checked={def.pii}
                      onChange={(checked) => patch(index, { pii: checked })}
                    />
                    <button
                      type="button"
                      onClick={() => removeAttribute(index)}
                      className="ml-auto inline-flex items-center gap-1 text-sm font-medium text-red-600 hover:text-red-700"
                    >
                      <IconTrash size={16} stroke={1.8} aria-hidden="true" />
                      削除
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}

          <div className="mt-6">
            <Button type="button" onClick={handleSave} disabled={saving}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </div>
        </Card>

        <BuiltinReference builtin={schema.builtin} />
      </div>
    </AdminShell>
  )
}

function Toggle({
  id,
  label,
  checked,
  onChange,
}: {
  id: string
  label: string
  checked: boolean
  onChange: (next: boolean) => void
}) {
  return (
    <label htmlFor={id} className="inline-flex items-center gap-2 text-sm text-slate-700">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="h-4 w-4 rounded border-slate-300"
      />
      {label}
    </label>
  )
}

function BuiltinReference({ builtin }: { builtin: UserAttributeDef[] }) {
  return (
    <Card className="p-6">
      <h2 className="text-base font-semibold text-slate-900">組み込み属性 (参照)</h2>
      <p className="mt-1 text-sm text-slate-600">
        OIDC §5.1 / SCIM 由来の読み取り専用カタログ。これらと同じ key は custom で定義できません。
      </p>
      <div className="mt-4 overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4 font-medium">key</th>
              <th className="py-2 pr-4 font-medium">type</th>
              <th className="py-2 pr-4 font-medium">可視性</th>
              <th className="py-2 pr-4 font-medium">scope</th>
            </tr>
          </thead>
          <tbody>
            {builtin.map((def) => (
              <tr key={def.key} className="border-b border-slate-100">
                <td className="py-2 pr-4 font-mono text-slate-800">{def.key}</td>
                <td className="py-2 pr-4 text-slate-600">{def.type}</td>
                <td className="py-2 pr-4 text-slate-600">{VISIBILITY_LABEL[def.visibility]}</td>
                <td className="py-2 pr-4 font-mono text-slate-500">{def.oidc_scope ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
