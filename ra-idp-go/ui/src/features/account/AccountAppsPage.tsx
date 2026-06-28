import { IconArrowDown, IconArrowUp, IconExternalLink, IconLayoutGrid } from '@tabler/icons-react'
import { useState } from 'react'
import { reorderMyApplications } from '../../api/account'
import { AuthenticationAPIError } from '../../api/core'
import { AccountShell } from '../../components/AccountShell'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import type { MyApplication } from '../../types'

function initials(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || '??'
}

function AppIcon({ app }: { app: MyApplication }) {
  if (app.icon_url) {
    return (
      <img
        src={app.icon_url}
        alt=""
        className="size-12 rounded-xl object-cover"
        aria-hidden="true"
      />
    )
  }
  return (
    <span className="flex size-12 items-center justify-center rounded-xl border border-blue-100 bg-blue-50 text-sm font-bold text-blue-700">
      {initials(app.name)}
    </span>
  )
}

function AppTile({ app }: { app: MyApplication }) {
  const launchable = Boolean(app.launch_url)
  const body = (
    <Card
      className={`flex h-full flex-col items-center gap-3 p-5 text-center transition ${
        launchable ? 'hover:border-blue-300 hover:shadow-md' : 'opacity-70'
      }`}
    >
      <AppIcon app={app} />
      <span className="flex items-center gap-1 text-sm font-semibold text-slate-900">
        {app.name}
        {launchable ? (
          <IconExternalLink size={14} className="text-slate-400" aria-hidden="true" />
        ) : null}
      </span>
      {launchable ? null : <span className="text-xs text-slate-400">起動 URL が未設定です</span>}
    </Card>
  )
  if (app.launch_url) {
    return (
      <a
        href={app.launch_url}
        target="_blank"
        rel="noreferrer"
        className="block focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
      >
        {body}
      </a>
    )
  }
  return body
}

function ReorderRow({
  app,
  index,
  total,
  onMove,
}: {
  app: MyApplication
  index: number
  total: number
  onMove: (index: number, delta: number) => void
}) {
  return (
    <Card className="flex items-center gap-3 p-3">
      <AppIcon app={app} />
      <span className="flex-1 truncate text-sm font-semibold text-slate-900">{app.name}</span>
      <span className="flex items-center gap-1">
        <Button
          type="button"
          variant="outline"
          size="default"
          className="size-9 px-0"
          disabled={index === 0}
          onClick={() => onMove(index, -1)}
          aria-label={`${app.name} を上へ移動`}
        >
          <IconArrowUp size={16} aria-hidden="true" />
        </Button>
        <Button
          type="button"
          variant="outline"
          size="default"
          className="size-9 px-0"
          disabled={index === total - 1}
          onClick={() => onMove(index, 1)}
          aria-label={`${app.name} を下へ移動`}
        >
          <IconArrowDown size={16} aria-hidden="true" />
        </Button>
      </span>
    </Card>
  )
}

export function AccountAppsPage({
  username,
  applications,
  csrfToken,
  isAdmin,
}: {
  username: string
  applications: MyApplication[]
  csrfToken: string
  isAdmin: boolean
}) {
  const [order, setOrder] = useState<MyApplication[]>(applications)
  const [draft, setDraft] = useState<MyApplication[] | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function startEditing() {
    setError(null)
    setDraft(order)
  }

  function cancelEditing() {
    setError(null)
    setDraft(null)
  }

  function moveItem(index: number, delta: number) {
    setDraft((current) => {
      if (!current) return current
      const target = index + delta
      if (target < 0 || target >= current.length) return current
      const next = [...current]
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  async function saveOrder() {
    if (!draft) return
    setSaving(true)
    setError(null)
    try {
      await reorderMyApplications(
        csrfToken,
        draft.map((app) => app.application_id),
      )
      setOrder(draft)
      setDraft(null)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError ? cause.message : '並び順を保存できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  const editing = draft !== null
  const items = draft ?? order

  return (
    <AccountShell
      active="apps"
      username={username}
      isAdmin={isAdmin}
      title="アプリ"
      description="あなたが利用できるアプリケーションです。タイルから起動できます。"
    >
      {order.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 p-10 text-center">
          <IconLayoutGrid size={28} className="text-slate-300" aria-hidden="true" />
          <p className="text-sm text-slate-500">利用できるアプリはまだありません。</p>
        </Card>
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-end gap-2">
            {editing ? (
              <>
                <Button type="button" variant="ghost" onClick={cancelEditing} disabled={saving}>
                  キャンセル
                </Button>
                <Button type="button" onClick={saveOrder} disabled={saving}>
                  {saving ? '保存中…' : '並び順を保存'}
                </Button>
              </>
            ) : (
              <Button type="button" variant="secondary" onClick={startEditing}>
                並び替え
              </Button>
            )}
          </div>
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
          {editing ? (
            <div className="flex flex-col gap-2">
              {items.map((app, index) => (
                <ReorderRow
                  key={app.application_id}
                  app={app}
                  index={index}
                  total={items.length}
                  onMove={moveItem}
                />
              ))}
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
              {items.map((app) => (
                <AppTile key={app.application_id} app={app} />
              ))}
            </div>
          )}
        </div>
      )}
    </AccountShell>
  )
}
