import { createFileRoute } from '@tanstack/react-router'
import { ConsentPage } from '@/pages/ConsentPage'

export const Route = createFileRoute('/consent')({
  component: ConsentPage,
})
