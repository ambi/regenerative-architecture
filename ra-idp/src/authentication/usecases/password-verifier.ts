import { createHash, timingSafeEqual } from 'crypto'

export interface PasswordVerifier {
  verify(password: string, storedHash: string): boolean
}

export class Sha256PasswordVerifier implements PasswordVerifier {
  verify(password: string, storedSha256Hex: string): boolean {
    const actual = createHash('sha256').update(password).digest('hex')
    if (actual.length !== storedSha256Hex.length) return false
    return timingSafeEqual(Buffer.from(actual), Buffer.from(storedSha256Hex))
  }
}
