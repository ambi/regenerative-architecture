import { createFileRoute } from '@tanstack/react-router'
import { DevicePage } from '@/pages/DevicePage'

export const Route = createFileRoute('/device')({
  component: DevicePage,
})
