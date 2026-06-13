import { randomUUID } from 'crypto'
import type { LoginSession, SessionStore } from '../ports/session-store'
import type {
  AuthenticationContext,
  AuthenticationContextResolver,
} from '../domain/authentication-context'
import type { UserRepository } from '../ports/user-repository'
import { deriveAcr } from './acr-vocabulary'

export const SESSION_COOKIE = 'ra_idp_session'
export const SESSION_TTL_SECONDS = 3600

export interface SessionManager extends AuthenticationContextResolver {
  create(
    tenantId: string,
    sub: string,
    amr: string[],
    now?: Date,
    options?: { authenticationPending?: boolean },
  ): Promise<AuthenticationContext>
  /**
   * 中間セッションに追加 factor を記録し認証完了状態に昇格する。
   * amr の重複は除き、acr は更新後の amr から再導出する。
   */
  completeFactor(sessionId: string, additionalAmr: string[]): Promise<AuthenticationContext | null>
  revoke(cookieHeader: string | undefined): Promise<void>
}

export class LoginSessionManager implements SessionManager {
  /**
   * ADR-031: `userRepo` を渡すと `resolve` が user の `disabled_at` を見て、
   * 無効化されたユーザーの session は副作用付きで失効させ null を返す。
   * userRepo を渡さない場合 (旧テスト互換) は session 単独の検証のみ行う。
   */
  constructor(
    private readonly sessionStore: SessionStore,
    private readonly userRepo?: UserRepository,
  ) {}

  async create(
    tenantId: string,
    sub: string,
    amr: string[],
    now = new Date(),
    options: { authenticationPending?: boolean } = {},
  ): Promise<AuthenticationContext> {
    const acr = deriveAcr(amr)
    const session: LoginSession = {
      id: randomUUID(),
      tenant_id: tenantId,
      sub,
      auth_time: Math.floor(now.getTime() / 1000),
      amr,
      acr,
      authentication_pending: options.authenticationPending ?? false,
      expires_at: new Date(now.getTime() + SESSION_TTL_SECONDS * 1000).toISOString(),
    }
    await this.sessionStore.save(session)
    return {
      sub,
      auth_time: session.auth_time,
      amr,
      acr,
      session_id: session.id,
      authentication_pending: session.authentication_pending,
    }
  }

  async completeFactor(
    sessionId: string,
    additionalAmr: string[],
  ): Promise<AuthenticationContext | null> {
    const session = await this.sessionStore.find(sessionId)
    if (!session) return null
    const mergedAmr = Array.from(new Set([...session.amr, ...additionalAmr]))
    const updated: LoginSession = {
      ...session,
      amr: mergedAmr,
      acr: deriveAcr(mergedAmr),
      authentication_pending: false,
    }
    await this.sessionStore.save(updated)
    return {
      sub: updated.sub,
      auth_time: updated.auth_time,
      amr: updated.amr,
      acr: updated.acr,
      session_id: updated.id,
      authentication_pending: false,
    }
  }

  async resolve(headers: Headers): Promise<AuthenticationContext | null> {
    const sessionId = parseCookies(headers.get('cookie') ?? undefined)[SESSION_COOKIE]
    if (!sessionId) return null

    const session = await this.sessionStore.find(sessionId)
    if (!session) return null
    if (this.userRepo) {
      const user = await this.userRepo.findBySub(session.sub)
      if (!user || user.disabled_at) {
        // ADR-031: 無効化された user の既存セッションは利用できない。
        // 副作用として session を即時失効させる。
        await this.sessionStore.delete(session.id)
        return null
      }
    }
    return {
      sub: session.sub,
      auth_time: session.auth_time,
      amr: session.amr,
      acr: session.acr,
      session_id: session.id,
      authentication_pending: session.authentication_pending,
    }
  }

  async revoke(cookieHeader: string | undefined): Promise<void> {
    const sessionId = parseCookies(cookieHeader)[SESSION_COOKIE]
    if (sessionId) await this.sessionStore.delete(sessionId)
  }
}

function parseCookies(header: string | undefined): Record<string, string> {
  const cookies: Record<string, string> = {}
  if (!header) return cookies
  for (const part of header.split(';')) {
    const [name, ...rest] = part.trim().split('=')
    if (!name) continue
    cookies[name] = decodeURIComponent(rest.join('='))
  }
  return cookies
}
