/**
 * Layer 4 — Adapter Layer（HTTP: Discovery + JWKS）
 *
 * ADR-011 に従い、Discovery 文書は仕様核（spec/scl.yaml）の interfaces / models /
 * annotations.discovery_template から `buildDiscoveryDocument` が組み立てる。
 */

import { Hono } from 'hono'
import { buildDiscoveryDocument } from '../../src/spec-bindings/discovery'
import type { KeyStore } from '../../src/ports/key-store'

export interface DiscoveryRoutesDeps {
  issuer: string
  keyStore: KeyStore
}

export function createDiscoveryRoutes(deps: DiscoveryRoutesDeps) {
  const app = new Hono()

  const metadata = buildDiscoveryDocument(deps.issuer)
  const oauthMetadata = { ...metadata }
  delete (oauthMetadata as { claims_supported?: unknown }).claims_supported

  app.get('/.well-known/openid-configuration', (c) => c.json(metadata))
  app.get('/.well-known/oauth-authorization-server', (c) => c.json(oauthMetadata))

  app.get('/jwks', async (c) => {
    const keys = await deps.keyStore.getAllKeys()
    return c.json({ keys: keys.map((k) => k.publicJwk) })
  })

  return app
}
