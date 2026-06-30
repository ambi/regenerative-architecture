import {
  closestCenter,
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  rectSortingStrategy,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { IconExternalLink, IconGripVertical, IconLayoutGrid } from '@tabler/icons-react'
import { useMemo, useRef, useState } from 'react'
import { reorderMyApplications } from '../../api/account'
import { AuthenticationAPIError } from '../../api/core'
import { AccountShell } from '../../components/AccountShell'
import { Card } from '../../components/ui/card'
import type { MyApplication, PortalCategory } from '../../types'

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

function AppTileContent({ app, dragHandle }: { app: MyApplication; dragHandle?: React.ReactNode }) {
  const launchable = Boolean(app.launch_url)
  return (
    <>
      {dragHandle ?? (
        <span
          className="absolute left-2 top-2 inline-flex size-8 items-center justify-center rounded-md border border-slate-200 bg-white/90 text-slate-500 shadow-xs"
          aria-hidden="true"
        >
          <IconGripVertical size={16} aria-hidden="true" />
        </span>
      )}
      <AppIcon app={app} />
      <span className="flex min-h-10 items-center gap-1 overflow-hidden text-ellipsis text-sm font-semibold leading-5 text-slate-900">
        {app.name}
        {launchable ? (
          <IconExternalLink size={14} className="shrink-0 text-slate-400" aria-hidden="true" />
        ) : null}
      </span>
      {launchable ? null : <span className="text-xs text-slate-400">起動 URL が未設定です</span>}
    </>
  )
}

function AppTile({
  app,
  onLaunch,
}: {
  app: MyApplication
  onLaunch: (app: MyApplication) => void
}) {
  const {
    attributes,
    isDragging,
    listeners,
    setActivatorNodeRef,
    setNodeRef,
    transform,
    transition,
  } = useSortable({ id: app.application_id })
  const launchable = Boolean(app.launch_url)
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <div ref={setNodeRef} style={style} className="rounded-lg">
      <Card
        onClick={() => onLaunch(app)}
        className={[
          'group relative flex h-full min-h-32 select-none flex-col items-center gap-3 p-5 text-center transition-[border-color,box-shadow,opacity]',
          launchable ? 'cursor-pointer hover:border-blue-300 hover:shadow-md' : 'opacity-70',
          isDragging ? 'border-blue-300 opacity-35 shadow-none' : '',
        ].join(' ')}
      >
        <AppTileContent
          app={app}
          dragHandle={
            <button
              type="button"
              ref={setActivatorNodeRef}
              className="absolute left-2 top-2 inline-flex size-8 touch-none cursor-grab items-center justify-center rounded-md border border-slate-200 bg-white/90 text-slate-500 shadow-xs transition-colors hover:border-blue-300 hover:text-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 active:cursor-grabbing"
              aria-label={`${app.name} をドラッグして並び替え`}
              onClick={(event) => event.stopPropagation()}
              {...attributes}
              {...listeners}
            >
              <IconGripVertical size={16} aria-hidden="true" />
            </button>
          }
        />
      </Card>
    </div>
  )
}

function AppGrid({
  apps,
  onLaunch,
}: {
  apps: MyApplication[]
  onLaunch: (app: MyApplication) => void
}) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
      {apps.map((app) => (
        <AppTile key={app.application_id} app={app} onLaunch={onLaunch} />
      ))}
    </div>
  )
}

function DragPreview({ app }: { app: MyApplication }) {
  return (
    <Card className="relative flex h-32 w-40 rotate-2 select-none flex-col items-center gap-3 border-blue-300 bg-white/95 p-5 text-center shadow-[0_26px_70px_-28px_rgb(37_99_235/70%)] backdrop-blur">
      <AppTileContent app={app} />
    </Card>
  )
}

type Section = { key: string; name: string; apps: MyApplication[] }

