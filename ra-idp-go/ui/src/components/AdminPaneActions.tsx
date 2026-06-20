import { IconArrowRight, IconDotsVertical, IconPencil } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Button } from './ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from './ui/dropdown-menu'

// AdminPaneActions は一覧画面の右ペイン共通のアクション行 (wi-39)。
// どのエンティティ (ユーザー / アプリケーション / グループ / ロール) でも
// 「詳細を開く」(primary) → 「編集」(outline) → その他操作 (⋮ メニュー) の順で
// 同じ配置・同じ体裁にそろえる。編集やメニューが無いエンティティでは省略する。
export function AdminPaneActions({
  detailHref,
  onEdit,
  busy = false,
  menu,
}: {
  detailHref?: string
  onEdit?: () => void
  busy?: boolean
  menu?: ReactNode
}) {
  return (
    <div className="flex items-center gap-2">
      {detailHref ? (
        <a
          href={detailHref}
          className="inline-flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-blue-600 px-3 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
        >
          詳細を開く
          <IconArrowRight size={16} aria-hidden="true" />
        </a>
      ) : null}
      {onEdit ? (
        <Button
          type="button"
          variant="outline"
          className="flex-1"
          disabled={busy}
          onClick={onEdit}
        >
          <IconPencil size={16} aria-hidden="true" />
          編集
        </Button>
      ) : null}
      {menu ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="outline"
              className="size-9 shrink-0 px-0"
              aria-label="その他の操作"
              disabled={busy}
            >
              <IconDotsVertical size={18} aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">{menu}</DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </div>
  )
}
