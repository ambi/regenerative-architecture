import { IconDownload, IconPlus, IconServerBolt, IconTrash, IconWorldShare, IconX } from '@tabler/icons-react'
import { type FormEvent, useState } from 'react'
import {
  AuthenticationAPIError,
  deleteWsFedRelyingParty,
  saveWsFedRelyingParty,
  tenantURL,
} from '../../api'
import { AdminShell } from '../../components/AdminShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import type {
  AdminWsFedRelyingPartiesPage as PageProps,
  WsFedClaimMappingRule,
  WsFedRelyingParty,
  WsFedTokenType,
} from '../../types'

const TOKEN_TYPE_SAML11: WsFedTokenType = 'urn:oasis:names:tc:SAML:1.0:assertion'
const TOKEN_TYPE_SAML20: WsFedTokenType = 'urn:oasis:names:tc:SAML:2.0:assertion'

type FormState = {
  wtrealm: string
  displayName: string
  replyURLs: string
  audience: string
  tokenType: WsFedTokenType
  nameIDFormat: string
  nameIDSource: string
  rulesJSON: string
}

const sampleRules = `[
  { "claim_type": "http://schemas.xmlsoap.org/claims/UPN", "source": "user_attribute", "source_key": "preferred_username", "required": true },
  { "claim_type": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", "source": "user_attribute", "source_key": "email" }
]`

const emptyForm: FormState = {
  wtrealm: '',
  displayName: '',
  replyURLs: '',
  audience: '',
  tokenType: TOKEN_TYPE_SAML11,
  nameIDFormat: 'urn:oasis:names:tc:SAML:2.0:nameid-format:persistent',
  nameIDSource: 'sub',
  rulesJSON: sampleRules,
}

function toForm(rp: WsFedRelyingParty): FormState {
  return {
    wtrealm: rp.wtrealm,
    displayName: rp.display_name ?? '',
    replyURLs: rp.reply_urls.join('\n'),
    audience: rp.audience ?? '',
    tokenType: rp.token_type ?? TOKEN_TYPE_SAML11,
    nameIDFormat: rp.claim_policy.name_id.format,
    nameIDSource: rp.claim_policy.name_id.source_attribute,
    rulesJSON: JSON.stringify(rp.claim_policy.rules ?? [], null, 2),
  }
}

