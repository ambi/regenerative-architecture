import { IconDeviceDesktopCheck, IconShieldCheck, IconX } from '@tabler/icons-react'
import { useState } from 'react'
import { AuthShell } from '../components/AuthShell'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import type { DevicePage as DevicePageData } from '../types'

export function DevicePage({ userCode }: DevicePageData) {
  const normalizedCode = userCode.replace(/-/g, '').toUpperCase()
  const [code, setCode] = useState(normalizedCode)

  return (
    <AuthShell
      asideTitle="新しいデバイスを、確かな手順で接続。"
      asideText="画面に表示されたコードを確認し、信頼できるデバイスだけを承認してください。"
    >
      <div className="flex flex-col gap-8">
        <div className="flex flex-col items-center gap-3 text-center">
          <div className="flex size-[58px] items-center justify-center rounded-[18px] bg-indigo-50 text-indigo-600">
            <IconDeviceDesktopCheck size={30} />
          </div>
          <h2 className="text-2xl font-bold tracking-tight">デバイスを接続</h2>
          <p className="text-slate-500">接続先に表示されているコードを入力してください。</p>
        </div>

        <form method="POST" action="/device">
          <input type="hidden" name="user_code" value={code} />
          <div className="flex flex-col items-center gap-6">
            <Input
              value={code}
              onChange={(event) =>
                setCode(
                  event.currentTarget.value
                    .replace(/[^a-z0-9]/gi, '')
                    .slice(0, 8)
                    .toUpperCase(),
                )
              }
              inputMode="text"
              autoComplete="one-time-code"
              aria-label="ユーザーコード"
              className="h-14 max-w-sm text-center font-mono text-xl font-bold tracking-[0.45em] uppercase"
            />
            <p className="text-xs text-slate-500">
              例:{' '}
              <code className="rounded bg-slate-100 px-1.5 py-1 font-mono text-slate-700">
                ABCD-EFGH
              </code>
            </p>
            <div className="flex w-full flex-col gap-2">
              <Button
                type="submit"
                name="action"
                value="allow"
                size="lg"
                disabled={code.length !== 8}
              >
                <IconShieldCheck size={18} />
                デバイスを承認
              </Button>
              <Button
                type="submit"
                name="action"
                value="deny"
                size="lg"
                variant="ghost"
                disabled={code.length !== 8}
              >
                <IconX size={17} />
                拒否
              </Button>
            </div>
          </div>
        </form>

        <div className="flex items-center justify-center gap-2 text-xs text-slate-500">
          <IconShieldCheck className="text-teal-600" size={15} />
          <span>心当たりのない接続は承認しないでください</span>
        </div>
      </div>
    </AuthShell>
  )
}
