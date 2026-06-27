import { request, tenantBasePath } from '../api/core'
import { ensureLoggedIn, type PortalAudience } from '../api/oidc'

export type AccountContextResponse = {
  csrf_token: string
  sub: string
  preferred_username?: string
  tenant_id?: string
  roles?: string[]
}

export function hasAdminRole(roles: string[] | undefined): boolean {
  return (roles ?? []).some((role) => role === 'admin' || role === 'system_admin')
}

export async function requirePortalAccount(
  audience: PortalAudience,
  pathname: string,
  search: string,
): Promise<AccountContextResponse> {
  await ensureLoggedIn(audience, `${tenantBasePath()}${pathname}${search}`)
  return request<AccountContextResponse>('/api/auth/account')
}
