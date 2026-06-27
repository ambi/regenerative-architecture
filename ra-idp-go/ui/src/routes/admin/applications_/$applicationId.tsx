import { createFileRoute, Outlet } from '@tanstack/react-router'

// $applicationId は詳細 (index) と編集 (edit) を束ねるレイアウトルート。
// 子ルートを描画するだけで、認可・データ取得は各子ルートの loader が担う。
export const Route = createFileRoute('/admin/applications_/$applicationId')({
  component: Outlet,
})
