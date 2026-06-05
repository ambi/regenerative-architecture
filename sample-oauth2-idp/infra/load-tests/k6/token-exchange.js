/**
 * k6 load test — /token endpoint
 *
 * spec/scl.yaml objectives から `bun run gen:k6` で生成される thresholds.json を読み込み、
 * SLO 違反時に exit code を non-zero にする。
 *
 * 使い方:
 *   k6 run infra/load-tests/k6/token-exchange.js
 *   k6 run -e BASE_URL=http://idp:3000 -e VUS=200 infra/load-tests/k6/token-exchange.js
 */

import http from 'k6/http'
import { check } from 'k6'
import { SharedArray } from 'k6/data'

const thresholdsConfig = JSON.parse(open('./thresholds.json'))

export const options = {
  scenarios: {
    token_exchange: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RPS || 100),
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: Number(__ENV.VUS || 50),
      maxVUs: Number(__ENV.MAX_VUS || 200),
    },
  },
  thresholds: thresholdsConfig.thresholds,
}

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000'

const clients = new SharedArray('clients', () => [
  { id: 'demo-web-app', secret: __ENV.DEMO_CLIENT_SECRET || 'demo-secret-please-rotate' },
])

export default function () {
  const c = clients[0]
  const auth = `Basic ${encoding(`${c.id}:${c.secret}`)}`
  const res = http.post(`${BASE_URL}/token`, `grant_type=client_credentials&scope=openid`, {
    headers: {
      Authorization: auth,
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    tags: { endpoint: 'token' },
  })
  check(res, {
    'status 200': (r) => r.status === 200,
    'has access_token': (r) => r.json('access_token') !== undefined,
  })
}

function encoding(s) {
  // k6 環境では Buffer がないので base64 を実装
  return (
    Array.from(s).reduce((acc) => acc, '') &&
    (typeof __ENV.BTOA === 'function' ? __ENV.BTOA(s) : btoa(s))
  )
}
