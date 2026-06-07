import { RouterProvider, createRouter } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { BrandingProvider } from './branding/context'
import { I18nProvider } from './i18n/context'
import { routeTree } from './routeTree.gen'
import './styles/globals.css'

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const root = document.getElementById('root')
if (!root) throw new Error('root element not found')
createRoot(root).render(
  <StrictMode>
    <BrandingProvider>
      <I18nProvider>
        <RouterProvider router={router} />
      </I18nProvider>
    </BrandingProvider>
  </StrictMode>,
)
