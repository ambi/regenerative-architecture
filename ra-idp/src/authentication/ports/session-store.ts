import type { LoginSession } from '../domain/login-session'

export type { LoginSession }

export interface SessionStore {
  find(id: string): Promise<LoginSession | null>
  save(session: LoginSession): Promise<void>
  delete(id: string): Promise<void>
}
