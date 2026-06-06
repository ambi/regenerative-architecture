/**
 * Layer 3 — Specification Binding (TypeScript)（Discovery）
 *
 * Discovery 文書（OIDC Discovery 1.0 / RFC 8414 Authorization Server Metadata）は
 * 仕様核から派生する成果物（ADR-011）。spec/scl.yaml の `interfaces` のパス、
 * `models` の列挙、`annotations.discovery_template` から組み立てる。
 */

import { scl, enumWireValues, httpBinding, toWire } from './scl'

const ENDPOINTS = {
  authorization_endpoint: 'Authorize',
  token_endpoint: 'Token',
  userinfo_endpoint: 'UserInfo',
  jwks_uri: 'GetJwks',
  introspection_endpoint: 'Introspect',
  revocation_endpoint: 'Revoke',
  pushed_authorization_request_endpoint: 'PushAuthorizationRequest',
  device_authorization_endpoint: 'DeviceAuthorization',
  registration_endpoint: 'RegisterClient',
  end_session_endpoint: 'EndSession',
} as const

function endpointPath(interfaceName: string): string {
  const iface = scl.interfaces[interfaceName]
  const http = iface ? httpBinding(iface) : undefined
  if (!http?.path) throw new Error(`interface ${interfaceName} has no http binding path`)
  return http.path
}

export function buildDiscoveryDocument(issuer: string): Record<string, unknown> {
  const tpl = (scl.annotations?.discovery_template ?? {}) as {
    scopes_supported?: string[]
    subject_types_supported?: string[]
    claims_supported?: string[]
    ui_locales_supported?: string[]
    introspection_endpoint_auth_methods?: string[]
    revocation_endpoint_auth_methods?: string[]
  }

  const toWireList = (names: string[]) => names.map(toWire)

  const doc: Record<string, unknown> = {
    issuer,
  }
  for (const [field, iface] of Object.entries(ENDPOINTS)) {
    doc[field] = `${issuer}${endpointPath(iface)}`
  }
  doc.scopes_supported = tpl.scopes_supported ?? []
  doc.response_types_supported = enumWireValues('ResponseType')
  doc.response_modes_supported = enumWireValues('ResponseMode')
  doc.grant_types_supported = enumWireValues('GrantType')
  doc.subject_types_supported = tpl.subject_types_supported ?? ['public']
  doc.id_token_signing_alg_values_supported = enumWireValues('SignatureAlgorithm')
  doc.token_endpoint_auth_methods_supported = enumWireValues('TokenEndpointAuthMethod').filter(
    (m) => m !== 'none',
  )
  doc.token_endpoint_auth_signing_alg_values_supported = enumWireValues('SignatureAlgorithm')
  doc.introspection_endpoint_auth_methods_supported = toWireList(
    tpl.introspection_endpoint_auth_methods ?? [],
  )
  doc.revocation_endpoint_auth_methods_supported = toWireList(
    tpl.revocation_endpoint_auth_methods ?? [],
  )
  doc.code_challenge_methods_supported = enumWireValues('CodeChallengeMethod')
  doc.require_pushed_authorization_requests = false
  doc.require_pkce = true
  doc.dpop_signing_alg_values_supported = enumWireValues('SignatureAlgorithm')
  doc.tls_client_certificate_bound_access_tokens = true
  doc.claims_supported = tpl.claims_supported ?? []
  doc.service_documentation = `${issuer}/docs`
  doc.ui_locales_supported = tpl.ui_locales_supported ?? ['en', 'ja']
  return doc
}
