import { randomUUID } from 'crypto'
import type { LoginSession, SessionStore } from '../ports/session-store'
import type {
  AuthenticationContext,
  AuthenticationContextResolver,
} from '../domain/authentication-context'

export const SESSION_COOKIE = 'ra_idp_session'
export const SESSION_TTL_SECONDS = 3600

export interface SessionManager extends AuthenticationContextResolver {
  create(sub: string, amr: string[], now?: Date): Promise<AuthenticationContext>
  revoke(cookieHeader: string | undefined): Promise<void>
}

export class LoginSessionManager implements SessionManager {
  constructor(private readonly sessionStore: SessionStore) {}

  async create(sub: string, amr: string[], now = new Date()): Promise<AuthenticationContext> {
    const session: LoginSession = {
      id: randomUUID(),
      sub,
      auth_time: Math.floor(now.getTime() / 1000),
      expires_at: new Date(now.getTime() + SESSION_TTL_SECONDS * 1000).toISOString(),
    }
    await this.sessionStore.save(session)
    return {
      sub,
      auth_time: session.auth_time,
      amr,
      session_id: session.id,
    }
  }

  async resolve(headers: Headers): Promise<AuthenticationContext | null> {
    const sessionId = parseCookies(headers.get('cookie') ?? undefined)[SESSION_COOKIE]
    if (!sessionId) return null

    const session = await this.sessionStore.find(sessionId)
    if (!session) return null
    return {
      sub: session.sub,
      auth_time: session.auth_time,
      amr: ['pwd'],
      session_id: session.id,
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
