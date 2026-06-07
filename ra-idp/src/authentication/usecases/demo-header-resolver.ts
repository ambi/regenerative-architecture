import {
  AuthenticationContextError,
  type AuthenticationContext,
  type AuthenticationContextResolver,
} from '../domain/authentication-context'
import type { UserRepository } from '../ports/user-repository'
import { deriveAcr } from './acr-vocabulary'

/**
 * デモ用途で X-User-Sub / X-User-Auth-Time ヘッダから AuthenticationContext を
 * 復元する。`./demo.sh` や統合テストでログイン UI を経由せずに認可フローを通すための導線。
 * 本番経路（Cookie セッション）に混入させないため LoginSessionManager とは分離する。
 */
export class DemoHeaderResolver implements AuthenticationContextResolver {
  constructor(private readonly userRepo: UserRepository) {}

  async resolve(headers: Headers): Promise<AuthenticationContext | null> {
    const sub = headers.get('x-user-sub') ?? undefined
    if (!sub) return null

    const user = await this.userRepo.findBySub(sub)
    if (!user) return null

    const amr = ['demo_header']
    return {
      sub: user.sub,
      auth_time: parseAuthTimeHeader(headers.get('x-user-auth-time') ?? undefined),
      amr,
      acr: deriveAcr(amr),
      session_id: 'header-session',
    }
  }
}

function parseAuthTimeHeader(value: string | undefined): number {
  if (!value) return Math.floor(Date.now() / 1000)
  const seconds = Number(value)
  if (!Number.isInteger(seconds) || seconds < 0) {
    throw new AuthenticationContextError('X-User-Auth-Time は Unix epoch 秒で指定してください')
  }
  return seconds
}
