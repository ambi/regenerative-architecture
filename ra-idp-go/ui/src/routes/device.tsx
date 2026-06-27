import { createFileRoute } from '@tanstack/react-router'
import { request } from '../api/core'
import { DevicePage } from '../features/auth-flow/DevicePage'
import type { DevicePage as DevicePageData } from '../types'
import { PageMarker } from './-page'

type DeviceResponse = { kind: 'device'; csrf_token: string; user_code: string }

export const Route = createFileRoute('/device')({
  loader: async ({ location }): Promise<DevicePageData> => {
    const userCode = new URLSearchParams(location.searchStr).get('user_code') ?? ''
    const data = await request<DeviceResponse>(
      `/api/auth/device?user_code=${encodeURIComponent(userCode)}`,
    )
    return { kind: 'device', csrfToken: data.csrf_token, userCode: data.user_code }
  },
  component: DeviceRoute,
})

function DeviceRoute() {
  const data = Route.useLoaderData()
  return (
    <PageMarker kind={data.kind}>
      <DevicePage {...data} />
    </PageMarker>
  )
}
