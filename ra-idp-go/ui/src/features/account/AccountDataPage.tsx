import { IconDownload, IconFileText } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthenticationAPIError, exportAccountData } from '../../api'
import { AccountShell } from '../../components/AccountShell'
import { Alert } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'

export function AccountDataPage({ username, isAdmin }: { username: string; isAdmin: boolean }) {
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState('')

  async function handleExport() {
    setDownloading(true)
    setError('')
    try {
      const data = await exportAccountData()
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `account-data-${username}.json`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError
          ? cause.message
          : 'データをエクスポートできませんでした。',
      )
    } finally {
      setDownloading(false)
    }
  }

  return (
    <AccountShell
      active="data"
      username={username}
      isAdmin={isAdmin}
      title="データとプライバシー"
      description="アカウントに保存されている個人データをダウンロードできます。"
    >
      {error ? <Alert variant="destructive">{error}</Alert> : null}

      <Card className="flex flex-col gap-4 p-5">
        <div className="flex items-start gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
            <IconFileText size={20} aria-hidden="true" />
          </span>
          <div>
            <p className="text-sm font-semibold text-slate-900">個人データのエクスポート</p>
            <p className="mt-1 text-sm leading-6 text-slate-600">
              プロフィール (表示名・属性・メール・ライフサイクル) と、アクセスを許可した アプリ
              (接続済みアプリ) を JSON ファイルとしてダウンロードします。サインイン
              履歴やセッションの同梱は今後対応します。
            </p>
          </div>
        </div>
        <div>
          <Button type="button" onClick={handleExport} disabled={downloading}>
            <IconDownload size={16} aria-hidden="true" />
            {downloading ? '生成中…' : 'データをダウンロード (JSON)'}
          </Button>
        </div>
      </Card>
    </AccountShell>
  )
}