// buildSections は manual order を保ったまま、各カテゴリ (position 昇順) にアプリを振り分ける。
// 1 アプリは付与された各カテゴリに現れ、カテゴリ未付与のアプリは末尾の「その他」へ集める。
function buildSections(apps: MyApplication[], categories: PortalCategory[]): Section[] {
  const sections: Section[] = categories.map((category) => ({
    key: category.category_id,
    name: category.name,
    apps: apps.filter((app) => app.category_ids.includes(category.category_id)),
  }))
  const known = new Set(categories.map((category) => category.category_id))
  const uncategorized = apps.filter((app) => !app.category_ids.some((id) => known.has(id)))
  if (uncategorized.length > 0) {
    sections.push({ key: '__uncategorized__', name: 'その他', apps: uncategorized })
  }
  return sections.filter((section) => section.apps.length > 0)
}

export function AccountAppsPage({
  username,
  applications,
  categories,
  csrfToken,
  isAdmin,
}: {
  username: string
  applications: MyApplication[]
  categories: PortalCategory[]
  csrfToken: string
  isAdmin: boolean
}) {
  const [order, setOrder] = useState<MyApplication[]>(applications)
  const [activeID, setActiveID] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const suppressNextClickRef = useRef(false)
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  )
  const itemIDs = useMemo(() => order.map((app) => app.application_id), [order])
  const appByID = useMemo(() => new Map(order.map((app) => [app.application_id, app])), [order])
  const sections = buildSections(order, categories)
  const grouped = categories.length > 0
  const activeApp = activeID ? appByID.get(activeID) : null

  async function persistOrder(next: MyApplication[]) {
    setSaving(true)
    setError(null)
    try {
      await reorderMyApplications(
        csrfToken,
        next.map((app) => app.application_id),
      )
    } catch (cause) {
      setError(
        cause instanceof AuthenticationAPIError ? cause.message : '並び順を保存できませんでした。',
      )
    } finally {
      setSaving(false)
    }
  }

  function handleDragStart(event: DragStartEvent) {
    suppressNextClickRef.current = true
    setActiveID(String(event.active.id))
  }

  function handleDragEnd(event: DragEndEvent) {
    const active = String(event.active.id)
    const over = event.over ? String(event.over.id) : null
    setActiveID(null)
    if (!over || active === over) return

    const oldIndex = order.findIndex((app) => app.application_id === active)
    const newIndex = order.findIndex((app) => app.application_id === over)
    if (oldIndex < 0 || newIndex < 0) return

    const next = arrayMove(order, oldIndex, newIndex)
    setOrder(next)
    void persistOrder(next)
  }

  function handleDragCancel() {
    setActiveID(null)
  }

  function handleLaunch(app: MyApplication) {
    if (suppressNextClickRef.current) {
      suppressNextClickRef.current = false
      return
    }
    if (activeID || !app.launch_url) return
    window.open(app.launch_url, '_blank', 'noopener,noreferrer')
  }

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
          {saving ? (
            <div className="flex items-center justify-end text-xs text-slate-500">
              並び順を保存中...
            </div>
          ) : null}
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
            onDragCancel={handleDragCancel}
          >
            <SortableContext items={itemIDs} strategy={rectSortingStrategy}>
              {grouped ? (
                <div className="flex flex-col gap-6">
                  {sections.map((section) => (
                    <section key={section.key} className="flex flex-col gap-3">
                      <h2 className="text-sm font-semibold text-slate-500">{section.name}</h2>
                      <AppGrid apps={section.apps} onLaunch={handleLaunch} />
                    </section>
                  ))}
                </div>
              ) : (
                <AppGrid apps={order} onLaunch={handleLaunch} />
              )}
            </SortableContext>
            <DragOverlay dropAnimation={null}>
              {activeApp ? <DragPreview app={activeApp} /> : null}
            </DragOverlay>
          </DndContext>
        </div>
      )}
    </AccountShell>
  )
}
