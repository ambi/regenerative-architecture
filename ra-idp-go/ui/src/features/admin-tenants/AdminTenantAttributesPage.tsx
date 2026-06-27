import { IconPencil, IconPlus, IconTrash, IconX } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, updateTenantUserAttributeSchema } from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type {
  AttributeType,
  AttrVisibility,
  UserAttributeDef,
  TenantUserAttributeSchema,
} from '../../types'

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
    label: '',
    type: 'string',
    multi_valued: false,
    required: false,
    editable_by_user: false,
    visibility: 'admin_readable',
    pii: true,
  }
}

// editing は追加 (index === null) か既存行の編集 (index >= 0)。
type EditingState = { index: number | null; draft: UserAttributeDef } | null

export function AdminTenantAttributesPage({
  csrfToken,
  actorUsername,
  schema,
}: {
  csrfToken: string
  actorUsername?: string
  schema: TenantUserAttributeSchema
}) {
  const [attributes, setAttributes] = useState<UserAttributeDef[]>(schema.attributes)
  const [editing, setEditing] = useState<EditingState>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  // persist は custom 定義一覧を全置換で保存し、成功したらサーバ正規化後の値で更新する。
  async function persist(next: UserAttributeDef[], success: string) {
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const result = await updateTenantUserAttributeSchema(csrfToken, next)
      setAttributes(result.attributes)
      setNotice(success)
      return true
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : '属性スキーマを保存できませんでした。',
      )
      return false
    } finally {
      setSaving(false)
    }
  }

  async function handleSubmit(draft: UserAttributeDef, index: number | null) {
    const cleaned: UserAttributeDef = {
      ...draft,
      key: draft.key.trim(),
      label: draft.label?.trim() || undefined,
      multi_valued: draft.type === 'string_array',
      claim_name: draft.claim_name?.trim() || undefined,
      oidc_scope: draft.oidc_scope?.trim() || undefined,
    }
    const next =
      index === null
        ? [...attributes, cleaned]
        : attributes.map((def, i) => (i === index ? cleaned : def))
    const ok = await persist(next, index === null ? '属性を追加しました。' : '属性を更新しました。')
    if (ok) setEditing(null)
  }

  async function handleDelete(index: number) {
    await persist(
      attributes.filter((_, i) => i !== index),
      '属性を削除しました。',
    )
  }

  return (
    <AdminShell
      active="tenant-attributes"
      actorUsername={actorUsername}
      title="ユーザー属性"
      description="このテナント固有のカスタム属性を定義します。組み込み属性はコードが提供します。"
      actions={
        <Button type="button" onClick={() => setEditing({ index: null, draft: newAttribute() })}>
          <IconPlus size={16} stroke={1.8} aria-hidden="true" />
          <span className="ml-1">属性を追加</span>
        </Button>
      }
    >
      <div className="grid gap-6">
        {error ? <Alert variant="destructive">{error}</Alert> : null}
        {notice ? <Alert variant="success">{notice}</Alert> : null}

        <Card className="overflow-hidden">
          <div className="border-b border-slate-200 p-5">
            <h2 className="text-base font-semibold text-slate-900">カスタム属性</h2>
            <p className="mt-1 text-sm text-slate-600">
              key は snake_case (英字始まり)。組み込み属性と同じ key は使えません。
            </p>
          </div>
          {attributes.length === 0 ? (
            <p className="px-5 py-10 text-center text-sm text-slate-500">
              カスタム属性はまだありません。「属性を追加」で定義できます。
            </p>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-5 py-3">属性</th>
                  <th className="px-5 py-3">型</th>
                  <th className="px-5 py-3">可視性</th>
                  <th className="px-5 py-3">本人編集</th>
                  <th className="px-5 py-3" />
                </tr>
              </thead>
              <tbody>
                {attributes.map((def, index) => (
                  <tr key={def.key} className="border-t border-slate-100">
                    <td className="px-5 py-3">
                      <div className="text-slate-800">{def.label || def.key}</div>
                      {def.label ? (
                        <div className="font-mono text-xs text-slate-500">{def.key}</div>
                      ) : null}
                    </td>
                    <td className="px-5 py-3 text-slate-600">{def.type}</td>
                    <td className="px-5 py-3 text-slate-600">{VISIBILITY_LABEL[def.visibility]}</td>
                    <td className="px-5 py-3 text-slate-600">
                      {def.editable_by_user ? '可' : '不可'}
                    </td>
                    <td className="px-5 py-3">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          className="px-2.5"
                          aria-label={`${def.key} を編集`}
                          disabled={saving}
                          onClick={() => setEditing({ index, draft: def })}
                        >
                          <IconPencil size={15} aria-hidden="true" />
                        </Button>
                        <Button
                          variant="ghost"
                          className="px-2.5 text-rose-700 hover:bg-rose-50"
                          aria-label={`${def.key} を削除`}
                          disabled={saving}
                          onClick={() => void handleDelete(index)}
                        >
                          <IconTrash size={15} aria-hidden="true" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>

        <BuiltinReference builtin={schema.builtin} />
      </div>

      {editing ? (
        <AttributeEditorDialog
          initial={editing.draft}
          isNew={editing.index === null}
          saving={saving}
          onClose={() => setEditing(null)}
          onSubmit={(draft) => void handleSubmit(draft, editing.index)}
        />
      ) : null}
    </AdminShell>
  )
}

function AttributeEditorDialog({
  initial,
  isNew,
  saving,
  onClose,
  onSubmit,
}: {
  initial: UserAttributeDef
  isNew: boolean
  saving: boolean
  onClose: () => void
  onSubmit: (draft: UserAttributeDef) => void
}) {
  const [draft, setDraft] = useState<UserAttributeDef>(initial)
  const keyInvalid = draft.key.trim() === ''

  function patch(change: Partial<UserAttributeDef>) {
    setDraft((current) => ({ ...current, ...change }))
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/30 p-5 backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="attribute-editor-title"
    >
      <button
        type="button"
        className="absolute inset-0 cursor-default"
        aria-label="閉じる"
        onClick={onClose}
      />
      <Card className="relative flex max-h-[88vh] w-full max-w-lg flex-col overflow-hidden shadow-2xl">
        <div className="flex items-start justify-between border-b border-slate-200 px-6 py-5">
          <h2 id="attribute-editor-title" className="text-xl font-semibold">
            {isNew ? '属性を追加' : '属性を編集'}
          </h2>
          <Button variant="ghost" className="px-2.5" onClick={onClose} aria-label="閉じる">
            <IconX size={18} aria-hidden="true" />
          </Button>
        </div>
        <form
          onSubmit={(event) => {
            event.preventDefault()
            if (!keyInvalid) onSubmit(draft)
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <div className="min-h-0 flex-1 overflow-y-auto p-6">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-1.5 sm:col-span-2">
                <Label htmlFor="attr-label">表示名</Label>
                <Input
                  id="attr-label"
                  value={draft.label ?? ''}
                  placeholder="例: 部署"
                  onChange={(event) => patch({ label: event.target.value })}
                />
                <p className="text-xs text-slate-500">
                  利用者・管理者に見せる日本語名。未設定なら key を表示します。
                </p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="attr-key">key</Label>
                <Input
                  id="attr-key"
                  value={draft.key}
                  placeholder="region"
                  className="font-mono"
                  aria-invalid={keyInvalid}
                  onChange={(event) => patch({ key: event.target.value })}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="attr-type">型</Label>
                <select
                  id="attr-type"
                  value={draft.type}
                  onChange={(event) => patch({ type: event.target.value as AttributeType })}
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
                <Label htmlFor="attr-visibility">可視性</Label>
                <select
                  id="attr-visibility"
                  value={draft.visibility}
                  onChange={(event) => patch({ visibility: event.target.value as AttrVisibility })}
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
                <Label htmlFor="attr-claim">claim 名 (任意)</Label>
                <Input
                  id="attr-claim"
                  value={draft.claim_name ?? ''}
                  placeholder="claim として開示する場合のみ"
                  className="font-mono"
                  onChange={(event) => patch({ claim_name: event.target.value })}
                />
              </div>
              <div className="grid gap-1.5 sm:col-span-2">
                <Label htmlFor="attr-scope">OIDC scope (任意)</Label>
                <Input
                  id="attr-scope"
                  value={draft.oidc_scope ?? ''}
                  placeholder="未指定なら custom_attribute"
                  className="font-mono"
                  onChange={(event) => patch({ oidc_scope: event.target.value })}
                />
              </div>
            </div>
            <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-slate-100 pt-5">
              <Toggle
                id="attr-required"
                label="必須"
                checked={draft.required}
                onChange={(checked) => patch({ required: checked })}
              />
              <Toggle
                id="attr-editable"
                label="本人が編集可"
                checked={draft.editable_by_user}
                onChange={(checked) => patch({ editable_by_user: checked })}
              />
              <Toggle
                id="attr-pii"
                label="PII (監査で hash 化)"
                checked={draft.pii}
                onChange={(checked) => patch({ pii: checked })}
              />
            </div>
          </div>
          <div className="flex justify-end gap-2 border-t border-slate-200 bg-slate-50 px-6 py-4">
            <Button type="button" variant="outline" onClick={onClose}>
              キャンセル
            </Button>
            <Button type="submit" disabled={saving || keyInvalid}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </div>
        </form>
      </Card>
    </div>
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
        OIDC §5.1 / SCIM 由来の読み取り専用カタログ。これらと同じ key はカスタムで定義できません。
      </p>
      <div className="mt-4 overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4 font-medium">表示名</th>
              <th className="py-2 pr-4 font-medium">key</th>
              <th className="py-2 pr-4 font-medium">型</th>
              <th className="py-2 pr-4 font-medium">可視性</th>
              <th className="py-2 pr-4 font-medium">scope</th>
            </tr>
          </thead>
          <tbody>
            {builtin.map((def) => (
              <tr key={def.key} className="border-b border-slate-100">
                <td className="py-2 pr-4 text-slate-800">{def.label || '—'}</td>
                <td className="py-2 pr-4 font-mono text-slate-600">{def.key}</td>
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
