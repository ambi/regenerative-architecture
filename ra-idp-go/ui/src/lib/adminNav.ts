import {
  IconActivity,
  IconBuildingCommunity,
  IconCheckupList,
  IconKey,
  IconLayoutDashboard,
  IconShieldLock,
  IconUserShield,
  IconUsers,
} from '@tabler/icons-react'
import { tenantBasePath, tenantURL } from '../api'

export type AdminNavKey =
  | 'dashboard'
  | 'users'
  | 'roles'
  | 'clients'
  | 'consents'
  | 'audit-events'
  | 'keys'
  | 'tenants'

export type AdminNavItem = {
  key: AdminNavKey
  label: string
  icon: typeof IconUsers
  href: string
  active: boolean
  controlPlaneOnly?: boolean
}

const DEFAULT_TENANT_ID = 'default'

export function adminNavItems(active: AdminNavKey): AdminNavItem[] {
  const items: AdminNavItem[] = [
    { key: 'dashboard', label: 'ダッシュボード', icon: IconLayoutDashboard, href: tenantURL('/admin'), active: active === 'dashboard' },
    { key: 'users', label: 'ユーザー', icon: IconUsers, href: tenantURL('/admin/users'), active: active === 'users' },
    { key: 'roles', label: 'ロールと権限', icon: IconUserShield, href: tenantURL('/admin/roles'), active: active === 'roles' },
    { key: 'clients', label: 'アプリケーション', icon: IconKey, href: tenantURL('/admin/clients'), active: active === 'clients' },
    { key: 'consents', label: '同意', icon: IconCheckupList, href: tenantURL('/admin/consents'), active: active === 'consents' },
    { key: 'audit-events', label: '監査ログ', icon: IconActivity, href: tenantURL('/admin/audit_events'), active: active === 'audit-events' },
    { key: 'keys', label: '署名鍵', icon: IconShieldLock, href: tenantURL('/admin/keys'), active: active === 'keys' },
  ]
  if (isControlPlane()) {
    items.push({
      key: 'tenants',
      label: 'テナント',
      icon: IconBuildingCommunity,
      href: `/realms/${DEFAULT_TENANT_ID}/admin/tenants`,
      active: active === 'tenants',
      controlPlaneOnly: true,
    })
  }
  return items
}

function isControlPlane(): boolean {
  return tenantBasePath() === `/realms/${DEFAULT_TENANT_ID}`
}
