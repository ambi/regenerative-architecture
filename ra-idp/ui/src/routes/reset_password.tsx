import { createFileRoute } from '@tanstack/react-router'
import { ResetPasswordPage } from '@/pages/ResetPasswordPage'

export const Route = createFileRoute('/reset_password')({
  component: ResetPasswordPage,
})
