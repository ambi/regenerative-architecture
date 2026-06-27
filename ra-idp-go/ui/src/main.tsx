import './styles.css'

import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createAppRouter } from './router'

const root = document.getElementById('root')
if (!root) {
  throw new Error('RA Identity root element is missing')
}

// ルーティングは client-side。各 route module の loader が対象画面のデータを取得し、
// ページ種別の meta 表明・認証エラー画面は router runtime が担う (wi-67)。
const router = createAppRouter()
createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
