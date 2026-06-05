/**
 * @deprecated 直接の import は ./memory/authorization-store に切り替える。
 * このファイルは Phase 1 (ADR-016) 以前の import を保つ薄いシム。
 */
export {
  InMemoryAuthorizationRequestStore,
  InMemoryAuthorizationCodeStore,
  InMemoryPARStore,
} from './memory/authorization-store'
