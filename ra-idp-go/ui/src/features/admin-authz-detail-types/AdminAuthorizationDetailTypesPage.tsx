import { IconPlus, IconShieldCheck, IconTrash, IconX } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import {
  AuthenticationAPIError,
  createAuthorizationDetailType,
  deleteAuthorizationDetailType,
  tenantURL,
  updateAuthorizationDetailType,
} from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type { AuthorizationDetailType } from '../../types'

type FormState = {
  type: string
  description: string
  displayTemplate: string
  state: AuthorizationDetailType['state']
  schemaJSON: string
}

const sampleSchema = `{
  "rules": [
    { "name": "actions", "semantics": "set", "required": true, "allowed": ["read", "write"] },
    { "name": "datatypes", "semantics": "set", "required": true }
  ]
}`

const emptyForm: FormState = {
  type: '',
  description: '',
  displayTemplate: '',
  state: 'Enabled',
  schemaJSON: sampleSchema,
}

function toForm(t: AuthorizationDetailType): FormState {
  return {
    type: t.type,
    description: t.description ?? '',
    displayTemplate: t.display_template,
    state: t.state,
    schemaJSON: JSON.stringify(t.schema, null, 2),
  }
}

export function AdminAuthorizationDetailTypesPage({
  csrfToken,
  actorUsername,
  types,
}: {
  csrfToken: string
  actorUsername?: string
  types: AuthorizationDetailType[]
}) {
  const [items, setItems] = useState(types)
  const [editing, setEditing] = useState<string | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  function reset() {
    setEditing(null)
    setCreating(false)
    setForm(emptyForm)
    setError('')
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError('')
    let schema: AuthorizationDetailType['schema']
    try {
      schema = JSON.parse(form.schemaJSON)
    } catch {
      setError('スキーマ JSON が不正です。')
      return
    }
    const input = {
      type: form.type,
      description: form.description,
      display_template: form.displayTemplate,
      state: form.state,
      schema,
    }
    try {
      if (editing) {
        const updated = await updateAuthorizationDetailType(csrfToken, editing, input)
        setItems((prev) => prev.map((t) => (t.type === editing ? updated : t)))
        setNotice(`${editing} を更新しました。`)
      } else {
        const created = await createAuthorizationDetailType(csrfToken, input)
        setItems((prev) => [...prev, created].sort((a, b) => a.type.localeCompare(b.type)))
        setNotice(`${created.type} を登録しました。`)
      }
      reset()
    } catch (cause) {
      setError(cause instanceof AuthenticationAPIError ? cause.message : '保存に失敗しました。')
    }
  }

  async function handleDelete(detailType: string) {
    setError('')
    try {
      await deleteAuthorizationDetailType(csrfToken, detailType)
      setItems((prev) => prev.filter((t) => t.type !== detailType))
      setNotice(`${detailType} を削除しました。`)
      if (editing === detailType) reset()
    } catch (cause) {
      setError(cause instanceof AuthenticationAPIError ? cause.message : '削除に失敗しました。')
    }
  }

  const showForm = creating || editing !== null

  return (
    <AdminShell
      active="authz-detail-types"
      actorUsername={actorUsername}
      title="認可詳細の種類"
      description="エージェントに渡す細粒度の認可詳細 (RFC 9396 authorization_details) の種類を定義します。受理するのはここに登録した種類のみです。"
      actions={
        showForm ? undefined : (
          <Button
            type="button"
            onClick={() => {
              setCreating(true)
              setForm(emptyForm)
            }}
          >
            <IconPlus size={17} aria-hidden="true" />
            種類を登録
          </Button>
        )
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      {showForm ? (
        <Card className="p-4">
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="type">種類 ID (type)</Label>
              <Input
                id="type"
                value={form.type}
                disabled={editing !== null}
                placeholder="payment_initiation"
                onChange={(e) => setForm({ ...form, type: e.target.value })}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="description">説明</Label>
              <Input
                id="description"
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="display_template">同意画面の表示テンプレート</Label>
              <Input
                id="display_template"
                value={form.displayTemplate}
                placeholder="口座 {creditorAccount} に対して {actions} を、最大 {instructedAmount} まで"
                onChange={(e) => setForm({ ...form, displayTemplate: e.target.value })}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="schema">検証スキーマ (JSON)</Label>
              <textarea
                id="schema"
                value={form.schemaJSON}
                onChange={(e) => setForm({ ...form, schemaJSON: e.target.value })}
                rows={10}
                spellCheck={false}
                className="rounded-md border border-slate-300 bg-white p-2.5 font-mono text-xs leading-5 text-slate-900 focus:border-blue-500 focus:outline-none"
              />
              <p className="text-xs leading-5 text-slate-500">
                各フィールドの semantics は set (集合包含) / at_most (上限) / enum / exact
                のいずれか。要求はここで束縛した範囲に限定され、同意・委譲・交換でこの半順序を超えられません。
              </p>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="state">状態</Label>
              <select
                id="state"
                value={form.state}
                onChange={(e) =>
                  setForm({ ...form, state: e.target.value as AuthorizationDetailType['state'] })
                }
                className="w-40 rounded-md border border-slate-300 bg-white p-2 text-sm text-slate-900 focus:border-blue-500 focus:outline-none"
              >
                <option value="Enabled">Enabled</option>
                <option value="Disabled">Disabled</option>
              </select>
            </div>
            <div className="flex gap-2.5">
              <Button type="submit">{editing ? '更新' : '登録'}</Button>
              <Button type="button" variant="ghost" onClick={reset}>
                <IconX size={17} aria-hidden="true" />
                キャンセル
              </Button>
            </div>
          </form>
        </Card>
      ) : null}

      {items.length === 0 ? (
        <Card className="p-8 text-center text-sm text-slate-500">
          まだ認可詳細の種類が登録されていません。
        </Card>
      ) : (
        <div className="flex flex-col gap-3">
          {items.map((t) => (
            <Card key={t.type} className="flex flex-col gap-3 p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="font-mono text-sm font-semibold text-slate-900">{t.type}</p>
                    <span
                      className={
                        t.state === 'Enabled'
                          ? 'rounded-full bg-emerald-50 px-2 py-0.5 text-[0.68rem] font-bold text-emerald-700'
                          : 'rounded-full bg-slate-100 px-2 py-0.5 text-[0.68rem] font-bold text-slate-500'
                      }
                    >
                      {t.state}
                    </span>
                  </div>
                  {t.description ? (
                    <p className="mt-0.5 text-xs leading-5 text-slate-500">{t.description}</p>
                  ) : null}
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setEditing(t.type)
                      setCreating(false)
                      setForm(toForm(t))
                    }}
                  >
                    編集
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => handleDelete(t.type)}>
                    <IconTrash size={16} aria-hidden="true" />
                  </Button>
                </div>
              </div>
              <div className="flex items-start gap-2 rounded-lg bg-slate-50 p-2.5 text-xs leading-5 text-slate-600">
                <IconShieldCheck
                  size={15}
                  className="mt-0.5 shrink-0 text-blue-600"
                  aria-hidden="true"
                />
                <div className="flex flex-wrap gap-1.5">
                  {t.schema.rules.map((rule) => (
                    <span key={rule.name} className="font-mono">
                      {rule.name}:{rule.semantics}
                      {rule.required ? '*' : ''}
                    </span>
                  ))}
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      <p className="text-xs text-slate-400">
        <a className="underline" href={tenantURL('/admin/applications')}>
          アプリケーション
        </a>{' '}
        が要求した詳細は、ここで定義した検証ルールで fail-closed に検査されます。
      </p>
    </AdminShell>
  )
}
