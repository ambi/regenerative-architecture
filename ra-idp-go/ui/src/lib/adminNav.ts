import {
  IconActivity,
  IconApps,
  IconBuildingCommunity,
  IconCheckupList,
  IconForms,
  IconLayoutDashboard,
  IconRobot,
  IconSettings,
  IconShieldLock,
  IconUsersGroup,
  IconUserShield,
  IconUsers,
  IconWorldShare,
} from '@tabler/icons-react'
import { tenantBasePath, tenantURL } from '../api'

export type AdminNavKey =
  | 'dashboard'
  | 'users'
  | 'groups'
  | 'agents'
  | 'roles'
  | 'applications'
  | 'entra-federation'
  | 'authz-detail-types'
  | 'consents'
  | 'audit-events'
  | 'keys'
  | 'tenants'
  | 'tenant-attributes'
  | 'settings'

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
    {
      key: 'dashboard',
      label: 'ダッシュボード',
      icon: IconLayoutDashboard,
      href: tenantURL('/admin'),
      active: active === 'dashboard',
    },
    {
      key: 'users',
      label: 'ユーザー',
      icon: IconUsers,
      href: tenantURL('/admin/users'),
      active: active === 'users',
    },
    {
      key: 'groups',
      label: 'グループ',
      icon: IconUsersGroup,
      href: tenantURL('/admin/groups'),
      active: active === 'groups',
    },
    {
      key: 'agents',
      label: 'エージェント',
      icon: IconRobot,
      href: tenantURL('/admin/agents'),
      active: active === 'agents',
    },
    {
      key: 'roles',
      label: 'ロール',
      icon: IconUserShield,
      href: tenantURL('/admin/roles'),
      active: active === 'roles',
    },
    {
      key: 'applications',
      label: 'アプリケーション',
      icon: IconApps,
      href: tenantURL('/admin/applications'),
      active: active === 'applications',
    },
    {
      key: 'authz-detail-types',
      label: '認可詳細の種類',
      icon: IconForms,
      href: tenantURL('/admin/authorization-detail-types'),
      active: active === 'authz-detail-types',
    },
    {
      key: 'consents',
      label: '同意',
      icon: IconCheckupList,
      href: tenantURL('/admin/consents'),
      active: active === 'consents',
    },
    {
      key: 'audit-events',
      label: '監査ログ',
      icon: IconActivity,
      href: tenantURL('/admin/audit_events'),
      active: active === 'audit-events',
    },
    {
      key: 'keys',
      label: '署名鍵',
      icon: IconShieldLock,
      href: tenantURL('/admin/keys'),
      active: active === 'keys',
    },
    // OAuth2 クライアント / WS-Federation RP の設定は「アプリケーション」に一本化した。
    // 高度な OIDC 設定 (grant / response 種別・PAR・DPoP、作成時の認証方式) もアプリ編集画面に
    // 畳んだため、専用の低レベル client 画面は撤去した。
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
  items.push({
    key: 'tenant-attributes',
    label: 'ユーザー属性',
    icon: IconForms,
    href: tenantURL('/admin/tenant/attributes'),
    active: active === 'tenant-attributes',
  })
  items.push({
    key: 'entra-federation',
    label: 'Entra 連携',
    icon: IconWorldShare,
    href: tenantURL('/admin/federation/entra'),
    active: active === 'entra-federation',
  })
  items.push({
    key: 'settings',
    label: '設定',
    icon: IconSettings,
    href: tenantURL('/admin/settings'),
    active: active === 'settings',
  })
  return items
}

function isControlPlane(): boolean {
  return tenantBasePath() === `/realms/${DEFAULT_TENANT_ID}`
}
