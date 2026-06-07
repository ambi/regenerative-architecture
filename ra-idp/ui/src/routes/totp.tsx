import { createFileRoute } from '@tanstack/react-router'
import { TotpPage } from '@/pages/TotpPage'

export const Route = createFileRoute('/totp')({
  component: TotpPage,
})
