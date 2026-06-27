import { IconChevronRight, IconDotsVertical, IconPencil } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Button } from './ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from './ui/dropdown-menu'

// AdminPaneActions は一覧画面の右ペイン共通のアクション行 (wi-39)。
// どのエンティティ (ユーザー / アプリケーション / グループ / ロール) でも
// 「詳細」→「編集」→ その他操作 (⋮ メニュー) の順で
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
  const hasSecondaryAction = Boolean(onEdit || menu)
  return (
    <div className="flex items-center gap-2">
      {detailHref ? (
        <Button asChild className={hasSecondaryAction ? 'flex-1' : 'min-w-28'}>
          <a href={detailHref}>
            詳細
            <IconChevronRight size={16} aria-hidden="true" />
          </a>
        </Button>
      ) : null}
      {onEdit ? (
        <Button type="button" variant="outline" className="flex-1" disabled={busy} onClick={onEdit}>
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
