import './styles.css'

import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createAppRouter } from './router'

const root = document.getElementById('root')
if (!root) {
  throw new Error('RA Identity root element is missing')
}

// ルーティングは client-side。初期 URL の loader (resolvePageData) がページデータを取得し、
// ページ種別の meta 表明・認証エラー画面は router (PageView / ErrorScreen) が担う (wi-67)。
const router = createAppRouter()
createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