export function AdminWsFedRelyingPartiesPage({
  csrfToken,
  actorUsername,
  relyingParties,
}: PageProps) {
  const [items, setItems] = useState(relyingParties)
  const [editing, setEditing] = useState<string | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const endpointLinks = [
    {
      label: 'Federation metadata',
      href: tenantURL('/federationmetadata/2007-06/federationmetadata.xml'),
      value: '/federationmetadata/2007-06/federationmetadata.xml',
      icon: IconDownload,
    },
    {
      label: 'WS-Trust MEX',
      href: tenantURL('/trust/mex'),
      value: '/trust/mex',
      icon: IconServerBolt,
    },
    {
      label: 'WS-Trust usernamemixed',
      href: tenantURL('/trust/usernamemixed'),
      value: '/trust/usernamemixed',
      icon: IconServerBolt,
    },
  ]

  function reset() {
    setEditing(null)
    setCreating(false)
    setForm(emptyForm)
    setError('')
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError('')
    let rules: WsFedClaimMappingRule[]
    try {
      rules = JSON.parse(form.rulesJSON || '[]')
      if (!Array.isArray(rules)) throw new Error('not an array')
    } catch {
      setError('claim ルールの JSON が不正です。配列で指定してください。')
      return
    }
    const replyURLs = form.replyURLs
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line !== '')
    if (replyURLs.length === 0) {
      setError('返信 URL (wreply) を 1 つ以上指定してください。')
      return
    }
    const input = {
      wtrealm: form.wtrealm.trim(),
      display_name: form.displayName.trim() || undefined,
      reply_urls: replyURLs,
      audience: form.audience.trim() || undefined,
      token_type: form.tokenType,
      claim_policy: {
        name_id: {
          format: form.nameIDFormat.trim(),
          source_attribute: form.nameIDSource.trim(),
        },
        rules,
      },
    }
    try {
      const saved = await saveWsFedRelyingParty(csrfToken, input)
      setItems((prev) => {
        const others = prev.filter((rp) => rp.wtrealm !== saved.wtrealm)
        return [...others, saved].sort((a, b) => a.wtrealm.localeCompare(b.wtrealm))
      })
      setNotice(`${saved.wtrealm} を保存しました。`)
      reset()
    } catch (cause) {
      setError(cause instanceof AuthenticationAPIError ? cause.message : '保存に失敗しました。')
    }
  }

  async function handleDelete(wtrealm: string) {
    setError('')
    try {
      await deleteWsFedRelyingParty(csrfToken, wtrealm)
      setItems((prev) => prev.filter((rp) => rp.wtrealm !== wtrealm))
      setNotice(`${wtrealm} を削除しました。`)
      if (editing === wtrealm) reset()
    } catch (cause) {
      setError(cause instanceof AuthenticationAPIError ? cause.message : '削除に失敗しました。')
    }
  }

  const showForm = creating || editing !== null

  return (
    <AdminShell
      active="wsfed"
      actorUsername={actorUsername}
      title="WS-Federation 連携先"
      description="WS-Federation passive requestor で SSO する relying party を登録します。許可した返信 URL (wreply) の閉集合と、発行する SAML assertion の claim mapping をここで束縛します。"
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
            連携先を登録
          </Button>
        )
      }
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}
      {notice ? <Alert variant="success">{notice}</Alert> : null}

      <div className="grid gap-2 rounded-md border border-slate-200 bg-slate-50 p-3 sm:grid-cols-3">
        {endpointLinks.map(({ label, href, value, icon: Icon }) => (
          <a
            key={value}
            className="flex min-w-0 items-start gap-2 rounded-md border border-slate-200 bg-white p-2.5 text-xs text-slate-600 hover:border-blue-300 hover:text-blue-700"
            href={href}
          >
            <Icon size={16} className="mt-0.5 shrink-0 text-blue-600" aria-hidden="true" />
            <span className="min-w-0">
              <span className="block font-semibold text-slate-800">{label}</span>
              <span className="block truncate font-mono">{value}</span>
            </span>
          </a>
        ))}
      </div>

      {showForm ? (
        <Card className="p-4">
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="wtrealm">識別子 (wtrealm)</Label>
              <Input
                id="wtrealm"
                value={form.wtrealm}
                disabled={editing !== null}
                placeholder="urn:example:relying-party"
                onChange={(e) => setForm({ ...form, wtrealm: e.target.value })}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="display_name">表示名</Label>
              <Input
                id="display_name"
                value={form.displayName}
                onChange={(e) => setForm({ ...form, displayName: e.target.value })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="reply_urls">返信 URL (wreply、1 行に 1 つ)</Label>
              <textarea
                id="reply_urls"
                value={form.replyURLs}
                onChange={(e) => setForm({ ...form, replyURLs: e.target.value })}
                rows={3}
                spellCheck={false}
                placeholder={'https://rp.example/wsfed'}
                className="rounded-md border border-slate-300 bg-white p-2.5 font-mono text-xs leading-5 text-slate-900 focus:border-blue-500 focus:outline-none"
              />
              <p className="text-xs leading-5 text-slate-500">
                sign-in 後の auto-POST 先と sign-out の戻り先は、ここに登録した URL に限定されます
                (open redirect 防止)。
              </p>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="audience">Audience (任意)</Label>
              <Input
                id="audience"
                value={form.audience}
                placeholder="未指定なら wtrealm を使用します"
                onChange={(e) => setForm({ ...form, audience: e.target.value })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="token_type">トークン種別 (SAML バージョン)</Label>
              <select
                id="token_type"
                value={form.tokenType}
                onChange={(e) => setForm({ ...form, tokenType: e.target.value as WsFedTokenType })}
                className="w-72 rounded-md border border-slate-300 bg-white p-2 text-sm text-slate-900 focus:border-blue-500 focus:outline-none"
              >
                <option value={TOKEN_TYPE_SAML11}>SAML 1.1 (Entra / AD FS 既定)</option>
                <option value={TOKEN_TYPE_SAML20}>SAML 2.0</option>
              </select>
              <p className="text-xs leading-5 text-slate-500">
                Entra domain federation / Microsoft 365 連携は SAML 1.1 が既定です。
              </p>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="name_id_format">NameID フォーマット</Label>
                <Input
                  id="name_id_format"
                  value={form.nameIDFormat}
                  onChange={(e) => setForm({ ...form, nameIDFormat: e.target.value })}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="name_id_source">NameID 供給元属性</Label>
                <Input
                  id="name_id_source"
                  value={form.nameIDSource}
                  placeholder="sub"
                  onChange={(e) => setForm({ ...form, nameIDSource: e.target.value })}
                  required
                />
              </div>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="rules">claim mapping ルール (JSON)</Label>
              <textarea
                id="rules"
                value={form.rulesJSON}
                onChange={(e) => setForm({ ...form, rulesJSON: e.target.value })}
                rows={10}
                spellCheck={false}
                className="rounded-md border border-slate-300 bg-white p-2.5 font-mono text-xs leading-5 text-slate-900 focus:border-blue-500 focus:outline-none"
              />
              <p className="text-xs leading-5 text-slate-500">
                各ルールの source は user_attribute (属性) / fixed (固定値) / nameid
                のいずれか。required:true の claim は値が解決できないと sign-in を fail-closed
                で拒否します。
              </p>
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
          まだ WS-Federation 連携先が登録されていません。
        </Card>
      ) : (
        <div className="flex flex-col gap-3">
          {items.map((rp) => (
            <Card key={rp.wtrealm} className="flex flex-col gap-3 p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="truncate font-mono text-sm font-semibold text-slate-900">
                      {rp.wtrealm}
                    </p>
                  </div>
                  {rp.display_name ? (
                    <p className="mt-0.5 text-xs leading-5 text-slate-500">{rp.display_name}</p>
                  ) : null}
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setEditing(rp.wtrealm)
                      setCreating(false)
                      setForm(toForm(rp))
                    }}
                  >
                    編集
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => handleDelete(rp.wtrealm)}>
                    <IconTrash size={16} aria-hidden="true" />
                  </Button>
                </div>
              </div>
              <div className="flex items-start gap-2 rounded-lg bg-slate-50 p-2.5 text-xs leading-5 text-slate-600">
                <IconWorldShare
                  size={15}
                  className="mt-0.5 shrink-0 text-blue-600"
                  aria-hidden="true"
                />
                <div className="flex flex-col gap-1">
                  <span>
                    <span className="font-semibold text-slate-700">wreply:</span>{' '}
                    {rp.reply_urls.join(', ')}
                  </span>
                  <span>
                    <span className="font-semibold text-slate-700">NameID:</span>{' '}
                    {rp.claim_policy.name_id.source_attribute}
                  </span>
                  <span>
                    <span className="font-semibold text-slate-700">トークン:</span>{' '}
                    {(rp.token_type ?? TOKEN_TYPE_SAML11) === TOKEN_TYPE_SAML20
                      ? 'SAML 2.0'
                      : 'SAML 1.1'}
                  </span>
                  <div className="flex flex-wrap gap-1.5">
                    {(rp.claim_policy.rules ?? []).map((rule) => (
                      <span key={rule.claim_type} className="font-mono">
                        {rule.claim_type.split('/').pop()}
                        {rule.required ? '*' : ''}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      <p className="text-xs text-slate-400">
        WS-Federation passive sign-in は{' '}
        <a className="underline" href={tenantURL('/wsfed?wa=wsignin1.0')}>
          /wsfed
        </a>{' '}
        で処理します。
      </p>
    </AdminShell>
  )
}
