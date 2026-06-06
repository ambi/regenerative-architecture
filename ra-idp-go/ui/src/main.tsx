import './styles.css'

import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { readPageData } from './page-data'
import { createAppRouter } from './router'

const pageData = readPageData()
const router = createAppRouter(pageData)
const root = document.getElementById('root')

if (!root) {
  throw new Error('RA Identity root element is missing')
}

createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
